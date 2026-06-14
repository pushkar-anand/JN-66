package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	bwglogger "github.com/pushkar-anand/build-with-go/logger"
)

// handleListCategories handles GET /api/categories.
func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.txnCfg.CatStore.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list categories", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cats)
}

// handleListLabels handles GET /api/labels.
func (s *Server) handleListLabels(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	labels, err := s.txnCfg.LabelStore.List(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list labels", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(labels)
}
