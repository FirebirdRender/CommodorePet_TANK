package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddlewareSetsHeaders(t *testing.T) {
	handler := CORSMiddleware("https://example.com", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test OPTIONS preflight
	req := httptest.NewRequest("OPTIONS", "/ws", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("OPTIONS: expected 200, got %d", rec.Code)
	}
	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "https://example.com" {
		t.Errorf("CORS origin: expected https://example.com, got %s", origin)
	}

	// Test normal GET
	req = httptest.NewRequest("GET", "/health", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET: expected 200, got %d", rec.Code)
	}
	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "https://example.com" {
		t.Errorf("CORS origin on GET: expected https://example.com, got %s", origin)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	tests := []struct {
		header string
		want   string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "no-referrer"},
		{"Permissions-Policy", "geolocation=(), microphone=()"},
	}
	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got != tt.want {
			t.Errorf("header %s: expected %q, got %q", tt.header, tt.want, got)
		}
	}
}

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowedOrigins []string
		want           bool
	}{
		{"wildcard", "https://evil.com", []string{"*"}, true},
		{"exact_match", "https://example.com", []string{"https://example.com"}, true},
		{"no_match", "https://evil.com", []string{"https://example.com"}, false},
		{"empty_allows_all", "https://anything.com", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OriginAllowed(tt.origin, tt.allowedOrigins)
			if got != tt.want {
				t.Errorf("OriginAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowedOrigins, got, tt.want)
			}
		})
	}
}

func TestRequestTooLarge(t *testing.T) {
	hub := NewHub()
	ts := NewTokenStore()
	reg := NewConnRegistry()
	api := NewRoomAPI(hub, reg, ts, "*")

	// 5KB body should be rejected (limit is 4KB)
	largeBody := `{"difficulty": 5, "player_name": "` + strings.Repeat("A", 5000) + `"}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/room", strings.NewReader(largeBody))
	api.handleCreateRoom(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for oversized body, got %d", w.Code)
	}
}

func TestWASMNoCacheMiddleware(t *testing.T) {
	handler := WASMNoCacheMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test .wasm file
	req := httptest.NewRequest("GET", "/game.wasm", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache, no-store, must-revalidate" {
		t.Errorf("wasm Cache-Control: expected 'no-cache, no-store, must-revalidate', got %q", cc)
	}

	// Test .js file
	req = httptest.NewRequest("GET", "/wasm_exec.js", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cc = rec.Header().Get("Cache-Control")
	if cc != "no-cache, no-store, must-revalidate" {
		t.Errorf("js Cache-Control: expected 'no-cache, no-store, must-revalidate', got %q", cc)
	}

	// Test .html file — should NOT have no-cache
	req = httptest.NewRequest("GET", "/index.html", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cc = rec.Header().Get("Cache-Control")
	if cc != "" {
		t.Errorf("html Cache-Control: expected empty, got %q", cc)
	}
}
