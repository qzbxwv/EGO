package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"egobackend/internal/auth"
	"egobackend/internal/database"
	"egobackend/internal/models"
)

type ContextKey string

const UserContextKey = ContextKey("user")

type AuthHandler struct {
	DB          *database.DB
	AuthService *auth.AuthService
}

func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		if strings.Contains(r.URL.Path, "/ws") {
			tokenString = r.URL.Query().Get("token")
			if tokenString == "" {
				RespondWithError(w, http.StatusUnauthorized, "Требуется токен в параметрах URL для WebSocket")
				return
			}
		} else {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				RespondWithError(w, http.StatusUnauthorized, "Требуется заголовок Authorization")
				return
			}
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if tokenString == "" {
			RespondWithError(w, http.StatusUnauthorized, "Токен авторизации отсутствует")
			return
		}

		username, err := h.AuthService.ValidateJWT(tokenString)
		if err != nil {
			log.Printf("Ошибка валидации токена для %s: %v", r.URL.Path, err)
			RespondWithError(w, http.StatusUnauthorized, "Невалидный или просроченный токен")
			return
		}

		user, err := h.DB.GetUserByUsername(username)
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, "Пользователь из токена не найден")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}
	if req.Username == "" || req.Password == "" {
		RespondWithError(w, http.StatusBadRequest, "Имя пользователя и пароль не могут быть пустыми")
		return
	}
	user, err := h.DB.GetUserByUsername(req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusUnauthorized, "Неверный логин или пароль")
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера")
		return
	}
	if !auth.CheckPasswordHash(req.Password, user.HashedPassword) {
		RespondWithError(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}
	accessToken, err := h.AuthService.CreateAccessToken(user.Username, user.Role)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Не удалось создать токен доступа")
		return
	}
	refreshToken, err := h.AuthService.CreateRefreshToken(user.Username)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Не удалось создать токен обновления")
		return
	}

	response := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          models.UserResponse{ID: user.ID, Username: user.Username, Role: user.Role, CreatedAt: user.CreatedAt},
	}

	RespondWithJSON(w, http.StatusOK, response)
	log.Printf("Пользователь '%s' успешно вошел в систему.", user.Username)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}
	if req.Username == "" || req.Password == "" {
		RespondWithError(w, http.StatusBadRequest, "Имя пользователя и пароль не могут быть пустыми")
		return
	}
	_, err := h.DB.GetUserByUsername(req.Username)
	if err == nil {
		RespondWithError(w, http.StatusConflict, "Пользователь с таким именем уже существует")
		return
	}
	if err != sql.ErrNoRows {
		RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при проверке пользователя")
		return
	}
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при хешировании пароля")
		return
	}
	newUser, err := h.DB.CreateUser(req.Username, hashedPassword)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Не удалось создать пользователя")
		return
	}
	log.Printf("Зарегистрирован новый пользователь: %s (ID: %d)", newUser.Username, newUser.ID)
	response := models.UserResponse{ID: newUser.ID, Username: newUser.Username, Role: newUser.Role, CreatedAt: newUser.CreatedAt}
	RespondWithJSON(w, http.StatusCreated, response)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	username, err := h.AuthService.ValidateJWT(req.RefreshToken)
	if err != nil {
		log.Printf("Попытка обновления с невалидным refresh-токеном: %v", err)
		RespondWithError(w, http.StatusUnauthorized, "Невалидный refresh-токен")
		return
	}

	user, err := h.DB.GetUserByUsername(username)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Пользователь из токена не найден")
		return
	}

	newAccessToken, err := h.AuthService.CreateAccessToken(user.Username, user.Role)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Не удалось создать новый access-токен")
		return
	}

	response := models.RefreshResponse{
		AccessToken: newAccessToken,
	}

	RespondWithJSON(w, http.StatusOK, response)
	log.Printf("Токен для пользователя '%s' был успешно обновлен.", user.Username)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		RespondWithError(w, http.StatusInternalServerError, "Не удалось получить пользователя из контекста")
		return
	}
	response := models.UserResponse{ID: user.ID, Username: user.Username, Role: user.Role, CreatedAt: user.CreatedAt}
	RespondWithJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.GoogleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	email, err := h.AuthService.ValidateGoogleJWT(req.Token, googleClientID)
	if err != nil {
		log.Printf("Ошибка верификации Google токена %v", err)
		RespondWithError(w, http.StatusUnauthorized, "Невалидный токен Google")
		return
	}
	user, err := h.DB.GetUserByUsername(email)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Создание нового пользователя через Google Auth: %s", email)
			randPass := "-veryhard__PASSFORemAil" + email
			hashPass, _ := auth.HashPassword(randPass)
			newUser, createErr := h.DB.CreateUser(email, hashPass)
			if createErr != nil {
				RespondWithError(w, http.StatusInternalServerError, "Не удалось создать пользователя")
				return
			}
			user = newUser
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера про поиске пользователя")
			return
		}
	}
	accessToken, err := h.AuthService.CreateAccessToken(user.Username, user.Role)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Не удалось создать accessJWT токен")
		return
	}
	refreshToken, err := h.AuthService.CreateRefreshToken(user.Username)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Не удалось создать refreshJWT токен")
		return
	}
	response := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          models.UserResponse{ID: user.ID, Username: user.Username, Role: user.Role, CreatedAt: user.CreatedAt},
	}
	RespondWithJSON(w, http.StatusOK, response)
	log.Printf("Пользователь '%s' успешно вошел через Google.", user.Username)
}
