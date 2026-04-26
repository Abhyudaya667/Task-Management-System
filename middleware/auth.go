package middleware

import (
	"net/http"

	"task-management-system/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)


func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		// 🔹 Step 1: Get access token from cookie
		tokenStr, err := c.Cookie("access_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Access token missing"})
			c.Abort()
			return
		}

		// 🔹 Step 2: Parse & validate token
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {

			// Ensure correct signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}

			return utils.AccessSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 🔹 Step 3: Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// 🔹 Step 4: Get user_id
		userID, ok := claims["user_id"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id"})
			c.Abort()
			return
		}

		// 🔹 Step 5: Store in context
		oid, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id format"})
			c.Abort()
			return
		}
		c.Set("user_id", oid) // now it's primitive.ObjectID
		// c.Set("user_id", userID)
		c.Set("email", claims["email"])
		c.Next()
	}
}