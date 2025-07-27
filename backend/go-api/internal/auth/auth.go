package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

type AuthService struct {
	jwtSecret []byte
}

func NewAuthService(secret string) (*AuthService, error) {
	if secret == "" {
		return nil, errors.New("JWT секрет не может быть пустым")
	}
	return &AuthService{jwtSecret: []byte(secret)}, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		log.Printf("Что-то пошло не так при хешировании пароля: %v", err)
		return "", err
	}
	return string(bytes), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *AuthService) CreateAccessToken(username, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  username,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour * 24).Unix(), // Живет 24 часа
		"role": role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) CreateRefreshToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(), // Живет 30 дней
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateJWT(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неожиданный алгоритм подписи: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if username, ok := claims["sub"].(string); ok {
			return username, nil
		}
	}

	return "", errors.New("невалидный токен")
}

func (s *AuthService) ValidateGoogleJWT(googletoken, audience string) (string, error) {
	payload, err := idtoken.Validate(context.Background(), googletoken, audience)
	if err != nil {
		return "", err
	}

	email, ok := payload.Claims["email"].(string)
	if !ok {
		return "", errors.New("в токене Google отсутствует email")
	}

	return email, nil
}
