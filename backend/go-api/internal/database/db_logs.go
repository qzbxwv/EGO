package database

import (
	"database/sql"
	"egobackend/internal/models"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
)

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

	attachmentsMap, err := db.getAttachmentsForLogs(logs)
	if err != nil {
		return logs, nil, err
	}

	return logs, attachmentsMap, nil
}

func (db *DB) GetSessionHistoryBefore(sessionID int, beforeTime time.Time, limit int) ([]models.RequestLog, map[int][]models.FileAttachment, error) {
	var logs []models.RequestLog
	query := `
        SELECT id, session_id, user_query, ego_thoughts_json, final_response, attached_file_ids, timestamp
        FROM request_logs
        WHERE session_id = $1 AND timestamp < $2
        ORDER BY timestamp DESC
        LIMIT $3`
	err := db.Select(&logs, query, sessionID, beforeTime, limit)
	if err != nil {
		return nil, nil, err
	}

	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	attachmentsMap, err := db.getAttachmentsForLogs(logs)
	if err != nil {
		return logs, nil, err
	}

	return logs, attachmentsMap, nil
}

func (db *DB) getAttachmentsForLogs(logs []models.RequestLog) (map[int][]models.FileAttachment, error) {
	if len(logs) == 0 {
		return make(map[int][]models.FileAttachment), nil
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
		q, args, err := sqlx.In("SELECT id, session_id, user_id, file_name, file_uri, mime_type, status, created_at, request_log_id FROM file_attachments WHERE id IN (?)", allFileIDs)
		if err != nil {
			return nil, err
		}
		q = db.Rebind(q)
		err = db.Select(&attachments, q, args...)
		if err != nil {
			return nil, err
		}
		for _, att := range attachments {
			if logID, ok := logIDMap[att.ID]; ok {
				attachmentsMap[logID] = append(attachmentsMap[logID], att)
			}
		}
	}

	return attachmentsMap, nil
}

func (db *DB) GetRequestLogByID(logID int64, userID int) (*models.RequestLog, error) {
	var log models.RequestLog
	query := `
        SELECT rl.* FROM request_logs rl
        JOIN chat_sessions cs ON rl.session_id = cs.id
        WHERE rl.id = $1 AND cs.user_id = $2`
	err := db.Get(&log, query, logID, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &log, err
}

func (db *DB) UpdateRequestLogQuery(logID int64, userID int, newQuery string) error {
	query := `
        UPDATE request_logs SET user_query = $1
        WHERE id = $2 AND session_id IN (SELECT id FROM chat_sessions WHERE user_id = $3)`
	_, err := db.Exec(query, newQuery, logID, userID)
	return err
}

func (db *DB) UpdateRequestLogResponse(logID int64, thoughtsJSON string, finalResponse string) error {
	query := `
        UPDATE request_logs
        SET ego_thoughts_json = $1, final_response = $2, timestamp = $3
        WHERE id = $4`
	_, err := db.Exec(query, thoughtsJSON, finalResponse, time.Now().UTC(), logID)
	return err
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
