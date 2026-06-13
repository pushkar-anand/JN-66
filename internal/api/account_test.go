package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

type mockAccountStore struct {
	createFn        func(ctx context.Context, p store.CreateAccountParams, userID string) (*sqlcgen.Account, error)
	listByUserFn    func(ctx context.Context, userID string) ([]sqlcgen.Account, error)
	updateBalanceFn func(ctx context.Context, accountID string, balance model.Money, asOf time.Time) error
}

func (m *mockAccountStore) Create(ctx context.Context, p store.CreateAccountParams, userID string) (*sqlcgen.Account, error) {
	return m.createFn(ctx, p, userID)
}

func (m *mockAccountStore) ListByUser(ctx context.Context, userID string) ([]sqlcgen.Account, error) {
	return m.listByUserFn(ctx, userID)
}

func (m *mockAccountStore) UpdateBalance(ctx context.Context, accountID string, balance model.Money, asOf time.Time) error {
	return m.updateBalanceFn(ctx, accountID, balance, asOf)
}

func newAccountsTestServer(s accountStoreAPI) *Server {
	return New(":0", okHandler, nil, nil, nil, &AccountsConfig{Store: s}, nil, nil)
}

func TestHandleCreateAccount_HappyPath(t *testing.T) {
	aid := uuid.New()
	mock := &mockAccountStore{
		createFn: func(_ context.Context, _ store.CreateAccountParams, _ string) (*sqlcgen.Account, error) {
			return &sqlcgen.Account{
				ID:                aid,
				Institution:       "HDFC",
				ExternalAccountID: "123456",
				Name:              "Test Account",
				AccountType:       sqlcgen.AccountTypeEnumBankSavings,
				AccountClass:      sqlcgen.AccountClassEnumAsset,
				Currency:          "INR",
				IsActive:          true,
			}, nil
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"institution":"HDFC","name":"Test Account","account_type":"savings","external_account_id":"123456"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body)), "uid-1")

	s.handleCreateAccount(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp accountResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, aid.String(), resp.ID)
	assert.Equal(t, "HDFC", resp.Institution)
	assert.Equal(t, "bank_savings", resp.AccountType)
}

func TestHandleCreateAccount_Duplicate(t *testing.T) {
	mock := &mockAccountStore{
		createFn: func(_ context.Context, _ store.CreateAccountParams, _ string) (*sqlcgen.Account, error) {
			return nil, &pgconn.PgError{Code: "23505", ConstraintName: "uq_accounts_institution_ext_account_id"}
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"institution":"HDFC","name":"Test Account","account_type":"savings","external_account_id":"123456"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body)), "uid-1")

	s.handleCreateAccount(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandleCreateAccount_InvalidJSON(t *testing.T) {
	s := newAccountsTestServer(&mockAccountStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader("not-json")), "uid-1")

	s.handleCreateAccount(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateAccount_MissingFields(t *testing.T) {
	s := newAccountsTestServer(&mockAccountStore{})
	body := `{"institution":"HDFC"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body)), "uid-1")

	s.handleCreateAccount(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateAccount_UnknownAccountType(t *testing.T) {
	s := newAccountsTestServer(&mockAccountStore{})
	body := `{"institution":"HDFC","name":"Test","account_type":"unknown","external_account_id":"123"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body)), "uid-1")

	s.handleCreateAccount(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateAccount_StoreError(t *testing.T) {
	mock := &mockAccountStore{
		createFn: func(_ context.Context, _ store.CreateAccountParams, _ string) (*sqlcgen.Account, error) {
			return nil, errors.New("db down")
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"institution":"HDFC","name":"Test Account","account_type":"savings","external_account_id":"123456"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body)), "uid-1")

	s.handleCreateAccount(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListAccounts_EmptyList(t *testing.T) {
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{}, nil
		},
	}

	s := newAccountsTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/accounts", nil), "uid-1")

	s.handleListAccounts(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []accountResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp)
}

func TestHandleListAccounts_WithAccounts(t *testing.T) {
	aid := uuid.New()
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{
				{
					ID:                aid,
					Institution:       "ICICI",
					ExternalAccountID: "999888",
					Name:              "Savings",
					AccountType:       sqlcgen.AccountTypeEnumBankSavings,
					AccountClass:      sqlcgen.AccountClassEnumAsset,
					Currency:          "INR",
					IsActive:          true,
				},
			}, nil
		},
	}

	s := newAccountsTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/accounts", nil), "uid-1")

	s.handleListAccounts(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []accountResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, aid.String(), resp[0].ID)
	assert.Equal(t, "ICICI", resp[0].Institution)
}

func TestHandleListAccounts_StoreError(t *testing.T) {
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return nil, errors.New("db error")
		},
	}

	s := newAccountsTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/accounts", nil), "uid-1")

	s.handleListAccounts(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func balanceRequest(r *http.Request, accountID string) *http.Request {
	return mux.SetURLVars(r, map[string]string{"id": accountID})
}

func TestHandleUpdateAccountBalance_HappyPath(t *testing.T) {
	aid := uuid.New()
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{{ID: aid}}, nil
		},
		updateBalanceFn: func(_ context.Context, _ string, _ model.Money, _ time.Time) error {
			return nil
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"balance_paise":150000,"as_of":"2026-06-01"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPatch, "/api/accounts/"+aid.String()+"/balance", strings.NewReader(body)), "uid-1")
	r = balanceRequest(r, aid.String())

	s.handleUpdateAccountBalance(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandleUpdateAccountBalance_AccountNotOwned(t *testing.T) {
	ownedID := uuid.New()
	otherID := uuid.New()
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{{ID: ownedID}}, nil
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"balance_paise":150000,"as_of":"2026-06-01"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPatch, "/api/accounts/"+otherID.String()+"/balance", strings.NewReader(body)), "uid-1")
	r = balanceRequest(r, otherID.String())

	s.handleUpdateAccountBalance(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleUpdateAccountBalance_ListStoreError(t *testing.T) {
	aid := uuid.New()
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return nil, errors.New("db error")
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"balance_paise":150000,"as_of":"2026-06-01"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPatch, "/api/accounts/"+aid.String()+"/balance", strings.NewReader(body)), "uid-1")
	r = balanceRequest(r, aid.String())

	s.handleUpdateAccountBalance(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateAccountBalance_InvalidJSON(t *testing.T) {
	aid := uuid.New()
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{{ID: aid}}, nil
		},
	}

	s := newAccountsTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPatch, "/api/accounts/"+aid.String()+"/balance", strings.NewReader("not-json")), "uid-1")
	r = balanceRequest(r, aid.String())

	s.handleUpdateAccountBalance(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateAccountBalance_MissingAsOf(t *testing.T) {
	aid := uuid.New()
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{{ID: aid}}, nil
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"balance_paise":150000}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPatch, "/api/accounts/"+aid.String()+"/balance", strings.NewReader(body)), "uid-1")
	r = balanceRequest(r, aid.String())

	s.handleUpdateAccountBalance(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateAccountBalance_UpdateStoreError(t *testing.T) {
	aid := uuid.New()
	mock := &mockAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{{ID: aid}}, nil
		},
		updateBalanceFn: func(_ context.Context, _ string, _ model.Money, _ time.Time) error {
			return errors.New("db error")
		},
	}

	s := newAccountsTestServer(mock)
	body := `{"balance_paise":150000,"as_of":"2026-06-01"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPatch, "/api/accounts/"+aid.String()+"/balance", strings.NewReader(body)), "uid-1")
	r = balanceRequest(r, aid.String())

	s.handleUpdateAccountBalance(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
