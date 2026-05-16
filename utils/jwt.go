package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var AccessSecret = []byte(getEnv("ACCESS_SECRET", "access_secret_key"))
var RefreshSecret = []byte(getEnv("REFRESH_SECRET", "refresh_secret_key"))

// Helper to read env with fallback
func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
func GenerateAccessToken(userID string, email string) (string, error) {

	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}
                  
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(AccessSecret)
}
func GenerateRefreshToken(userID string) (string, error) {

	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(RefreshSecret)
}