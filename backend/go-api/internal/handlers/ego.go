package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"egobackend/internal/database"
	"egobackend/internal/engine"
	"egobackend/internal/models"
	"egobackend/internal/storage"

	"github.com/go-chi/chi/v5"
)

type EgoHandler struct {
	DB               *database.DB
	PythonBackendURL string
	S3Service        *storage.S3Service
}

func (h *EgoHandler) ProccessStream(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		h.writeError(w, "Не удалось получить пользователя из контекста", http.StatusInternalServerError)
		return
	}

	mode := chi.URLParam(r, "mode")
	var req models.StreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Ошибка парсинга JSON-тела запроса", http.StatusBadRequest)
		return
	}
	req.Mode = mode

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, "Клиент не поддерживает стриминг", http.StatusInternalServerError)
		return
	}

	processor := engine.NewProcessor(h.DB, h.PythonBackendURL, h.S3Service)

	callback := func(eventType string, data interface{}) {
		eventData := map[string]interface{}{"type": eventType, "data": data}
		jsonData, _ := json.Marshal(eventData)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
	}

	var sessionID int64
	if req.SessionID != nil {
		sessionID = int64(*req.SessionID)
	}

	go processor.ProcessRequest(req, user, sessionID, callback)
}

func (h *EgoHandler) writeError(w http.ResponseWriter, msg string, code int) {
	log.Printf("!!! HTTP ОШИБКА (%d): %s", code, msg)
	http.Error(w, msg, code)
}
