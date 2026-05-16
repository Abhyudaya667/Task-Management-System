package controllers

import (
	"context"
	"net/http"
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

func Register(c *gin.Context) {
	var user models.User

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := config.DB.Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if email already exists
	var existingUser models.User
	err := collection.FindOne(
    ctx,
    bson.M{
        "$or": []bson.M{
            {"email": user.Email},
            {"username": user.UserName},
        },
    },
	).Decode(&existingUser)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email or UserName already registered"})
		return
	}
	if err != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Hash password - handle the error!
	hashedPassword, err := utils.HashPassword(user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.Password = hashedPassword
	user.CreatedAt = time.Now()

	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not created"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
}
func Login(c *gin.Context) {
	var input struct {
		UserName    string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// Validate request
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := config.DB.Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find user by email
	var user models.User
	err := collection.FindOne(ctx, bson.M{"username": input.UserName}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check password
	if !utils.CheckPassword(input.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate Access Token (short-lived)
	accessToken, err := utils.GenerateAccessToken(user.ID.Hex(), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	// Generate Refresh Token (long-lived)
	refreshToken, err := utils.GenerateRefreshToken(user.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}
	
	// Store Refresh Token in DB
	refreshCollection := config.DB.Collection("refresh_token")
	refreshCollection.DeleteMany(ctx, bson.M{"user_id": user.ID})
	refreshDoc := models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	_, err = refreshCollection.InsertOne(ctx, refreshDoc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store refresh token"})
		return
	}

	//  Store BOTH tokens in cookies
	println(accessToken)
	print(refreshToken)
	// Access Token (15 minutes)
	c.SetCookie(
		"access_token",
		accessToken,
		15*60, // seconds
		"/",
		"",
		false, // set true in production (HTTPS)
		true,  // HttpOnly
	)

	// Refresh Token (7 days)
	c.SetCookie(
		"refresh_token",
		refreshToken,
		7*24*60*60,
		"/",
		"",
		false,
		true,
	)

	// Response
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
	})
}
func Logout(c *gin.Context) {

	// 🔹 Step 1: Get user_id from middleware
	// userIDVal, exists := c.Get("user_id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// userIDStr, ok := userIDVal.(string)
	// if !ok {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id"})
	// 	return
	// }

	// userID, err := primitive.ObjectIDFromHex(userIDStr)
	// if err != nil {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id format"})
	// 	return
	// }

	// 🔹 Step 2: Get refresh token from cookie
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token missing"})
		return
	}

	collection := config.DB.Collection("refresh_token")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 🔹 Step 3: Verify refresh token exists in DB
	var tokenDoc models.RefreshToken

	err = collection.FindOne(ctx, bson.M{
		"user_id": userID,
		"token":   refreshToken,
	}).Decode(&tokenDoc)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	// 🔹 Step 4: Delete refresh token (this session only)
	_, err = collection.DeleteOne(ctx, bson.M{
		"user_id": userID,
		"token":   refreshToken,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// 🔹 Step 5: Clear cookies
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

	// 🔹 Step 6: Response
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}
func RefreshAccessToken(c *gin.Context) {

	// 🔹 Step 1: Get refresh token from cookie
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token missing"})
		return
	}

	// 🔹 Step 2: Parse & validate refresh token (JWT)
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

	// 🔹 Step 3: Extract claims
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

	// 🔹 Step 4: Check refresh token exists in DB (same token, same user)
	collection := config.DB.Collection("refresh_token")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tokenDoc models.RefreshToken

	err = collection.FindOne(ctx, bson.M{
		"user_id": userID,
		"token":   refreshToken,
	}).Decode(&tokenDoc)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	// 🔹 Step 5: Generate new access token
	accessToken, err := utils.GenerateAccessToken(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	// 🔹 Step 6: Update access token cookie
	c.SetCookie(
		"access_token",
		accessToken,
		15*60, // 15 minutes
		"/",
		"",
		false, // true in production
		true,  // HttpOnly
	)

	// 🔹 Step 7: Response
	c.JSON(http.StatusOK, gin.H{
		"message": "Access token refreshed",
	})
}