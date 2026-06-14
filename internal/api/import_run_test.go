package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// testRunID is the import run UUID used across get-run handler tests.
const testRunID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

type mockImportRunGetter struct {
	getFn func(ctx context.Context, userID, runID uuid.UUID) (*sqlcgen.ImportRun, error)
}

func (m *mockImportRunGetter) Get(ctx context.Context, userID, runID uuid.UUID) (*sqlcgen.ImportRun, error) {
	if m.getFn != nil {
		return m.getFn(ctx, userID, runID)
	}
	return nil, pgx.ErrNoRows
}

func newGetRunTestServer(getter importRunGetter) *Server {
	return New(":0", okHandler, nil, nil, nil, nil, &ImportConfig{
		RunGetter: getter,
	}, nil, nil, nil)
}

func requestWithRunID(userID, runID string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/import/"+runID, nil)
	r = mux.SetURLVars(r, map[string]string{"id": runID})
	return requestWithUser(r, userID)
}

// stubRun returns a minimal ImportRun for the given user and run IDs.
func stubRun(userID, runID string) *sqlcgen.ImportRun {
	now := pgtype.Timestamptz{Time: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC), Valid: true}
	fin := pgtype.Timestamptz{Time: time.Date(2026, 1, 15, 10, 0, 5, 0, time.UTC), Valid: true}
	return &sqlcgen.ImportRun{
		ID:            uuid.MustParse(runID),
		UserID:        uuid.MustParse(userID),
		Status:        sqlcgen.ImportStatusEnumSuccess,
		RowsParsed:    10,
		RowsInserted:  8,
		RowsDuplicate: 2,
		RowsFailed:    0,
		StartedAt:     now,
		FinishedAt:    fin,
	}
}

func TestHandleGetImportRun_InvalidID(t *testing.T) {
	s := newGetRunTestServer(&mockImportRunGetter{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/import/not-a-uuid", nil)
	r = mux.SetURLVars(r, map[string]string{"id": "not-a-uuid"})
	r = requestWithUser(r, "11111111-1111-1111-1111-111111111111")

	s.handleGetImportRun(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetImportRun_NotFound(t *testing.T) {
	getter := &mockImportRunGetter{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*sqlcgen.ImportRun, error) {
			return nil, pgx.ErrNoRows
		},
	}
	s := newGetRunTestServer(getter)
	w := httptest.NewRecorder()

	s.handleGetImportRun(w, requestWithRunID("11111111-1111-1111-1111-111111111111", testRunID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleGetImportRun_NotFoundWrapped(t *testing.T) {
	// pgx.ErrNoRows wrapped in a store error must still produce 404.
	getter := &mockImportRunGetter{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*sqlcgen.ImportRun, error) {
			return nil, errors.Join(errors.New("get import run"), pgx.ErrNoRows)
		},
	}
	s := newGetRunTestServer(getter)
	w := httptest.NewRecorder()

	s.handleGetImportRun(w, requestWithRunID("11111111-1111-1111-1111-111111111111", testRunID))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleGetImportRun_StoreError(t *testing.T) {
	getter := &mockImportRunGetter{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*sqlcgen.ImportRun, error) {
			return nil, errors.New("db connection reset")
		},
	}
	s := newGetRunTestServer(getter)
	w := httptest.NewRecorder()

	s.handleGetImportRun(w, requestWithRunID("11111111-1111-1111-1111-111111111111", testRunID))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleGetImportRun_Success(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	run := stubRun(userID, testRunID)
	var capturedUserID, capturedRunID uuid.UUID

	getter := &mockImportRunGetter{
		getFn: func(_ context.Context, uid, rid uuid.UUID) (*sqlcgen.ImportRun, error) {
			capturedUserID = uid
			capturedRunID = rid
			return run, nil
		},
	}
	s := newGetRunTestServer(getter)
	w := httptest.NewRecorder()

	s.handleGetImportRun(w, requestWithRunID(userID, testRunID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, uuid.MustParse(userID), capturedUserID)
	assert.Equal(t, uuid.MustParse(testRunID), capturedRunID)

	var resp importRunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, testRunID, resp.ID)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, int32(10), resp.Parsed)
	assert.Equal(t, int32(8), resp.Inserted)
	assert.Equal(t, int32(2), resp.Duplicate)
	assert.Equal(t, int32(0), resp.Failed)
	assert.NotEmpty(t, resp.StartedAt)
	assert.NotEmpty(t, resp.FinishedAt)
	assert.Empty(t, resp.AccountID) // not set in stub
}

func TestHandleGetImportRun_WithAccountID(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	run := stubRun(userID, testRunID)
	accountUUID := uuid.MustParse(testImportAccountID)
	run.AccountID = pgtype.UUID{Bytes: accountUUID, Valid: true}

	getter := &mockImportRunGetter{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*sqlcgen.ImportRun, error) {
			return run, nil
		},
	}
	s := newGetRunTestServer(getter)
	w := httptest.NewRecorder()

	s.handleGetImportRun(w, requestWithRunID(userID, testRunID))

	require.Equal(t, http.StatusOK, w.Code)
	var resp importRunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, testImportAccountID, resp.AccountID)
}

func TestHandleGetImportRun_RunningStatus(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	run := stubRun(userID, testRunID)
	run.Status = sqlcgen.ImportStatusEnumRunning
	run.FinishedAt = pgtype.Timestamptz{Valid: false} // not finished yet

	getter := &mockImportRunGetter{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*sqlcgen.ImportRun, error) {
			return run, nil
		},
	}
	s := newGetRunTestServer(getter)
	w := httptest.NewRecorder()

	s.handleGetImportRun(w, requestWithRunID(userID, testRunID))

	require.Equal(t, http.StatusOK, w.Code)
	var resp importRunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "running", resp.Status)
	assert.Empty(t, resp.FinishedAt)
}
