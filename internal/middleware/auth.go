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

// AuthMiddleware validates the Authorization header against a set of static API keys.
// The Bearer scheme is matched in a case-insensitive manner.
// Unauthorized responses include a `WWW-Authenticate: Bearer` header.
func AuthMiddleware(apiKeys []string, allowNoAuth bool) (gin.HandlerFunc, error) {
	if len(apiKeys) == 0 {
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
		// Check token against all configured keys using constant time comparison.
		match := false
		for _, k := range apiKeys {
			if subtle.ConstantTimeCompare([]byte(token), []byte(k)) == 1 {
				match = true
			}
		}
		if !match {
			c.Header("WWW-Authenticate", "Bearer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}, nil
}
