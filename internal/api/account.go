package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// accountStoreAPI is the account-store surface needed by the accounts handlers.
type accountStoreAPI interface {
	Create(ctx context.Context, p store.CreateAccountParams, userID string) (*sqlcgen.Account, error)
	ListByUser(ctx context.Context, userID string) ([]sqlcgen.Account, error)
}

// AccountsConfig holds dependencies for the accounts API routes.
// Pass nil to api.New to disable these routes.
type AccountsConfig struct {
	Store accountStoreAPI
}

type createAccountRequest struct {
	Institution       string `json:"institution"         validate:"required"`
	Name              string `json:"name"                validate:"required"`
	AccountType       string `json:"account_type"        validate:"required,oneof=savings current salary credit_card loan wallet fd ppf"`
	ExternalAccountID string `json:"external_account_id" validate:"required"`
}

type accountResponse struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Institution       string `json:"institution"`
	ExternalAccountID string `json:"external_account_id"`
	AccountType       string `json:"account_type"`
	AccountClass      string `json:"account_class"`
	Currency          string `json:"currency"`
	IsActive          bool   `json:"is_active"`
}

func toAccountResponse(a sqlcgen.Account) accountResponse {
	return accountResponse{
		ID:                a.ID.String(),
		Name:              a.Name,
		Institution:       a.Institution,
		ExternalAccountID: a.ExternalAccountID,
		AccountType:       string(a.AccountType),
		AccountClass:      string(a.AccountClass),
		Currency:          a.Currency,
		IsActive:          a.IsActive,
	}
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createAccountRequest
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
		slog.ErrorContext(r.Context(), "account create: validator error", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !result.Valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(result.Failed)
		return
	}

	accountType, err := parseAccountType(req.AccountType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := UserIDFromContext(r.Context())

	acc, err := s.accountsCfg.Store.Create(r.Context(), store.CreateAccountParams{
		Institution:       req.Institution,
		ExternalAccountID: req.ExternalAccountID,
		Name:              req.Name,
		AccountType:       accountType,
		Currency:          "INR",
		IsActive:          true,
	}, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "account already exists for this institution and account number", http.StatusConflict)
			return
		}
		slog.ErrorContext(r.Context(), "account create: store error",
			slog.String("user_id", userID),
			bwglogger.Error(err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "account created",
		slog.String("user_id", userID),
		slog.String("account_id", acc.ID.String()),
		slog.String("institution", acc.Institution),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAccountResponse(*acc))
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	accounts, err := s.accountsCfg.Store.ListByUser(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "account list: store error",
			slog.String("user_id", userID),
			bwglogger.Error(err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = toAccountResponse(a)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func parseAccountType(s string) (sqlcgen.AccountTypeEnum, error) {
	m := map[string]sqlcgen.AccountTypeEnum{
		"savings":     sqlcgen.AccountTypeEnumBankSavings,
		"current":     sqlcgen.AccountTypeEnumBankCurrent,
		"salary":      sqlcgen.AccountTypeEnumBankSalary,
		"credit_card": sqlcgen.AccountTypeEnumCreditCard,
		"loan":        sqlcgen.AccountTypeEnumLoan,
		"wallet":      sqlcgen.AccountTypeEnumWallet,
		"fd":          sqlcgen.AccountTypeEnumFd,
		"ppf":         sqlcgen.AccountTypeEnumPpf,
	}
	t, ok := m[strings.ToLower(s)]
	if !ok {
		return "", fmt.Errorf("unknown account type %q — valid: savings, current, salary, credit_card, loan, wallet, fd, ppf", s)
	}
	return t, nil
}
