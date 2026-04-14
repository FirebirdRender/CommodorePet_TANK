package server

import (
	"net/http"
	"net/http/httptest"
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
