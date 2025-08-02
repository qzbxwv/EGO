package handlers

import (
	"egobackend/internal/models"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *SessionHandler) EditLog(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	logIDStr := chi.URLParam(r, "logID")
	logID, err := strconv.ParseInt(logIDStr, 10, 64)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid log ID")
		return
	}

	var req models.UpdateLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Query == "" {
		RespondWithError(w, http.StatusBadRequest, "Query cannot be empty")
		return
	}

	logToEdit, err := h.DB.GetRequestLogByID(logID, user.ID)
	if err != nil || logToEdit == nil {
		RespondWithError(w, http.StatusNotFound, "Log not found or access denied")
		return
	}

	err = h.DB.UpdateRequestLogQuery(logID, user.ID, req.Query)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to update log")
		return
	}

	w.WriteHeader(http.StatusOK)
}
