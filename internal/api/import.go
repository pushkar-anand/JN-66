package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"

	"github.com/pushkaranand/finagent/internal/importer"
	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// importUserGetter is the user-store surface needed by the import handler.
type importUserGetter interface {
	GetByID(ctx context.Context, id string) (*sqlcgen.User, error)
}

// importAccountStore is the account-store surface needed by the import handler.
// ListByUser is used for ownership verification — it only returns accounts the
// authenticated user is a member of, so a missing entry means 404.
type importAccountStore interface {
	ListByUser(ctx context.Context, userID string) ([]sqlcgen.Account, error)
	UpdateBalance(ctx context.Context, accountID string, balance model.Money, asOf time.Time) error
}

// ImportConfig holds dependencies for the POST /api/import handler.
// Pass nil to api.New to disable the route.
//
// TxnStore, RunStore, and CatStore are concrete types because importer.NewImporter
// requires *store.TransactionStore, *store.ImportRunStore, and *store.CategoryStore
// directly. Interface extraction for these belongs in the importer package.
type ImportConfig struct {
	UserGetter   importUserGetter
	AccountStore importAccountStore
	TxnStore     *store.TransactionStore
	RunStore     *store.ImportRunStore
	CatStore     *store.CategoryStore
	LLMProvider  llm.Provider
	TaggingModel string
}

type importTxnRequest struct {
	Date         string `json:"date"          validate:"required,txndate"`
	Description  string `json:"description"   validate:"required"`
	AmountPaise  int64  `json:"amount_paise"  validate:"required,min=1"`
	Direction    string `json:"direction"     validate:"required,oneof=debit credit"`
	Reference    string `json:"reference"`
	BalancePaise *int64 `json:"balance_paise"`
}

type importRequest struct {
	AccountID    string             `json:"account_id"   validate:"required,uuid"`
	NoEnrich     bool               `json:"no_enrich"`
	Transactions []importTxnRequest `json:"transactions" validate:"required,min=1,dive"`
}

type importResponse struct {
	RunID     string `json:"run_id"`
	AccountID string `json:"account_id"`
	Parsed    int    `json:"parsed"`
	Inserted  int    `json:"inserted"`
	Duplicate int    `json:"duplicate"`
	Failed    int    `json:"failed"`
}

// validateTxnDate accepts YYYY-MM-DD or RFC3339.
func validateTxnDate(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func parseTxnDate(s string) time.Time {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := s.v.ValidateStruct(r.Context(), &req)
	if err != nil {
		slog.ErrorContext(r.Context(), "import: validator error", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !result.Valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(result.Failed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	userID := UserIDFromContext(r.Context())

	// Resolve account — ListByUser joins through account_members, so only the
	// caller's accounts are returned. Missing entry → 404 (not found or not a member).
	accounts, err := s.importCfg.AccountStore.ListByUser(ctx, userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "import: list accounts error",
			slog.String("user_id", userID),
			bwglogger.Error(err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var account *sqlcgen.Account
	for i := range accounts {
		if accounts[i].ID.String() == req.AccountID {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}

	user, err := s.importCfg.UserGetter.GetByID(ctx, userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "import: get user error",
			slog.String("user_id", userID),
			bwglogger.Error(err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Map request rows to RawTransaction.
	rows := make([]parser.RawTransaction, len(req.Transactions))
	for i, t := range req.Transactions {
		rows[i] = parser.RawTransaction{
			Date:        parseTxnDate(t.Date),
			Description: t.Description,
			Amount:      t.AmountPaise,
			Direction:   sqlcgen.TxnDirectionEnum(t.Direction),
			Reference:   t.Reference,
			Balance:     t.BalancePaise,
		}
	}

	// Build enricher unless skipped.
	var enricher *importer.Enricher
	if !req.NoEnrich && s.importCfg.LLMProvider != nil {
		cats, err := s.importCfg.CatStore.List(ctx)
		if err != nil {
			slog.WarnContext(ctx, "import: failed to load categories, enrichment will be uncategorised", bwglogger.Error(err))
		}
		catInfos := make([]importer.CategoryInfo, len(cats))
		for i, c := range cats {
			catInfos[i] = importer.CategoryInfo{Slug: c.Slug, Description: c.Description}
		}
		enricher = importer.NewEnricher(s.importCfg.LLMProvider, s.importCfg.TaggingModel, catInfos)
	}

	imp := importer.NewImporter(s.importCfg.TxnStore, s.importCfg.RunStore, s.importCfg.CatStore, enricher)
	res, err := imp.Run(ctx, importer.RunParams{
		User:      user,
		AccountID: account.ID,
		SourceRef: "api",
		Rows:      rows,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "import: run error",
			slog.String("user_id", userID),
			slog.String("account_id", req.AccountID),
			bwglogger.Error(err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if res.LatestBalance != nil {
		if err := s.importCfg.AccountStore.UpdateBalance(ctx, req.AccountID, model.Money(*res.LatestBalance), res.LatestBalanceDate); err != nil {
			slog.WarnContext(r.Context(), "import: update balance error", bwglogger.Error(err))
		}
	}

	slog.InfoContext(r.Context(), "import complete",
		slog.String("user_id", userID),
		slog.String("account_id", req.AccountID),
		slog.String("run_id", res.RunID.String()),
		slog.Int("inserted", res.Inserted),
		slog.Int("duplicate", res.Duplicate),
	)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importResponse{
		RunID:     res.RunID.String(),
		AccountID: req.AccountID,
		Parsed:    res.Parsed,
		Inserted:  res.Inserted,
		Duplicate: res.Duplicate,
		Failed:    res.Failed,
	})
}
