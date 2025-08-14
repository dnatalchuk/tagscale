package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tagscale/internal/config"
	"tagscale/internal/server"
)

func TestCustomCORSOrigins(t *testing.T) {
	cfg := &config.Config{AllowedOrigins: []string{"https://example.com"}, AllowNoAuth: true}
	srv := server.New(cfg, nil, nil, nil)
	handler := srv.Router()

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("expected Access-Control-Allow-Origin %q, got %q", "https://example.com", got)
	}
}
