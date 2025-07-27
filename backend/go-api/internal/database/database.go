package database

import (
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DB struct {
	*sqlx.DB
}

func New() (*DB, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("Критическая ошибка: переменная окружения DATABASE_URL не установлена")
	}

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Успешное подключение к БД PostgreSQL")
	return &DB{db}, nil
}

func (db *DB) Migrate() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			hashed_password TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			mode TEXT NOT NULL,
			custom_instructions TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS request_logs (
			id SERIAL PRIMARY KEY,
			session_id INTEGER NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
			user_query TEXT NOT NULL,
			ego_thoughts_json TEXT,
			final_response TEXT,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			attached_file_ids TEXT
		);`,

		`CREATE TABLE IF NOT EXISTS file_attachments (
			id SERIAL PRIMARY KEY,
			session_id INTEGER NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			request_log_id INTEGER REFERENCES request_logs(id) ON DELETE SET NULL,
			file_name TEXT NOT NULL,
			file_uri TEXT NOT NULL UNIQUE, -- Уникальное имя файла на диске
			mime_type TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
	}

	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			log.Printf("Предупреждение при выполнении миграции: %v. Возможно, таблица уже существует в правильном формате.", err)
		}
	}

	log.Println("Миграция всех таблиц завершена")
	return nil
}
