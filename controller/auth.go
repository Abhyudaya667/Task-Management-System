package controllers

import (
	"context"
	"net/http"
	"os"
	"time"

	"task-management-system/config"
	"task-management-system/models"
	"task-management-system/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ─── Register ────────────────────────────────────────────────────────────────
//
// Flow:
//  1. Validate input.
//  2. If email/username already exists in users (verified) → 409 Conflict.
//  3. Hash the password.
//  4. Insert into pending_registrations (with 24 h TTL). Multiple attempts for
//     the same email are allowed; only the one whose link gets clicked will win.
//  5. Send verification email whose link carries only the pendingID JWT.
//  6. Return 201 — user never touches the users collection yet.
func Register(c *gin.Context) {
	var req struct {
		UserName string `json:"username" binding:"required"`
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	usersCol := config.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Block only already-verified users.
	var existingUser models.User
	err := usersCol.FindOne(ctx, bson.M{
		"$or": []bson.M{
			{"email": req.Email},
			{"username": req.UserName},
		},
	}).Decode(&existingUser)

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email or username is already in use."})
		return
	}
	if err != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Hash the password — it lives in MongoDB, never in the JWT.
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Save to pending_registrations.
	pending := models.PendingRegistration{
		UserName:       req.UserName,
		Email:          req.Email,
		HashedPassword: hashedPassword,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	res, err := config.DB.Collection("pending_registrations").InsertOne(ctx, pending)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate registration"})
		return
	}
	pendingID := res.InsertedID.(primitive.ObjectID).Hex()

	// Generate token — contains only the pendingID.
	token, err := utils.GenerateVerificationToken(pendingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}

	// Send email.
	if err := utils.SendVerificationEmail(req.Email, req.UserName, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration initiated. Please check your email to complete verification.",
	})
}

// ─── VerifyEmail ─────────────────────────────────────────────────────────────
//
// Flow:
//  1. Parse JWT → pendingID.
//  2. Fetch the pending_registrations document.
//  3. Check users collection for conflicts (race condition protection).
//  4. Create the verified user.
//  5. Delete the pending record (cleanup).
//  6. Redirect to FRONTEND_URL/login?verified=true.
func VerifyEmail(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	pendingIDStr, err := utils.ParseVerificationToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired verification link"})
		return
	}

	pendingOID, err := primitive.ObjectIDFromHex(pendingIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid verification link"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pendingCol := config.DB.Collection("pending_registrations")
	var pending models.PendingRegistration
	if err := pendingCol.FindOne(ctx, bson.M{"_id": pendingOID}).Decode(&pending); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusGone, gin.H{"error": "Verification link has expired or was already used"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Race-condition guard: ensure email/username weren't registered concurrently.
	usersCol := config.DB.Collection("users")
	var existingUser models.User
	if err := usersCol.FindOne(ctx, bson.M{
		"$or": []bson.M{
			{"email": pending.Email},
			{"username": pending.UserName},
		},
	}).Decode(&existingUser); err == nil {
		// Someone beat us to it — clean up the stale pending record.
		pendingCol.DeleteOne(ctx, bson.M{"_id": pendingOID})
		c.JSON(http.StatusConflict, gin.H{"error": "Email or username was already registered by someone else"})
		return
	}

	// Insert verified user.
	newUser := models.User{
		UserName:  pending.UserName,
		Email:     pending.Email,
		Password:  pending.HashedPassword,
		CreatedAt: time.Now(),
	}
	if _, err := usersCol.InsertOne(ctx, newUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Remove the now-used pending record.
	pendingCol.DeleteOne(ctx, bson.M{"_id": pendingOID})

	// Redirect to frontend.
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusFound, frontendURL+"/login?verified=true")
}

// ─── Login ───────────────────────────────────────────────────────────────────
//
// Because only verified users are in the users collection, no EmailVerified
// check is needed.
func Login(c *gin.Context) {
	var input struct {
		UserName string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	usersCol := config.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var user models.User
	if err := usersCol.FindOne(ctx, bson.M{"username": input.UserName}).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !utils.CheckPassword(input.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate tokens.
	accessToken, err := utils.GenerateAccessToken(user.ID.Hex(), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}
	refreshToken, err := utils.GenerateRefreshToken(user.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	// Store refresh token in DB.
	refreshCol := config.DB.Collection("refresh_token")
	refreshCol.DeleteMany(ctx, bson.M{"user_id": user.ID})
	refreshDoc := models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if _, err := refreshCol.InsertOne(ctx, refreshDoc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store refresh token"})
		return
	}

	// Set cookies.
	c.SetCookie("access_token", accessToken, 15*60, "/", "", false, true)
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
}

// ─── Logout ──────────────────────────────────────────────────────────────────

func Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token missing"})
		return
	}

	col := config.DB.Collection("refresh_token")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tokenDoc models.RefreshToken
	if err := col.FindOne(ctx, bson.M{"user_id": userID, "token": refreshToken}).Decode(&tokenDoc); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	if _, err := col.DeleteOne(ctx, bson.M{"user_id": userID, "token": refreshToken}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// ─── RefreshAccessToken ───────────────────────────────────────────────────────

func RefreshAccessToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token missing"})
		return
	}

	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return utils.RefreshSecret, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
		return
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id"})
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id format"})
		return
	}

	col := config.DB.Collection("refresh_token")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tokenDoc models.RefreshToken
	if err := col.FindOne(ctx, bson.M{"user_id": userID, "token": refreshToken}).Decode(&tokenDoc); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	accessToken, err := utils.GenerateAccessToken(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	c.SetCookie("access_token", accessToken, 15*60, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Access token refreshed"})
}