package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Incipe-win/OpenStory/internal/auth"
)

// Context keys for storing authenticated user info.
const (
	KeyUserID   = "user_id"
	KeyUsername = "username"
	KeyRole     = "role"
)

// Auth returns a Gin middleware that validates JWT access tokens.
func Auth(jwtSvc *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code": "UNAUTHORIZED", "message": "missing authorization header",
			}})
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code": "UNAUTHORIZED", "message": "invalid authorization format",
			}})
			c.Abort()
			return
		}

		claims, err := jwtSvc.ValidateAccessToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code": "UNAUTHORIZED", "message": "invalid or expired token",
			}})
			c.Abort()
			return
		}

		c.Set(KeyUserID, claims.UserID)
		c.Set(KeyUsername, claims.Username)
		c.Set(KeyRole, claims.Role)
		c.Next()
	}
}

// GetUserID extracts the authenticated user's ID from the Gin context.
func GetUserID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(KeyUserID)
	uid, _ := v.(uuid.UUID)
	return uid
}

func GetRole(c *gin.Context) string {
	v, _ := c.Get(KeyRole)
	role, _ := v.(string)
	return role
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetRole(c) != role {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{
				"code": "FORBIDDEN", "message": "forbidden",
			}})
			c.Abort()
			return
		}
		c.Next()
	}
}
