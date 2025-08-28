package middleware_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
		{"first key", "Bearer secret1", http.StatusOK},
		{"second key", "Bearer secret2", http.StatusOK},
		{"valid token with spaces", "  Bearer    secret1  ", http.StatusOK},
		{"lowercase bearer", "bearer secret1", http.StatusOK},
		{"uppercase bearer", "BEARER secret1", http.StatusOK},
		{"invalid token", "Bearer wrong", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			h, err := middleware.AuthMiddleware(logger, []string{"secret1", "secret2"}, nil, false)
			require.NoError(t, err)
			router.Use(h)
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

func TestAuthMiddlewareHashedTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h1 := sha256.Sum256([]byte("secret1"))
	h2 := sha256.Sum256([]byte("secret2"))
	hashes := []string{strings.ToUpper(hex.EncodeToString(h1[:])), strings.ToUpper(hex.EncodeToString(h2[:]))}

	tests := []struct {
		name     string
		header   string
		expected int
	}{
		{"first hash", "Bearer secret1", http.StatusOK},
		{"second hash", "Bearer secret2", http.StatusOK},
		{"invalid token", "Bearer wrong", http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			h, err := middleware.AuthMiddleware(logger, nil, hashes, false)
			require.NoError(t, err)
			router.Use(h)
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

func TestAuthMiddlewareNoAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("error when not allowed", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		_, err := middleware.AuthMiddleware(logger, nil, nil, false)
		require.Error(t, err)
	})

	t.Run("allow when explicitly permitted", func(t *testing.T) {
		router := gin.New()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		h, err := middleware.AuthMiddleware(logger, nil, nil, true)
		require.NoError(t, err)
		router.Use(h)
		router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAuthMiddlewareLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		header     string
		logMessage string
		notInLog   []string
	}{
		{"missing header", "", "missing Authorization header", nil},
		{"malformed header", "Token foo", "malformed Authorization header", nil},
		{"invalid token", "Bearer wrong", "invalid API key", []string{"wrong"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			router := gin.New()
			h, err := middleware.AuthMiddleware(logger, []string{"secret1"}, nil, false)
			require.NoError(t, err)
			router.Use(h)
			router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			req.RemoteAddr = "1.2.3.4:5678"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusUnauthorized, w.Code)

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			require.Len(t, lines, 1)

			var entry map[string]any
			err = json.Unmarshal([]byte(lines[0]), &entry)
			require.NoError(t, err)
			require.Equal(t, tc.logMessage, entry["msg"])
			require.Equal(t, "1.2.3.4", entry["ip"])
			require.Equal(t, "/", entry["path"])
			for _, s := range tc.notInLog {
				require.NotContains(t, lines[0], s)
			}
		})
	}
}
