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

// ─── Email Verification ──────────────────────────────────────────────────────

var VerifySecret = []byte(getEnv("VERIFY_SECRET", "verify_secret_key"))

// GenerateVerificationToken creates a short-lived JWT that references the
// pending_registrations document by its ObjectID.
// No user data (especially no password) is embedded in the token.
func GenerateVerificationToken(pendingID string) (string, error) {
	claims := jwt.MapClaims{
		"pending_id": pendingID,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(VerifySecret)
}

// ParseVerificationToken validates the token and returns the pending_id.
func ParseVerificationToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return VerifySecret, nil
	})
	if err != nil || !token.Valid {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrInvalidKey
	}
	pendingID, _ := claims["pending_id"].(string)
	return pendingID, nil
}