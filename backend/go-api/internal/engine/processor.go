package engine

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"egobackend/internal/database"
	"egobackend/internal/models"

	"github.com/google/uuid"
)

type Processor struct {
	DB               *database.DB
	PythonBackendURL string
	httpClient       *http.Client
}

func NewProcessor(db *database.DB, pyURL string) *Processor {
	return &Processor{
		DB:               db,
		PythonBackendURL: pyURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Minute,
		},
	}
}

type EventCallback func(eventType string, data interface{})

func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	return string(([]rune(s))[:maxLen])
}

func (p *Processor) ProcessRequest(req models.StreamRequest, user *models.User, callback EventCallback) {
	log.Printf("[PROCESSOR] Запрос от %s (ID %d) принят. Режим: %s.", user.Username, user.ID, req.Mode)
	callback("status", "Подготовка сессии...")

	session, attachedFileIDs, err := p.prepareSessionAndFiles(req, user, callback)
	if err != nil {
		callback("error", map[string]string{"message": err.Error()})
		return
	}

	callback("status", "Загрузка истории чата...")
	historyLogs, historyAttachments, err := p.DB.GetSessionHistory(session.ID, 10)
	if err != nil {
		callback("error", map[string]string{"message": "Ошибка загрузки истории: " + err.Error()})
		return
	}
	chatHistory := p.buildChatHistory(historyLogs, historyAttachments)

	thoughtsHistory, err := p.runThinkerLoop(req, session, chatHistory, callback)
	if err != nil {
		callback("error", map[string]string{"message": "Ошибка в цикле мышления: " + err.Error()})
		return
	}

	log.Printf("[PROCESSOR] Начало фазы синтеза для сессии %d.", session.ID)
	callback("status", "Синтез финального ответа...")
	thoughtsHistoryJSON, _ := json.Marshal(thoughtsHistory)

	synthesisRequest := models.PythonRequest{
		Query:              req.Query,
		ChatHistory:        chatHistory,
		ThoughtsHistory:    string(thoughtsHistoryJSON),
		Mode:               req.Mode,
		CustomInstructions: session.CustomInstructions,
	}

	finalResponse, err := p.processPythonStream("/synthesize_stream", synthesisRequest, callback)
	if err != nil {
		callback("error", map[string]string{"message": "Ошибка синтеза: " + err.Error()})
		return
	}

	p.saveLog(session.ID, req.Query, thoughtsHistory, finalResponse, attachedFileIDs)
	callback("done", "Процесс завершен")
}

func (p *Processor) runThinkerLoop(req models.StreamRequest, session *models.ChatSession, chatHistory string, callback EventCallback) ([]map[string]interface{}, error) {
	var thoughtsHistory []map[string]interface{}
	maxThoughts := 15

	filesForFirstIteration := req.Files

	for i := 0; i < maxThoughts; i++ {
		log.Printf("[PROCESSOR] Итерация Мышления #%d", i+1)

		pythonRequestData := models.PythonRequest{
			Query:              req.Query,
			Mode:               req.Mode,
			ChatHistory:        chatHistory,
			ThoughtsHistory:    mustMarshal(thoughtsHistory),
			CustomInstructions: session.CustomInstructions,
		}

		thoughtData, err := p.callGenerateThoughtMultipart(pythonRequestData, filesForFirstIteration)
		if err != nil {
			log.Printf("!!! Ошибка генерации мысли на итерации %d: %v", i+1, err)
			thoughtsHistory = append(thoughtsHistory, map[string]interface{}{"type": "system_error", "error": err.Error()})
			continue
		}

		p.processThoughtData(thoughtData, &thoughtsHistory, callback)

		if !thoughtData.Thought.NextThoughtNeeded {
			log.Printf("[PROCESSOR] Мышление завершено по флагу NextThoughtNeeded=false.")
			break
		}
	}
	return thoughtsHistory, nil
}

func (p *Processor) processThoughtData(thoughtData *models.ThoughtResponseWithData, thoughtsHistory *[]map[string]interface{}, callback EventCallback) {
	thought := thoughtData.Thought
	if thoughtData.Usage != nil {
		callback("usage_update", thoughtData.Usage)
	}

	*thoughtsHistory = append(*thoughtsHistory, map[string]interface{}{"type": "thought", "content": thought})
	if thought.ThoughtHeader != "" {
		callback("thought_header", thought.ThoughtHeader)
	}

	if len(thought.ToolCalls) > 0 {
		toolResults := p.executeTools(thought.ToolCalls, callback)
		*thoughtsHistory = append(*thoughtsHistory, toolResults...)
	}
}

func (p *Processor) callGenerateThoughtMultipart(requestData models.PythonRequest, files []models.FilePayload) (*models.ThoughtResponseWithData, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	jsonPart, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга request_data: %w", err)
	}
	if err := writer.WriteField("request_data", string(jsonPart)); err != nil {
		return nil, fmt.Errorf("ошибка записи поля request_data: %w", err)
	}

	if len(files) > 0 {
		for _, file := range files {
			fileBytes, err := base64.StdEncoding.DecodeString(file.Base64Data)
			if err != nil {
				log.Printf("!!! Ошибка декодирования base64 для файла %s: %v", file.FileName, err)
				continue
			}

			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, file.FileName))
			h.Set("Content-Type", file.MimeType)

			part, err := writer.CreatePart(h)
			if err != nil {
				return nil, fmt.Errorf("ошибка создания form-file для %s: %w", file.FileName, err)
			}
			if _, err := part.Write(fileBytes); err != nil {
				return nil, fmt.Errorf("ошибка записи байтов файла %s: %w", file.FileName, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("ошибка закрытия multipart writer: %w", err)
	}

	url := p.PythonBackendURL + "/generate_thought"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания multipart запроса: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	log.Printf("--> [HTTP MULTIPART] Вызов Python. Эндпоинт: /generate_thought")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		log.Printf("!!! [HTTP MULTIPART] КРИТИЧЕСКАЯ ОШИБКА вызова Python: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("<-- [HTTP MULTIPART] Ответ от Python получен. Статус: %d", resp.StatusCode)
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python-сервис вернул ошибку (статус %d): %s", resp.StatusCode, string(responseBody))
	}

	var response models.ThoughtResponseWithData
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга multipart ответа: %w. Ответ: %s", err, string(responseBody))
	}
	return &response, nil
}

func (p *Processor) executeTools(toolCalls []models.ToolCall, callback EventCallback) []map[string]interface{} {
	log.Printf("[PROCESSOR] Обнаружено %d вызовов инструментов. Запускаю...", len(toolCalls))
	var wg sync.WaitGroup
	resultsChan := make(chan map[string]interface{}, len(toolCalls))

	for _, toolCall := range toolCalls {
		wg.Add(1)
		go func(tc models.ToolCall) {
			defer wg.Done()
			callback("tool_call", tc)
			toolResult, err := p.callPythonTool(tc.ToolName, tc.ToolQuery)
			if err != nil {
				log.Printf("!!! Ошибка вызова инструмента '%s': %v", tc.ToolName, err)
				resultsChan <- map[string]interface{}{"type": "tool_error", "tool_name": tc.ToolName, "error": err.Error()}
			} else {
				log.Printf("[PROCESSOR] Инструмент '%s' вернул результат.", tc.ToolName)
				resultsChan <- map[string]interface{}{"type": "tool_output", "tool_name": tc.ToolName, "output": toolResult}
			}
		}(toolCall)
	}

	wg.Wait()
	close(resultsChan)

	var results []map[string]interface{}
	for result := range resultsChan {
		results = append(results, result)
		callback(result["type"].(string), result)
	}
	return results
}

func (p *Processor) prepareSessionAndFiles(req models.StreamRequest, user *models.User, callback EventCallback) (*models.ChatSession, []int64, error) {
	var sessionIDStr string
	if req.SessionID != nil {
		sessionIDStr = fmt.Sprintf("%d", *req.SessionID)
	}

	sessionTitle := truncateString(req.Query, 50)
	if utf8.RuneCountInString(req.Query) > 50 {
		sessionTitle += "..."
	}
	if sessionTitle == "" && len(req.Files) > 0 {
		var names []string
		for _, f := range req.Files {
			names = append(names, f.FileName)
		}
		sessionTitle = strings.Join(names, ", ")
		if utf8.RuneCountInString(sessionTitle) > 50 {
			sessionTitle = truncateString(sessionTitle, 47) + "..."
		}
	}
	if sessionTitle == "" {
		sessionTitle = "Новый чат"
	}

	session, err := p.DB.GetOrCreateSession(sessionIDStr, sessionTitle, user.ID, req.Mode)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка работы с сессией: %w", err)
	}

	if req.SessionID == nil && req.CustomInstructions != nil && *req.CustomInstructions != "" {
		if err := p.DB.UpdateSessionInstructions(session.ID, user.ID, *req.CustomInstructions); err != nil {
			log.Printf("!!! ОШИБКА: Не удалось сохранить инструкции для новой сессии %d: %v", session.ID, err)
		} else {
			session.CustomInstructions = req.CustomInstructions
		}
	}

	if req.SessionID == nil {
		callback("session_created", session)
	}
	var attachedFileIDs []int64
	if len(req.Files) > 0 {
		log.Printf("[PROCESSOR] Получено %d файлов для сессии %d.", len(req.Files), session.ID)
		uploadDir := "./uploads"
		if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
			return nil, nil, fmt.Errorf("ошибка сервера при создании папки для загрузок: %w", err)
		}
		for _, fileData := range req.Files {
			data, err := base64.StdEncoding.DecodeString(fileData.Base64Data)
			if err != nil {
				log.Printf("!!! ОШИБКА: Не удалось декодировать Base64 для файла %s: %v", fileData.FileName, err)
				continue
			}
			uniqueFileName := fmt.Sprintf("%s%s", uuid.New().String(), filepath.Ext(fileData.FileName))
			filePath := filepath.Join(uploadDir, uniqueFileName)
			if err := os.WriteFile(filePath, data, 0644); err != nil {
				log.Printf("!!! ОШИБКА: Не удалось сохранить файл %s на диск: %v", filePath, err)
				continue
			}
			fileID, err := p.DB.SaveFileAttachment(session.ID, user.ID, fileData.FileName, uniqueFileName, fileData.MimeType, "uploaded")
			if err != nil {
				log.Printf("!!! Ошибка сохранения метаданных файла в БД: %v", err)
				os.Remove(filePath)
				continue
			}
			attachedFileIDs = append(attachedFileIDs, fileID)
		}
	}
	return session, attachedFileIDs, nil
}

func (p *Processor) buildChatHistory(logs []models.RequestLog, attachments map[int][]models.FileAttachment) string {
	var chatHistoryBuilder strings.Builder
	for _, logEntry := range logs {
		finalResponseText := ""
		if logEntry.FinalResponse != nil {
			finalResponseText = *logEntry.FinalResponse
		}
		attachmentsText := ""
		if attachments, ok := attachments[logEntry.ID]; ok && len(attachments) > 0 {
			var names []string
			for _, a := range attachments {
				names = append(names, a.FileName)
			}
			attachmentsText = fmt.Sprintf(" [Attached Files: %s]", strings.Join(names, ", "))
		}
		chatHistoryBuilder.WriteString(fmt.Sprintf("User:%s %s\nEGO: %s\n\n", attachmentsText, logEntry.UserQuery, finalResponseText))
	}
	return chatHistoryBuilder.String()
}

func (p *Processor) saveLog(sessionID int, query string, thoughtsHistory []map[string]interface{}, finalResponse string, attachedFileIDs []int64) {
	thoughtsHistoryJSON, _ := json.Marshal(thoughtsHistory)
	attachedFileIDsJSON, _ := json.Marshal(attachedFileIDs)

	logEntry := &models.RequestLog{
		SessionID:       sessionID,
		UserQuery:       query,
		EgoThoughtsJSON: string(thoughtsHistoryJSON),
		FinalResponse:   &finalResponse,
		Timestamp:       time.Now().UTC(),
		AttachedFileIDs: string(attachedFileIDsJSON),
	}

	logID, err := p.DB.SaveRequestLog(logEntry)
	if err != nil {
		log.Printf("!!! [PROCESSOR] КРИТИЧЕСКАЯ ОШИБКА: Не удалось сохранить лог в БД: %v", err)
	} else {
		if err := p.DB.AssociateFilesWithRequestLog(logID, attachedFileIDs); err != nil {
			log.Printf("!!! [PROCESSOR] ОШИБКА: Не удалось связать файлы с логом %d: %v", logID, err)
		}
	}
}

func (p *Processor) callPythonTool(toolName, toolQuery string) (string, error) {
	toolRequestBody := map[string]string{"query": toolQuery}
	toolResultBody, err := p.callPythonService(fmt.Sprintf("/execute_tool/%s", toolName), toolRequestBody)
	if err != nil {
		return "", err
	}
	var toolResult map[string]string
	if err := json.Unmarshal(toolResultBody, &toolResult); err != nil {
		return "", fmt.Errorf("ошибка парсинга результата инструмента: %w", err)
	}
	if result, ok := toolResult["result"]; ok {
		return result, nil
	}
	return "", fmt.Errorf("ключ 'result' не найден в ответе инструмента")
}

func (p *Processor) processPythonStream(endpoint string, requestBody interface{}, callback EventCallback) (string, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("ошибка маршалинга тела запроса: %w", err)
	}
	url := p.PythonBackendURL + endpoint
	resp, err := p.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка HTTP POST запроса к Python: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка стрима от Python (статус %d): %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
			return i + 2, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	var fullResponseBuilder strings.Builder
	for scanner.Scan() {
		eventBlock := scanner.Bytes()
		if !bytes.HasPrefix(eventBlock, []byte("data: ")) {
			continue
		}
		jsonPayload := bytes.TrimPrefix(eventBlock, []byte("data: "))
		if len(jsonPayload) == 0 {
			continue
		}
		var rawEvent map[string]interface{}
		if err := json.Unmarshal(jsonPayload, &rawEvent); err == nil {
			if eventType, ok := rawEvent["type"].(string); ok {
				if data, ok := rawEvent["data"]; ok {
					callback(eventType, data)
					if eventType == "chunk" {
						if dataMap, ok := data.(map[string]interface{}); ok {
							if text, ok := dataMap["text"].(string); ok {
								fullResponseBuilder.WriteString(text)
							}
						}
					}
				}
			}
		} else {
			log.Printf("!!! ОШИБКА ПАРСИНГА JSON из стрима: %v. Payload: %s", err, string(jsonPayload))
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("ошибка чтения потока от Python: %w", err)
	}
	log.Println("[STREAM] Конец потока от Python.")
	return fullResponseBuilder.String(), nil
}

func (p *Processor) callPythonService(endpoint string, requestBody interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	url := p.PythonBackendURL + endpoint
	log.Printf("--> [HTTP JSON] Вызов Python. Эндпоинт: %s. Размер тела запроса: %.2f KB", endpoint, float64(len(jsonData))/1024.0)

	resp, err := p.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("!!! [HTTP JSON] КРИТИЧЕСКАЯ ОШИБКА вызова Python: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("<-- [HTTP JSON] Ответ от Python получен. Статус: %d", resp.StatusCode)
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python-сервис вернул ошибку (статус %d): %s", resp.StatusCode, string(responseBody))
	}
	return responseBody, nil
}

func mustMarshal(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		log.Printf("!!! CRITICAL: Failed to marshal object for history: %v", err)
		return "[]"
	}
	return string(bytes)
}
