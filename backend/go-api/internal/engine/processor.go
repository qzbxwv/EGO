package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"egobackend/internal/database"
	"egobackend/internal/models"
	"egobackend/internal/storage"

	"github.com/google/uuid"
)

type Processor struct {
	DB               *database.DB
	PythonBackendURL string
	S3Service        *storage.S3Service
	httpClient       *http.Client
}

func NewProcessor(db *database.DB, pyURL string, s3 *storage.S3Service) *Processor {
	return &Processor{
		DB:               db,
		PythonBackendURL: pyURL,
		S3Service:        s3,
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

func (p *Processor) ProcessRequest(req models.StreamRequest, user *models.User, tempID int64, callback EventCallback) {
	var session *models.ChatSession
	var userQuery string
	var filesForRequest []models.FilePayload
	var historyLogs []models.RequestLog
	var historyAttachments map[int][]models.FileAttachment
	var err error
	var newAttachedFileIDs []int64

	if req.IsRegeneration {
		log.Printf("[PROCESSOR] Запуск регенерации для лога ID %d", req.RequestLogIDToRegen)
		logToRegen, errGetLog := p.DB.GetRequestLogByID(req.RequestLogIDToRegen, user.ID)
		if errGetLog != nil || logToRegen == nil {
			callback("error", map[string]string{"message": "Ошибка: лог для регенерации не найден или нет доступа."})
			return
		}
		session, err = p.DB.GetSessionByID(logToRegen.SessionID, user.ID)
		if err != nil || session == nil {
			callback("error", map[string]string{"message": "Ошибка получения сессии для регенерации"})
			return
		}
		userQuery = logToRegen.UserQuery
		historyLogs, historyAttachments, err = p.DB.GetSessionHistoryBefore(session.ID, logToRegen.Timestamp, 10)
		if err != nil {
			callback("error", map[string]string{"message": "Ошибка загрузки чистой истории: " + err.Error()})
			return
		}
		var originalFileIDs []int
		if err := json.Unmarshal([]byte(logToRegen.AttachedFileIDs), &originalFileIDs); err == nil && len(originalFileIDs) > 0 {
			attachments, err := p.DB.GetAttachmentsByIDs(originalFileIDs)
			if err != nil {
				log.Printf("!!! Ошибка получения файлов для регенерации: %v", err)
			} else {
				for _, att := range attachments {
					fileBytes, err := p.S3Service.DownloadFile(context.Background(), att.FileURI)
					if err != nil {
						continue
					}
					encodedData := base64.StdEncoding.EncodeToString(fileBytes)
					filesForRequest = append(filesForRequest, models.FilePayload{
						Base64Data: encodedData, MimeType: att.MimeType, FileName: att.FileName,
					})
				}
			}
		}
	} else {
		log.Printf("[PROCESSOR] Запрос от %s (ID %d) принят. Режим: %s.", user.Username, user.ID, req.Mode)

		var wasCreated bool
		session, wasCreated, err = p.getOrCreateSessionFromRequest(req, user)
		if err != nil {
			callback("error", map[string]string{"message": err.Error()})
			return
		}

		if wasCreated {
			callback("session_created", session)
		}

		newAttachedFileIDs, err = p.saveAttachmentsFromRequest(req, user, session.ID)
		if err != nil {
			log.Printf("!!! ОШИБКА при сохранении файлов: %v", err)
		}

		userQuery = req.Query
		filesForRequest = req.Files
		historyLogs, historyAttachments, err = p.DB.GetSessionHistory(session.ID, 10)
		if err != nil {
			callback("error", map[string]string{"message": "Ошибка загрузки истории: " + err.Error()})
			return
		}
	}

	chatHistory := p.buildChatHistory(historyLogs, historyAttachments)
	allFilesPayload := filesForRequest
	processedFileNames := make(map[string]bool)
	for _, f := range allFilesPayload {
		processedFileNames[f.FileName] = true
	}
	for _, attachmentsInLog := range historyAttachments {
		for _, att := range attachmentsInLog {
			if _, exists := processedFileNames[att.FileName]; exists {
				continue
			}
			fileBytes, err := p.S3Service.DownloadFile(context.Background(), att.FileURI)
			if err != nil {
				log.Printf("!!! ОШИБКА: Не удалось загрузить исторический файл %s из S3: %v", att.FileURI, err)
				continue
			}
			encodedData := base64.StdEncoding.EncodeToString(fileBytes)
			allFilesPayload = append(allFilesPayload, models.FilePayload{
				Base64Data: encodedData, MimeType: att.MimeType, FileName: att.FileName,
			})
			processedFileNames[att.FileName] = true
		}
	}
	log.Printf("[PROCESSOR] Всего будет отправлено в Python %d файлов.", len(allFilesPayload))

	thoughtsHistory, err := p.runThinkerLoop(userQuery, req.Mode, session.CustomInstructions, chatHistory, allFilesPayload, callback)
	if err != nil {
		callback("error", map[string]string{"message": "Ошибка в цикле мышления: " + err.Error()})
		return
	}

	thoughtsHistoryJSON, _ := json.Marshal(thoughtsHistory)
	synthesisRequest := models.PythonRequest{
		Query: userQuery, ChatHistory: chatHistory, ThoughtsHistory: string(thoughtsHistoryJSON), Mode: req.Mode, CustomInstructions: session.CustomInstructions,
	}
	finalResponse, err := p.processPythonMultipartStream("/synthesize_stream", synthesisRequest, allFilesPayload, callback)
	if err != nil {
		callback("error", map[string]string{"message": "Ошибка синтеза: " + err.Error()})
		return
	}

	if req.IsRegeneration {
		err = p.DB.UpdateRequestLogResponse(req.RequestLogIDToRegen, string(thoughtsHistoryJSON), finalResponse)
		if err != nil {
			log.Printf("!!! ОШИБКА: Не удалось обновить лог %d: %v", req.RequestLogIDToRegen, err)
		} else {
			log.Printf("[PROCESSOR] Лог %d успешно обновлен после регенерации.", req.RequestLogIDToRegen)
		}
	} else {
		attachedFileIDsJSON, _ := json.Marshal(newAttachedFileIDs)
		logEntry := &models.RequestLog{
			SessionID: session.ID, UserQuery: userQuery, EgoThoughtsJSON: string(thoughtsHistoryJSON), FinalResponse: &finalResponse, Timestamp: time.Now().UTC(), AttachedFileIDs: string(attachedFileIDsJSON),
		}
		logID, err := p.DB.SaveRequestLog(logEntry)
		if err != nil {
			log.Printf("!!! [PROCESSOR] КРИТИЧЕСКАЯ ОШИБКА: Не удалось сохранить лог в БД: %v", err)
		} else {
			if err := p.DB.AssociateFilesWithRequestLog(logID, newAttachedFileIDs); err != nil {
				log.Printf("!!! [PROCESSOR] ОШИБКА: Не удалось связать файлы с логом %d: %v", logID, err)
			}
			callback("log_saved", map[string]int64{"temp_id": tempID, "db_id": logID})
		}
	}
	callback("done", "Процесс завершен")
}

func (p *Processor) getOrCreateSessionFromRequest(req models.StreamRequest, user *models.User) (*models.ChatSession, bool, error) {
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

	session, wasCreated, err := p.DB.GetOrCreateSession(sessionIDStr, sessionTitle, user.ID, req.Mode)
	if err != nil {
		return nil, false, fmt.Errorf("ошибка работы с сессией: %w", err)
	}

	if wasCreated && req.CustomInstructions != nil && *req.CustomInstructions != "" {
		if err := p.DB.UpdateSessionInstructions(session.ID, user.ID, *req.CustomInstructions); err != nil {
			log.Printf("!!! ОШИБКА: Не удалось сохранить инструкции для новой сессии %d: %v", session.ID, err)
		} else {
			session.CustomInstructions = req.CustomInstructions
		}
	}

	return session, wasCreated, nil
}

func (p *Processor) saveAttachmentsFromRequest(req models.StreamRequest, user *models.User, sessionID int) ([]int64, error) {
	var attachedFileIDs []int64
	if len(req.Files) > 0 {
		log.Printf("[PROCESSOR] Получено %d файлов для загрузки в S3 для сессии %d.", len(req.Files), sessionID)
		for _, fileData := range req.Files {
			data, err := base64.StdEncoding.DecodeString(fileData.Base64Data)
			if err != nil {
				log.Printf("!!! ОШИБКА: Не удалось декодировать Base64 для файла %s: %v", fileData.FileName, err)
				continue
			}
			s3Key := fmt.Sprintf("%s%s", uuid.New().String(), filepath.Ext(fileData.FileName))
			err = p.S3Service.UploadFile(context.Background(), s3Key, fileData.MimeType, data)
			if err != nil {
				log.Printf("!!! ОШИБКА: Не удалось загрузить файл %s в S3: %v", fileData.FileName, err)
				continue
			}
			fileID, err := p.DB.SaveFileAttachment(sessionID, user.ID, fileData.FileName, s3Key, fileData.MimeType, "uploaded")
			if err != nil {
				log.Printf("!!! Ошибка сохранения метаданных файла в БД: %v. Удаляю объект из S3...", err)
				_ = p.S3Service.DeleteFiles(context.Background(), []string{s3Key})
				continue
			}
			attachedFileIDs = append(attachedFileIDs, fileID)
		}
	}
	return attachedFileIDs, nil
}

func (p *Processor) runThinkerLoop(query, mode string, customInstructions *string, chatHistory string, allFilesPayload []models.FilePayload, callback EventCallback) ([]map[string]interface{}, error) {
	var thoughtsHistory []map[string]interface{}
	maxThoughts := 15
	for i := 0; i < maxThoughts; i++ {
		pythonRequestData := models.PythonRequest{
			Query: query, Mode: mode, ChatHistory: chatHistory, ThoughtsHistory: mustMarshal(thoughtsHistory), CustomInstructions: customInstructions,
		}
		thoughtData, err := p.callGenerateThoughtMultipart(pythonRequestData, allFilesPayload)
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
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("ошибка закрытия multipart writer: %w", err)
	}
	url := p.PythonBackendURL + "/generate_thought"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания multipart запроса: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	log.Printf("--> [HTTP MULTIPART] Вызов Python. Эндпоинт: /generate_thought. Количество файлов: %d", len(files))
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

func (p *Processor) processPythonMultipartStream(endpoint string, requestData models.PythonRequest, files []models.FilePayload, callback EventCallback) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	jsonPart, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("ошибка маршалинга request_data: %w", err)
	}
	if err := writer.WriteField("request_data", string(jsonPart)); err != nil {
		return "", fmt.Errorf("ошибка записи поля request_data: %w", err)
	}
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
			return "", fmt.Errorf("ошибка создания form-file для %s: %w", file.FileName, err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			return "", fmt.Errorf("ошибка записи байтов файла %s: %w", file.FileName, err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("ошибка закрытия multipart writer: %w", err)
	}
	url := p.PythonBackendURL + endpoint
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("ошибка создания multipart запроса для стрима: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	log.Printf("--> [HTTP MULTIPART STREAM] Вызов Python. Эндпоинт: %s. Количество файлов: %d", endpoint, len(files))
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка HTTP POST запроса к Python (стрим): %w", err)
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
			attachmentsText = fmt.Sprintf(" [Attached: %s]", strings.Join(names, ", "))
		}
		chatHistoryBuilder.WriteString(fmt.Sprintf("User: %s%s\nEGO: %s\n\n", logEntry.UserQuery, attachmentsText, finalResponseText))
	}
	return chatHistoryBuilder.String()
}
