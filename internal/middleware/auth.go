// internal/middleware/auth.go
package middleware

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the Authorization header against a static API key.
// The Bearer scheme is matched in a case-insensitive manner.
// Unauthorized responses include a `WWW-Authenticate: Bearer` header.
func AuthMiddleware(apiKey string, allowNoAuth bool) (gin.HandlerFunc, error) {
	if apiKey == "" {
		if allowNoAuth {
			log.Println("Warning: starting without API key; authentication disabled")
			return func(c *gin.Context) { c.Next() }, nil
		}
		return nil, fmt.Errorf("API key is required unless ALLOW_NO_AUTH is set")
	}

	return func(c *gin.Context) {

		// Normalize whitespace to ensure consistent parsing of the auth header.
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.Header("WWW-Authenticate", "Bearer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Fields(authHeader)
		// Match the Bearer scheme case-insensitively to be RFC compliant.
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Header("WWW-Authenticate", "Bearer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		token := parts[1]
		// Use constant time comparison to mitigate timing attacks on API key checks.
		if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}, nil
}
