package database

import (
	"database/sql"
	"egobackend/internal/models"
	"fmt"
	"log"
	"strconv"
	"time"
)

func (db *DB) GetUserSessions(userID int) ([]models.ChatSession, error) {
	var sessions []models.ChatSession
	query := `SELECT id, user_id, title, mode, custom_instructions, created_at FROM chat_sessions WHERE user_id = $1 ORDER BY created_at DESC`
	err := db.Select(&sessions, query, userID)
	return sessions, err
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

func (db *DB) GetOrCreateSession(sessionIDStr string, title string, userID int, mode string) (*models.ChatSession, bool, error) {
	if sessionIDStr != "" && sessionIDStr != "new" {
		sessionID, err := strconv.Atoi(sessionIDStr)
		if err != nil {
			return nil, false, fmt.Errorf("неверный формат session_id")
		}

		var session models.ChatSession
		err = db.Get(&session, "SELECT id, user_id, title, mode, custom_instructions, created_at FROM chat_sessions WHERE id = $1 AND user_id = $2", sessionID, userID)
		if err == nil {
			log.Printf("Найдена существующая сессия %d для пользователя %d", sessionID, userID)
			return &session, false, nil
		}
		if err != sql.ErrNoRows {
			return nil, false, err
		}
	}

	log.Printf("Создание новой сессии для пользователя %d с заголовком '%s'", userID, title)
	if mode == "" {
		mode = "default"
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
		return nil, false, err
	}

	session.ID = newID
	return &session, true, nil
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
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &session, err
}

func (db *DB) UpdateSessionTitle(sessionID, userID int, title string) error {
	query := `UPDATE chat_sessions SET title = $1 WHERE id = $2 AND user_id = $3`
	_, err := db.Exec(query, title, sessionID, userID)
	return err
}
