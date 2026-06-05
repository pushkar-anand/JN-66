package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// testImportAccountID is the account UUID used across import handler tests.
const testImportAccountID = "22222222-2222-2222-2222-222222222222"

type mockImportAccountStore struct {
	listByUserFn    func(ctx context.Context, userID string) ([]sqlcgen.Account, error)
	updateBalanceFn func(ctx context.Context, accountID string, balance model.Money, asOf time.Time) error
}

func (m *mockImportAccountStore) ListByUser(ctx context.Context, userID string) ([]sqlcgen.Account, error) {
	if m.listByUserFn != nil {
		return m.listByUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockImportAccountStore) UpdateBalance(ctx context.Context, accountID string, balance model.Money, asOf time.Time) error {
	if m.updateBalanceFn != nil {
		return m.updateBalanceFn(ctx, accountID, balance, asOf)
	}
	return nil
}

type mockImportUserGetter struct {
	getByIDFn func(ctx context.Context, id string) (*sqlcgen.User, error)
}

func (m *mockImportUserGetter) GetByID(ctx context.Context, id string) (*sqlcgen.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &sqlcgen.User{ID: uuid.New()}, nil
}

func newImportTestServer(userGetter importUserGetter, accountStore importAccountStore) *Server {
	return New(":0", okHandler, nil, nil, nil, nil, &ImportConfig{
		UserGetter:   userGetter,
		AccountStore: accountStore,
		// TxnStore, RunStore, CatStore intentionally nil — tests must not reach imp.Run
	}, nil)
}

// importAccountInList returns an account store mock that reports the test account as owned by the user.
func importAccountInList() *mockImportAccountStore {
	return &mockImportAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{
				{ID: uuid.MustParse(testImportAccountID)},
			}, nil
		},
	}
}

func validImportBody() string {
	return `{
		"account_id": "` + testImportAccountID + `",
		"transactions": [{
			"date": "2025-01-15",
			"description": "Test payment",
			"amount_paise": 50000,
			"direction": "debit"
		}]
	}`
}

func TestHandleImport_InvalidJSON(t *testing.T) {
	s := newImportTestServer(&mockImportUserGetter{}, &mockImportAccountStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader("not-json")), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImport_MissingAccountID(t *testing.T) {
	s := newImportTestServer(&mockImportUserGetter{}, &mockImportAccountStore{})
	body := `{"transactions":[{"date":"2025-01-15","description":"x","amount_paise":100,"direction":"debit"}]}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(body)), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImport_EmptyTransactions(t *testing.T) {
	s := newImportTestServer(&mockImportUserGetter{}, &mockImportAccountStore{})
	body := `{"account_id":"` + testImportAccountID + `","transactions":[]}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(body)), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImport_BadDate(t *testing.T) {
	s := newImportTestServer(&mockImportUserGetter{}, &mockImportAccountStore{})
	body := `{"account_id":"` + testImportAccountID + `","transactions":[{
		"date":"not-a-date","description":"x","amount_paise":100,"direction":"debit"
	}]}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(body)), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImport_ZeroAmount(t *testing.T) {
	s := newImportTestServer(&mockImportUserGetter{}, &mockImportAccountStore{})
	body := `{"account_id":"` + testImportAccountID + `","transactions":[{
		"date":"2025-01-15","description":"x","amount_paise":0,"direction":"debit"
	}]}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(body)), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImport_InvalidDirection(t *testing.T) {
	s := newImportTestServer(&mockImportUserGetter{}, &mockImportAccountStore{})
	body := `{"account_id":"` + testImportAccountID + `","transactions":[{
		"date":"2025-01-15","description":"x","amount_paise":100,"direction":"transfer"
	}]}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(body)), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImport_AccountNotFound(t *testing.T) {
	accountStore := &mockImportAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			// Return a different account — not the one being imported into.
			return []sqlcgen.Account{{ID: uuid.New()}}, nil
		},
	}

	s := newImportTestServer(&mockImportUserGetter{}, accountStore)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(validImportBody())), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleImport_NoAccounts(t *testing.T) {
	accountStore := &mockImportAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return []sqlcgen.Account{}, nil
		},
	}

	s := newImportTestServer(&mockImportUserGetter{}, accountStore)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(validImportBody())), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleImport_ListByUserError(t *testing.T) {
	accountStore := &mockImportAccountStore{
		listByUserFn: func(_ context.Context, _ string) ([]sqlcgen.Account, error) {
			return nil, errors.New("db error")
		},
	}

	s := newImportTestServer(&mockImportUserGetter{}, accountStore)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(validImportBody())), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleImport_GetUserError(t *testing.T) {
	userGetter := &mockImportUserGetter{
		getByIDFn: func(_ context.Context, _ string) (*sqlcgen.User, error) {
			return nil, errors.New("user not found")
		},
	}

	s := newImportTestServer(userGetter, importAccountInList())
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(validImportBody())), "uid-1")

	s.handleImport(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleImport_RFC3339Date(t *testing.T) {
	userGetter := &mockImportUserGetter{
		getByIDFn: func(_ context.Context, _ string) (*sqlcgen.User, error) {
			return nil, errors.New("stop before importer")
		},
	}

	// RFC3339 date should pass validation; test exercises the validator path only.
	body := `{"account_id":"` + testImportAccountID + `","transactions":[{
		"date":"2025-01-15T00:00:00Z","description":"x","amount_paise":100,"direction":"credit"
	}]}`
	s := newImportTestServer(userGetter, importAccountInList())
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(body)), "uid-1")

	s.handleImport(w, r)

	// Validation passed; GetByID failed → 500, not 400.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
