package database

import (
	"database/sql"
	"egobackend/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

func (db *DB) GetUserSessions(userID int) ([]models.ChatSession, error) {
	var sessions []models.ChatSession
	query := `SELECT id, user_id, title, mode, custom_instructions, created_at FROM chat_sessions WHERE user_id = $1 ORDER BY created_at DESC`
	err := db.Select(&sessions, query, userID)
	return sessions, err
}

func (db *DB) GetSessionHistory(sessionID int, limit int) ([]models.RequestLog, map[int][]models.FileAttachment, error) {
	var logs []models.RequestLog
	query := `
        SELECT id, session_id, user_query, ego_thoughts_json, final_response, 
               prompt_tokens, completion_tokens, total_tokens, attached_file_ids, timestamp 
        FROM request_logs 
        WHERE session_id = $1 
        ORDER BY timestamp DESC 
        LIMIT $2`
	err := db.Select(&logs, query, sessionID, limit)
	if err != nil {
		return nil, nil, err
	}

	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	attachmentsMap := make(map[int][]models.FileAttachment)
	var allFileIDs []int64
	logIDMap := make(map[int64]int)

	for _, log := range logs {
		var fileIDs []int64
		if log.AttachedFileIDs != "" && log.AttachedFileIDs != "[]" {
			if err := json.Unmarshal([]byte(log.AttachedFileIDs), &fileIDs); err == nil {
				allFileIDs = append(allFileIDs, fileIDs...)
				for _, fileID := range fileIDs {
					logIDMap[fileID] = log.ID
				}
			}
		}
	}

	if len(allFileIDs) > 0 {
		var attachments []models.FileAttachment
		query, args, err := sqlx.In("SELECT id, session_id, user_id, file_name, mime_type, status, created_at, request_log_id FROM file_attachments WHERE id IN (?)", allFileIDs)
		if err != nil {
			return logs, nil, err
		}
		query = db.Rebind(query)
		err = db.Select(&attachments, query, args...)
		if err != nil {
			return logs, nil, err
		}

		for _, att := range attachments {
			logID, ok := logIDMap[att.ID]
			if ok {
				attachmentsMap[logID] = append(attachmentsMap[logID], att)
			}
		}
	}

	return logs, attachmentsMap, nil
}

func (db *DB) DeleteSession(sessionID, userID int) error {
	query := `DELETE FROM chat_sessions WHERE id = $1 AND user_id = $2`
	_, err := db.Exec(query, sessionID, userID)
	return err
}

func (db *DB) CheckSessionOwnership(sessionID, userID int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM chat_sessions WHERE id = $1 AND user_id = $2)`
	err := db.Get(&exists, query, sessionID, userID)
	return exists, err
}

func (db *DB) GetOrCreateSession(sessionIDStr string, title string, userID int, mode string) (*models.ChatSession, error) {
	if sessionIDStr != "" && sessionIDStr != "new" {
		sessionID, err := strconv.Atoi(sessionIDStr)
		if err != nil {
			return nil, fmt.Errorf("неверный формат session_id")
		}

		var session models.ChatSession
		err = db.Get(&session, "SELECT id, user_id, title, mode, custom_instructions, created_at FROM chat_sessions WHERE id = $1 AND user_id = $2", sessionID, userID)
		if err == nil {
			log.Printf("Найдена существующая сессия %d для пользователя %d", sessionID, userID)
			return &session, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	log.Printf("Создание новой сессии для пользователя %d с заголовком '%s'", userID, title)
	if mode == "" {
		mode = "fast"
	}
	session := models.ChatSession{
		UserID:    userID,
		Title:     title,
		Mode:      mode,
		CreatedAt: time.Now().UTC(),
	}

	query := `INSERT INTO chat_sessions (user_id, title, mode, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	var newID int
	err := db.QueryRow(query, session.UserID, session.Title, session.Mode, session.CreatedAt).Scan(&newID)
	if err != nil {
		return nil, err
	}

	session.ID = newID
	return &session, nil
}

func (db *DB) SaveRequestLog(logEntry *models.RequestLog) (int64, error) {
	query := `INSERT INTO request_logs (
				  session_id, user_query, ego_thoughts_json, final_response, 
				  prompt_tokens, completion_tokens, total_tokens, attached_file_ids, timestamp
			  ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`

	var logID int64
	err := db.QueryRow(
		query,
		logEntry.SessionID,
		logEntry.UserQuery,
		logEntry.EgoThoughtsJSON,
		logEntry.FinalResponse,
		logEntry.PromptTokens,
		logEntry.CompletionTokens,
		logEntry.TotalTokens,
		logEntry.AttachedFileIDs,
		logEntry.Timestamp,
	).Scan(&logID)

	return logID, err
}

func (db *DB) UpdateSessionInstructions(sessionID, userID int, customInstructions string) error {
	query := `UPDATE chat_sessions SET custom_instructions = $1 WHERE id = $2 AND user_id = $3`
	_, err := db.Exec(query, customInstructions, sessionID, userID)
	return err
}

func (db *DB) GetSessionByID(sessionID, userID int) (*models.ChatSession, error) {
	var session models.ChatSession
	query := "SELECT id, user_id, title, mode, custom_instructions, created_at FROM chat_sessions WHERE id = $1 AND user_id = $2"
	err := db.Get(&session, query, sessionID, userID)
	return &session, err
}
