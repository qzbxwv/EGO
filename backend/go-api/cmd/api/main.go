package main

import (
	"egobackend/internal/auth"
	"egobackend/internal/database"
	"egobackend/internal/handlers"
	"egobackend/internal/models"
	"egobackend/internal/websocket"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

func CoopMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		next.ServeHTTP(w, r)
	})
}

func startFileCleanupRoutine(db *database.DB) {
	log.Println("[CLEANUP] Запуск фонового процесса очистки старых файлов...")
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	uploadDir := "./uploads"

	for range ticker.C {
		log.Println("[CLEANUP] Выполняется плановая очистка старых файлов (старше 24 часов)...")

		deletedURIs, err := db.DeleteOldFileAttachments(24 * time.Hour)
		if err != nil {
			log.Printf("!!! [CLEANUP] ОШИБКА во время удаления записей из БД: %v", err)
			continue
		}

		if len(deletedURIs) == 0 {
			log.Println("[CLEANUP] Старых файлов для удаления не найдено.")
			continue
		}

		log.Printf("[CLEANUP] Найдено %d записей в БД для удаления. Начинаю удаление физических файлов...", len(deletedURIs))

		deletedCount := 0
		for _, uri := range deletedURIs {
			filePath := filepath.Join(uploadDir, uri)
			err := os.Remove(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					log.Printf("... [CLEANUP] ПРЕДУПРЕЖДЕНИЕ: Файл %s для удаления не найден на диске.", filePath)
				} else {
					log.Printf("!!! [CLEANUP] ОШИБКА при удалении файла %s: %v", filePath, err)
				}
			} else {
				log.Printf("... [CLEANUP] Файл %s успешно удален.", filePath)
				deletedCount++
			}
		}
		log.Printf("[CLEANUP] Очистка завершена. Успешно удалено %d физических файлов.", deletedCount)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Внимание: не удалось загрузить .env файл.")
	}
	dbPath := os.Getenv("DATABASE_URL")
	serverAddr := os.Getenv("SERVER_ADDRESS")
	jwtSecret := os.Getenv("SECRET_KEY")
	pythonBackendURL := os.Getenv("PYTHON_BACKEND_URL")
	if dbPath == "" || serverAddr == "" || jwtSecret == "" || pythonBackendURL == "" {
		log.Fatal("Критическая ошибка: одна или несколько переменных окружения не установлены")
	}

	db, err := database.New()
	if err != nil {
		log.Fatalf("Критическая ошибка! Не удалось подключиться к БД: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		log.Fatalf("Критическая ошибка! Не удалось выполнить миграцию БД: %v", err)
	}

	go startFileCleanupRoutine(db)

	authSvc, err := auth.NewAuthService(jwtSecret)
	if err != nil {
		log.Fatalf("Критическая ошибка: не удалось создать сервис аутентификации: %v", err)
	}

	hub := websocket.NewHub()
	go hub.Run()

	authHandler := &handlers.AuthHandler{DB: db, AuthService: authSvc}
	sessionHandler := &handlers.SessionHandler{DB: db}

	r := chi.NewRouter()
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:4173"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		Debug:            false,
	})
	r.Use(corsMiddleware.Handler)

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(CoopMiddleware)

	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)
	r.Post("/auth/google", authHandler.GoogleLogin)
	r.Post("/auth/refresh", authHandler.Refresh)

	r.Group(func(r chi.Router) {
		r.Use(authHandler.AuthMiddleware)

		r.Get("/me", authHandler.Me)

		r.Get("/sessions", sessionHandler.GetSessions)
		r.Get("/sessions/{sessionID}", sessionHandler.GetSession)
		r.Get("/sessions/{sessionID}/history", sessionHandler.GetHistory)
		r.Delete("/sessions/{sessionID}", sessionHandler.DeleteSession)
		r.Patch("/sessions/{sessionID}", sessionHandler.UpdateSession)

		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(handlers.UserContextKey).(*models.User)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			websocket.ServeWs(hub, w, r, user, db, pythonBackendURL)
		})
	})

	log.Printf("Сервер готов к обслуживанию и слушает на %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, r); err != nil {
		log.Fatalf("Сервер упал с ошибкой: %v", err)
	}
}
