package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// var secretKey = []byte(getSecret())

// func getSecret() string {
// 	secret := os.Getenv("JWT_SECRET")
// 	if secret == "" {
// 		return "default_secret_key" // fallback (not for production)
// 	}
// 	return secret
// }

// type Claims struct {
// 	UserID string `json:"user_id"`
// 	Email  string `json:"email"`
// 	jwt.RegisteredClaims
// }

// func GenerateJWT(userID string, email string) (string, error) {
// 	claims := Claims{
// 		UserID: userID,
// 		Email:  email,
// 		RegisteredClaims: jwt.RegisteredClaims{
// 			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
// 			IssuedAt:  jwt.NewNumericDate(time.Now()),
// 		},
// 	}

// 	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
// 	return token.SignedString(secretKey)
// }
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