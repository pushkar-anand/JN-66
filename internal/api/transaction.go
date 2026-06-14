package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// txnStoreAPI is the transaction-store surface needed by the transaction handlers.
type txnStoreAPI interface {
	List(ctx context.Context, p store.ListTransactionsParams) ([]sqlcgen.VTransaction, error)
	Count(ctx context.Context, p store.ListTransactionsParams) (int64, error)
	GetByID(ctx context.Context, id, userID string) (*sqlcgen.VTransaction, error)
	UpdateEnrichment(ctx context.Context, p store.EnrichmentParams) error
}

// catStoreAPI is the category-store surface needed by the transaction handlers.
type catStoreAPI interface {
	List(ctx context.Context) ([]sqlcgen.Category, error)
	GetBySlug(ctx context.Context, slug string) (*sqlcgen.Category, error)
}

// labelStoreAPI is the label-store surface needed by the transaction and label handlers.
type labelStoreAPI interface {
	List(ctx context.Context, userID string) ([]sqlcgen.Label, error)
	FindOrCreate(ctx context.Context, userID, name string) (string, error)
	AddToTransaction(ctx context.Context, txnID, labelID string) error
	RemoveFromTransaction(ctx context.Context, txnID, labelID string) error
	ListForTransaction(ctx context.Context, txnID uuid.UUID) ([]sqlcgen.Label, error)
	ListForTransactions(ctx context.Context, txnIDs []uuid.UUID) (map[uuid.UUID][]sqlcgen.Label, error)
}

// memStoreAPI is the memory-store surface needed by the transaction handlers.
type memStoreAPI interface {
	Save(ctx context.Context, userID *string, content string, memType sqlcgen.MemoryTypeEnum, tags []string) (*sqlcgen.AgentMemory, error)
}

// TransactionConfig holds dependencies for the transaction API routes.
// Pass nil to api.New to disable these routes.
type TransactionConfig struct {
	TxnStore   txnStoreAPI
	CatStore   catStoreAPI
	LabelStore labelStoreAPI
	MemStore   memStoreAPI
}

// transactionResponse is the JSON shape for a single transaction.
type transactionResponse struct {
	ID                    string   `json:"id"`
	AccountID             string   `json:"account_id"`
	TxnDate               string   `json:"txn_date"`
	Description           string   `json:"description"`
	DescriptionNormalized *string  `json:"description_normalized,omitempty"`
	Amount                int64    `json:"amount"`
	Currency              string   `json:"currency"`
	Direction             string   `json:"direction"`
	CounterpartyName      *string  `json:"counterparty_name,omitempty"`
	CounterpartyID        *string  `json:"counterparty_identifier,omitempty"`
	PaymentMode           *string  `json:"payment_mode,omitempty"`
	CategoryID            *string  `json:"category_id,omitempty"`
	CategorySlug          *string  `json:"category_slug,omitempty"`
	Notes                 *string  `json:"notes,omitempty"`
	TaggingStatus         *string  `json:"tagging_status,omitempty"`
	Labels                []string `json:"labels"`
}

func toTxnResponse(t sqlcgen.VTransaction, categorySlug *string, labels []string) transactionResponse {
	r := transactionResponse{
		ID:                    t.ID.String(),
		AccountID:             t.AccountID.String(),
		Description:           t.Description,
		DescriptionNormalized: t.DescriptionNormalized,
		Amount:                t.Amount,
		Currency:              t.Currency,
		Direction:             string(t.Direction),
		CounterpartyName:      t.CounterpartyName,
		CounterpartyID:        t.CounterpartyIdentifier,
		Labels:                labels,
	}
	if t.TxnDate.Valid {
		r.TxnDate = t.TxnDate.Time.Format("2006-01-02")
	}
	if t.PaymentMode != nil {
		s := string(*t.PaymentMode)
		r.PaymentMode = &s
	}
	if t.CategoryID.Valid {
		s := uuid.UUID(t.CategoryID.Bytes).String()
		r.CategoryID = &s
	}
	r.CategorySlug = categorySlug
	r.Notes = t.Notes
	if t.TaggingStatus != nil {
		s := string(*t.TaggingStatus)
		r.TaggingStatus = &s
	}
	return r
}

// handleListTransactions handles GET /api/transactions.
func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	var page int32 = 1
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p >= 1 && p <= 100_000 {
		page = int32(p)
	}
	var limit int32 = 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l >= 1 && l <= 200 {
		limit = int32(l)
	}

	params := store.ListTransactionsParams{
		UserID: userID,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
	if v := q.Get("account_id"); v != "" {
		params.AccountID = &v
	}
	if v := q.Get("category_id"); v != "" {
		params.CategoryID = &v
	}
	if v := q.Get("direction"); v != "" {
		d := sqlcgen.TxnDirectionEnum(v)
		params.Direction = &d
	}

	rows, err := s.txnCfg.TxnStore.List(r.Context(), params)
	if err != nil {
		slog.ErrorContext(r.Context(), "list transactions", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	total, err := s.txnCfg.TxnStore.Count(r.Context(), params)
	if err != nil {
		slog.ErrorContext(r.Context(), "count transactions", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Build category slug lookup.
	cats, _ := s.txnCfg.CatStore.List(r.Context())
	catSlug := make(map[string]string, len(cats))
	for _, c := range cats {
		catSlug[c.ID.String()] = c.Slug
	}

	txns := make([]transactionResponse, len(rows))
	for i, row := range rows {
		var slug *string
		if row.CategoryID.Valid {
			if sl, ok := catSlug[uuid.UUID(row.CategoryID.Bytes).String()]; ok {
				slug = &sl
			}
		}
		txns[i] = toTxnResponse(row, slug, []string{})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"transactions": txns,
		"total":        total,
		"page":         page,
	})
}

// handleGetTransaction handles GET /api/transactions/{id}.
func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := mux.Vars(r)["id"]
	txn, err := s.txnCfg.TxnStore.GetByID(r.Context(), id, userID)
	if err != nil {
		slog.DebugContext(r.Context(), "get transaction", bwglogger.Error(err))
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	labels, _ := s.txnCfg.LabelStore.ListForTransaction(r.Context(), txn.ID)
	labelNames := make([]string, len(labels))
	for i, l := range labels {
		labelNames[i] = l.Name
	}

	var catSlug *string
	if txn.CategoryID.Valid {
		cats, _ := s.txnCfg.CatStore.List(r.Context())
		for _, c := range cats {
			if c.ID.String() == uuid.UUID(txn.CategoryID.Bytes).String() {
				sl := c.Slug
				catSlug = &sl
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toTxnResponse(*txn, catSlug, labelNames))
}

type patchEnrichmentRequest struct {
	CategorySlug *string `json:"category_slug"`
	Notes        *string `json:"notes"`
}

// saveTaggingHint persists a tagging_hint memory when a user manually changes a category.
// oldSlug may be empty (transaction previously uncategorised).
func saveTaggingHint(ctx context.Context, memStore memStoreAPI, userID, counterparty, description, oldSlug, newSlug string) {
	hint := fmt.Sprintf(
		"Counterparty '%s' (bank description: '%s') → category '%s'. User manually corrected from '%s'.",
		counterparty, description, newSlug, oldSlug,
	)
	uid := userID
	if _, err := memStore.Save(ctx, &uid, hint, sqlcgen.MemoryTypeEnumTaggingHint, []string{counterparty, newSlug}); err != nil {
		slog.WarnContext(ctx, "save tagging hint", slog.String("counterparty", counterparty), slog.String("new_slug", newSlug), bwglogger.Error(err))
	}
}

// handlePatchEnrichment handles PATCH /api/transactions/{id}/enrichment.
// Updates category and/or notes with tagging_status="manual".
// If the category changed and a counterparty is present, saves a tagging_hint memory.
func (s *Server) handlePatchEnrichment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := mux.Vars(r)["id"]
	txn, err := s.txnCfg.TxnStore.GetByID(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req patchEnrichmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if _, isSizeErr := errors.AsType[*http.MaxBytesError](err); isSizeErr {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ep := store.EnrichmentParams{
		TransactionID: txn.ID.String(),
		Notes:         req.Notes,
	}
	manual := sqlcgen.TaggingStatusEnumManual
	ep.TaggingStatus = &manual

	var newSlug string
	if req.CategorySlug != nil {
		cat, err := s.txnCfg.CatStore.GetBySlug(r.Context(), *req.CategorySlug)
		if err != nil {
			http.Error(w, "unknown category_slug", http.StatusBadRequest)
			return
		}
		catIDStr := cat.ID.String()
		ep.CategoryID = &catIDStr
		newSlug = cat.Slug
	}

	if err := s.txnCfg.TxnStore.UpdateEnrichment(r.Context(), ep); err != nil {
		slog.ErrorContext(r.Context(), "update enrichment", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Auto-save tagging hint when category changed and counterparty is known.
	if req.CategorySlug != nil && newSlug != "" && txn.CounterpartyIdentifier != nil && s.txnCfg.MemStore != nil {
		oldSlug := ""
		if txn.CategoryID.Valid {
			cats, _ := s.txnCfg.CatStore.List(r.Context())
			for _, c := range cats {
				if c.ID == uuid.UUID(txn.CategoryID.Bytes) {
					oldSlug = c.Slug
					break
				}
			}
		}
		if oldSlug != newSlug {
			saveTaggingHint(r.Context(), s.txnCfg.MemStore, userID, *txn.CounterpartyIdentifier, txn.Description, oldSlug, newSlug)
		}
	}

	// Return updated transaction.
	updated, err := s.txnCfg.TxnStore.GetByID(r.Context(), id, userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "fetch updated transaction", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var sl *string
	if req.CategorySlug != nil {
		sl = &newSlug
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toTxnResponse(*updated, sl, nil))
}
