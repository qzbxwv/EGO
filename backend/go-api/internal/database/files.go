package database

import (
	"context"
	"egobackend/internal/models"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

func (db *DB) SaveFileAttachment(sessionID, userID int, fileName, fileURI, mimeType, status string) (int64, error) {
	query := `INSERT INTO file_attachments (session_id, user_id, file_name, file_uri, mime_type, status, created_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	var fileID int64
	err := db.QueryRow(query, sessionID, userID, fileName, fileURI, mimeType, status, time.Now().UTC()).Scan(&fileID)
	return fileID, err
}

func (db *DB) AssociateFilesWithRequestLog(logID int64, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}

	tx, err := db.BeginTxx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if err != nil {
			log.Printf("!!! DB ОШИБКА: Откат транзакции из-за ошибки: %v", err)
			tx.Rollback()
		}
	}()

	stmt, err := tx.Preparex("UPDATE file_attachments SET request_log_id = $1 WHERE id = $2")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, fileID := range fileIDs {
		if _, err = stmt.Exec(logID, fileID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) GetAttachmentsByIDs(ids []int) ([]models.FileAttachment, error) {
	if len(ids) == 0 {
		return []models.FileAttachment{}, nil
	}
	query, args, err := sqlx.In("SELECT * FROM file_attachments WHERE id IN (?)", ids)
	if err != nil {
		return nil, err
	}
	query = db.Rebind(query)
	var attachments []models.FileAttachment
	err = db.Select(&attachments, query, args...)
	if err != nil {
		return nil, err
	}
	return attachments, nil
}

func (db *DB) DeleteOldFileAttachments(maxAge time.Duration) ([]string, error) {
	cutoffTime := time.Now().UTC().Add(-maxAge)
	query := `DELETE FROM file_attachments WHERE created_at < $1 RETURNING file_uri`

	var deletedURIs []string
	err := db.Select(&deletedURIs, query, cutoffTime)
	if err != nil {
		return nil, err
	}
	return deletedURIs, nil
}
