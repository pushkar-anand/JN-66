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

	"github.com/pushkaranand/finagent/internal/apikey"
	"github.com/pushkaranand/finagent/internal/channel"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

func okHandler(_ context.Context, msg channel.Message) (channel.Response, error) {
	return channel.Response{Text: "reply to: " + msg.Text, Markdown: true}, nil
}

func errHandler(_ context.Context, _ channel.Message) (channel.Response, error) {
	return channel.Response{}, errors.New("agent boom")
}

func newTestServer(h channel.MessageHandler) *Server {
	return New(":0", h, nil, nil, nil)
}

func requestWithUser(r *http.Request, userID string) *http.Request {
	return r.WithContext(WithUserID(r.Context(), userID))
}

func TestHandleLive(t *testing.T) {
	s := newTestServer(okHandler)
	w := httptest.NewRecorder()
	s.handleLive(w, httptest.NewRequest(http.MethodGet, "/healthz/live", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestHandleReady_NoDB(t *testing.T) {
	s := newTestServer(okHandler) // db=nil → always ready
	w := httptest.NewRecorder()
	s.handleReady(w, httptest.NewRequest(http.MethodGet, "/healthz/ready", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestHandleChat_HappyPath(t *testing.T) {
	s := newTestServer(okHandler)
	body := `{"text":"hello","session_id":"sess-1"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body)), "uid-1")

	s.handleChat(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp chatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Text, "hello")
	assert.True(t, resp.Markdown)
	assert.Equal(t, "sess-1", resp.SessionID)
}

func TestHandleChat_EmptySessionIDGeneratesOne(t *testing.T) {
	s := newTestServer(okHandler)
	body := `{"text":"hello"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body)), "uid-1")

	s.handleChat(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp chatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.SessionID)
}

func TestHandleChat_MissingText(t *testing.T) {
	s := newTestServer(okHandler)
	body := `{"text":""}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body)), "uid-1")

	s.handleChat(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleChat_MissingUserID(t *testing.T) {
	s := newTestServer(okHandler)
	body := `{"text":"hello"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))

	s.handleChat(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleChat_InvalidJSON(t *testing.T) {
	s := newTestServer(okHandler)
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("not-json")), "uid-1")

	s.handleChat(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleChat_AgentError(t *testing.T) {
	s := newTestServer(errHandler)
	body := `{"text":"hello"}`
	w := httptest.NewRecorder()
	r := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body)), "uid-1")

	s.handleChat(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

// mockUserLookup is a test double for userLookup.
type mockUserLookup struct {
	user *sqlcgen.User
	err  error
}

func (m *mockUserLookup) GetByAPIKeyPrefix(_ context.Context, _ string) (*sqlcgen.User, error) {
	return m.user, m.err
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	s := New(":0", okHandler, &mockUserLookup{err: errors.New("no user")}, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"text":"hi"}`))
	r.Header.Set("Content-Type", "application/json")

	s.srv.Handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	s := New(":0", okHandler, &mockUserLookup{err: errors.New("not found")}, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"text":"hi"}`))
	r.Header.Set("Authorization", "Bearer wrongtoken")

	s.srv.Handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	uid := uuid.New()
	// Token must be at least 16 chars (prefix length); use a realistic 64-char hex token.
	token := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	hash, err := apikey.Hash(token)
	require.NoError(t, err)

	s := New(":0", okHandler, &mockUserLookup{user: &sqlcgen.User{ID: uid, ApiKeyHash: hash}}, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"text":"hi"}`))
	r.Header.Set("Authorization", "Bearer "+token)

	s.srv.Handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}
