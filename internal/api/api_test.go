package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkaranand/finagent/internal/channel"
)

func okHandler(_ context.Context, msg channel.Message) (channel.Response, error) {
	return channel.Response{Text: "reply to: " + msg.Text, Markdown: true}, nil
}

func errHandler(_ context.Context, _ channel.Message) (channel.Response, error) {
	return channel.Response{}, errors.New("agent boom")
}

func newTestServer(h channel.MessageHandler) *Server {
	return New(":0", h)
}

func TestHandleHealth(t *testing.T) {
	s := newTestServer(okHandler)
	w := httptest.NewRecorder()
	s.handleHealth(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestHandleChat_HappyPath(t *testing.T) {
	s := newTestServer(okHandler)
	body := `{"user_id":"uid-1","text":"hello","session_id":"sess-1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))

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
	body := `{"user_id":"uid-1","text":"hello"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))

	s.handleChat(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp chatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.SessionID)
}

func TestHandleChat_MissingText(t *testing.T) {
	s := newTestServer(okHandler)
	body := `{"user_id":"uid-1","text":""}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))

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
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("not-json"))

	s.handleChat(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleChat_AgentError(t *testing.T) {
	s := newTestServer(errHandler)
	body := `{"user_id":"uid-1","text":"hello"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))

	s.handleChat(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "agent boom")
}
