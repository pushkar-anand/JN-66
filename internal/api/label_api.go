package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"
)

type addLabelRequest struct {
	Name string `json:"name" validate:"required"`
}

// handleAddLabel handles POST /api/transactions/{id}/labels.
func (s *Server) handleAddLabel(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	txnID := mux.Vars(r)["id"]

	// Verify the transaction belongs to this user before touching its labels.
	if _, err := s.txnCfg.TxnStore.GetByID(r.Context(), txnID, userID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req addLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if _, isSizeErr := errors.AsType[*http.MaxBytesError](err); isSizeErr {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}

	labelID, err := s.txnCfg.LabelStore.FindOrCreate(r.Context(), userID, req.Name)
	if err != nil {
		slog.ErrorContext(r.Context(), "find or create label", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := s.txnCfg.LabelStore.AddToTransaction(r.Context(), txnID, labelID); err != nil {
		slog.ErrorContext(r.Context(), "add label to transaction", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRemoveLabel handles DELETE /api/transactions/{id}/labels/{labelId}.
func (s *Server) handleRemoveLabel(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	txnID := vars["id"]
	labelID := vars["labelId"]

	if _, err := s.txnCfg.TxnStore.GetByID(r.Context(), txnID, userID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if err := s.txnCfg.LabelStore.RemoveFromTransaction(r.Context(), txnID, labelID); err != nil {
		slog.ErrorContext(r.Context(), "remove label from transaction", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
