package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"egobackend/internal/database"
	"egobackend/internal/models"

	"github.com/go-chi/chi/v5"
)

type SessionHandler struct {
	DB *database.DB
}

type UpdateSessionRequest struct {
	Title              *string `json:"title"`
	CustomInstructions *string `json:"custom_instructions"`
}

func (h *SessionHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionID")
	sessionID, err := strconv.Atoi(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	var req UpdateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CustomInstructions == nil && req.Title == nil {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	isOwner, err := h.DB.CheckSessionOwnership(sessionID, user.ID)
	if err != nil {
		http.Error(w, "Server error checking ownership", http.StatusInternalServerError)
		return
	}
	if !isOwner {
		http.Error(w, "Session not found or access denied", http.StatusNotFound)
		return
	}

	if req.CustomInstructions != nil {
		err = h.DB.UpdateSessionInstructions(sessionID, user.ID, *req.CustomInstructions)
		if err != nil {
			log.Printf("Failed to update instructions for session %d: %v", sessionID, err)
			http.Error(w, "Failed to update session instructions", http.StatusInternalServerError)
			return
		}
	}

	if req.Title != nil {
		err = h.DB.UpdateSessionTitle(sessionID, user.ID, *req.Title)
		if err != nil {
			log.Printf("Failed to update title for session %d: %v", sessionID, err)
			http.Error(w, "Failed to update session title", http.StatusInternalServerError)
			return
		}
	}

	session, err := h.DB.GetSessionByID(sessionID, user.ID)
	if err != nil {
		http.Error(w, "Server error fetching updated session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (h *SessionHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Пользователь не найден в контексте", http.StatusInternalServerError)
		return
	}

	sessions, err := h.DB.GetUserSessions(user.ID)
	if err != nil {
		http.Error(w, "Ошибка получения сессий", http.StatusInternalServerError)
		return
	}

	response := make([]models.SessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = models.SessionResponse{
			ID:                 s.ID,
			Title:              s.Title,
			Mode:               s.Mode,
			CustomInstructions: s.CustomInstructions,
			CreatedAt:          s.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *SessionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Пользователь не найден в контексте", http.StatusInternalServerError)
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionID")
	sessionID, err := strconv.Atoi(sessionIDStr)
	if err != nil {
		http.Error(w, "Неверный ID сессии", http.StatusBadRequest)
		return
	}

	isOwner, err := h.DB.CheckSessionOwnership(sessionID, user.ID)
	if err != nil {
		http.Error(w, "Ошибка сервера при проверке сессии", http.StatusInternalServerError)
		return
	}

	if !isOwner {
		http.Error(w, "Сессия не найдена", http.StatusNotFound)
		return
	}

	logs, attachmentsMap, err := h.DB.GetSessionHistory(sessionID, 50)
	if err != nil {
		http.Error(w, "Ошибка получения истории", http.StatusInternalServerError)
		return
	}

	response := make([]models.LogResponse, len(logs))
	for i, l := range logs {
		var attachments []models.FileAttachmentResponse
		if atts, ok := attachmentsMap[l.ID]; ok {
			for _, att := range atts {
				attachments = append(attachments, models.FileAttachmentResponse{
					FileName: att.FileName,
					MimeType: att.MimeType,
				})
			}
		}

		response[i] = models.LogResponse{
			ID:            l.ID,
			UserQuery:     l.UserQuery,
			FinalResponse: l.FinalResponse,
			Timestamp:     l.Timestamp,
			Attachments:   attachments,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *SessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Пользователь не найден в контексте", http.StatusInternalServerError)
		return
	}

	sessionID, _ := strconv.Atoi(chi.URLParam(r, "sessionID"))
	if sessionID == 0 {
		http.Error(w, "Неверный ID сессии", http.StatusBadRequest)
		return
	}

	err := h.DB.DeleteSession(sessionID, user.ID)
	if err != nil {
		http.Error(w, "Ошибка удаления сессии", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Пользователь не найден", http.StatusUnauthorized)
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionID")
	sessionID, err := strconv.Atoi(sessionIDStr)
	if err != nil {
		http.Error(w, "Неверный ID сессии", http.StatusBadRequest)
		return
	}

	session, err := h.DB.GetSessionByID(sessionID, user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Сессия не найдена", http.StatusNotFound)
			return
		}
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}
