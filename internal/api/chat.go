package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"

	"github.com/pushkaranand/finagent/internal/channel"
)

type chatRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

type chatResponse struct {
	Text      string `json:"text"`
	Markdown  bool   `json:"markdown"`
	SessionID string `json:"session_id"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	slog.DebugContext(r.Context(), "chat request",
		slog.String("user_id", userID),
		slog.String("session_id", sessionID),
		slog.Int("text_len", len(req.Text)),
	)

	msg := channel.Message{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		UserID:    userID,
		Text:      req.Text,
		Timestamp: time.Now(),
	}

	resp, err := s.handler(r.Context(), msg)
	if err != nil {
		slog.ErrorContext(r.Context(), "agent handler error",
			slog.String("user_id", userID),
			bwglogger.Error(err),
		)
		http.Error(w, "agent error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chatResponse{
		Text:      resp.Text,
		Markdown:  resp.Markdown,
		SessionID: sessionID,
	})
}
