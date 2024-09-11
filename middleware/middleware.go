package middleware

import (
	"go-novel/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken := c.GetHeader("Authorization")
		if accessToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		accessToken = strings.TrimPrefix(accessToken, "Bearer ")

		claims, err := utils.ValidateAccessToken(accessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token"})
			return
		}

		userID := uint(claims["id"].(float64))
		username := claims["username"].(string)
		role := claims["role"].(string)

		c.Set("userID", userID)
		c.Set("username", username)
		c.Set("role", role)

		c.Next()
	}
}
