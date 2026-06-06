package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

type mockFDStore struct {
	createFn func(ctx context.Context, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error)
}

func (m *mockFDStore) CreateWithAccount(ctx context.Context, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
	return m.createFn(ctx, p)
}

func newFDTestServer(s fdStoreAPI) *Server {
	return New(":0", okHandler, nil, nil, nil, nil, nil, &FDConfig{Store: s})
}

func validCreateFDBody() string {
	return `{
		"institution": "sbi",
		"principal_amount": 50000,
		"interest_rate": 7.25,
		"tenure_months": 6,
		"start_date": "2026-06-01",
		"maturity_date": "2026-12-01"
	}`
}

func TestHandleCreateFD_HappyPath(t *testing.T) {
	fdID := uuid.New()
	accountID := uuid.New()
	mock := &mockFDStore{
		createFn: func(_ context.Context, _ store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
			return &sqlcgen.FixedDeposit{ID: fdID, AccountID: accountID}, nil
		},
	}

	s := newFDTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(validCreateFDBody())), "uid-1")

	s.handleCreateFD(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp createFDResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, fdID.String(), resp.ID)
	assert.Equal(t, accountID.String(), resp.AccountID)
}

func TestHandleCreateFD_InvalidJSON(t *testing.T) {
	s := newFDTestServer(&mockFDStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader("not-json")), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateFD_MissingInstitution(t *testing.T) {
	body := `{"principal_amount":50000,"interest_rate":7.25,"tenure_months":6,"start_date":"2026-06-01","maturity_date":"2026-12-01"}`
	s := newFDTestServer(&mockFDStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(body)), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateFD_ZeroPrincipal(t *testing.T) {
	body := `{"institution":"sbi","principal_amount":0,"interest_rate":7.25,"tenure_months":6,"start_date":"2026-06-01","maturity_date":"2026-12-01"}`
	s := newFDTestServer(&mockFDStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(body)), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateFD_BadStartDate(t *testing.T) {
	body := `{"institution":"sbi","principal_amount":50000,"interest_rate":7.25,"tenure_months":6,"start_date":"not-a-date","maturity_date":"2026-12-01"}`
	s := newFDTestServer(&mockFDStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(body)), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateFD_BadMaturityDate(t *testing.T) {
	body := `{"institution":"sbi","principal_amount":50000,"interest_rate":7.25,"tenure_months":6,"start_date":"2026-06-01","maturity_date":"bad"}`
	s := newFDTestServer(&mockFDStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(body)), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateFD_InvalidInterestPayout(t *testing.T) {
	body := `{"institution":"sbi","principal_amount":50000,"interest_rate":7.25,"tenure_months":6,"start_date":"2026-06-01","maturity_date":"2026-12-01","interest_payout":"weekly"}`
	s := newFDTestServer(&mockFDStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(body)), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateFD_InvalidAutoRenewalType(t *testing.T) {
	body := `{"institution":"sbi","principal_amount":50000,"interest_rate":7.25,"tenure_months":6,"start_date":"2026-06-01","maturity_date":"2026-12-01","auto_renewal_type":"full"}`
	s := newFDTestServer(&mockFDStore{})
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(body)), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateFD_StoreError(t *testing.T) {
	mock := &mockFDStore{
		createFn: func(_ context.Context, _ store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
			return nil, errors.New("db down")
		},
	}

	s := newFDTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(validCreateFDBody())), "uid-1")

	s.handleCreateFD(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleCreateFD_DefaultsAccountName(t *testing.T) {
	mock := &mockFDStore{
		createFn: func(_ context.Context, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, "sbi FD", p.AccountName)
			return &sqlcgen.FixedDeposit{ID: uuid.New(), AccountID: uuid.New()}, nil
		},
	}

	s := newFDTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(validCreateFDBody())), "uid-1")

	s.handleCreateFD(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleCreateFD_UserIDFromContext(t *testing.T) {
	mock := &mockFDStore{
		createFn: func(_ context.Context, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, "uid-1", p.UserID)
			return &sqlcgen.FixedDeposit{ID: uuid.New(), AccountID: uuid.New()}, nil
		},
	}

	s := newFDTestServer(mock)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/fds", strings.NewReader(validCreateFDBody())), "uid-1")

	s.handleCreateFD(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
}
