package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"tagscale/internal/middleware"
)

func TestAuthMiddlewareTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		header   string
		expected int
	}{
		{"valid token", "Bearer secret", http.StatusOK},
		{"valid token with spaces", "  Bearer    secret  ", http.StatusOK},
		{"lowercase bearer", "bearer secret", http.StatusOK},
		{"uppercase bearer", "BEARER secret", http.StatusOK},
		{"invalid token", "Bearer wrong", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(middleware.AuthMiddleware("secret"))
			router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tc.expected, w.Code)
			if tc.expected == http.StatusUnauthorized {
				require.Equal(t, "Bearer", w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}
