package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID             int       `db:"id" json:"id"`
	Username       string    `db:"username" json:"username"`
	HashedPassword string    `db:"hashed_password" json:"-"`
	Role           string    `db:"role" json:"role"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

type ChatSession struct {
	ID                 int       `db:"id" json:"id"`
	UserID             int       `db:"user_id" json:"-"`
	Title              string    `db:"title" json:"title"`
	Mode               string    `db:"mode" json:"mode"`
	CustomInstructions *string   `db:"custom_instructions" json:"custom_instructions,omitempty"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
}

type RequestLog struct {
	ID               int       `db:"id"`
	SessionID        int       `db:"session_id"`
	UserQuery        string    `db:"user_query"`
	EgoThoughtsJSON  string    `db:"ego_thoughts_json"`
	FinalResponse    *string   `db:"final_response"`
	PromptTokens     int       `db:"prompt_tokens"`
	CompletionTokens int       `db:"completion_tokens"`
	TotalTokens      int       `db:"total_tokens"`
	AttachedFileIDs  string    `db:"attached_file_ids"`
	Timestamp        time.Time `db:"timestamp"`
}

type FileAttachment struct {
	ID           int64         `db:"id"`
	SessionID    int           `db:"session_id"`
	UserID       int           `db:"user_id"`
	RequestLogID sql.NullInt64 `db:"request_log_id"`
	FileName     string        `db:"file_name"`
	FileURI      string        `db:"file_uri"`
	MimeType     string        `db:"mime_type"`
	Status       string        `db:"status"`
	CreatedAt    time.Time     `db:"created_at"`
}

type FilePayload struct {
	Base64Data string `json:"base64_data"`
	MimeType   string `json:"mime_type"`
	FileName   string `json:"file_name"`
}

type StreamRequest struct {
	Query              string        `json:"query"`
	Mode               string        `json:"mode"`
	SessionID          *int          `json:"session_id,omitempty"`
	Files              []FilePayload `json:"files,omitempty"`
	CustomInstructions *string       `json:"custom_instructions,omitempty"`
}

type ToolCall struct {
	ToolName  string `json:"tool_name"`
	ToolQuery string `json:"tool_query"`
}

type ThoughtResponse struct {
	Thoughts          string     `json:"thoughts"`
	Evaluate          string     `json:"evaluate"`
	Confidence        float64    `json:"confidence"`
	ToolReasoning     string     `json:"tool_reasoning"`
	ToolCalls         []ToolCall `json:"tool_calls"`
	ThoughtHeader     string     `json:"thoughts_header"`
	NextThoughtNeeded bool       `json:"nextThoughtNeeded"`
}

type ThoughtResponseWithData struct {
	Thought          ThoughtResponse        `json:"thought"`
	Usage            map[string]interface{} `json:"usage"`
	UploadedFileURIs []string               `json:"uploaded_file_uris"`
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken string `json:"access_token"`
}

type UserResponse struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type SessionResponse struct {
	ID                 int       `json:"id"`
	Title              string    `json:"title"`
	Mode               string    `json:"mode"`
	CustomInstructions *string   `json:"custom_instructions,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type FileAttachmentResponse struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
}

type LogResponse struct {
	ID            int                      `json:"id"`
	UserQuery     string                   `json:"user_query"`
	FinalResponse *string                  `json:"final_response"`
	Timestamp     time.Time                `json:"timestamp"`
	Attachments   []FileAttachmentResponse `json:"attachments"`
}

type GoogleAuthRequest struct {
	Token string `json:"token"`
}

type PythonRequest struct {
	Query              string        `json:"query"`
	Mode               string        `json:"mode"`
	ChatHistory        string        `json:"chat_history"`
	ThoughtsHistory    string        `json:"thoughts_history,omitempty"`
	CustomInstructions *string       `json:"custom_instructions,omitempty"`
	Files              []FilePayload `json:"files,omitempty"`
	CachedFiles        []CachedFile  `json:"cached_files,omitempty"`
}

type CachedFile struct {
	URI      string `json:"uri"`
	MimeType string `json:"mime_type"`
}
