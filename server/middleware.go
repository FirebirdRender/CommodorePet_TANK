package server

import (
	"net/http"
	"strings"
)

// CORSMiddleware adds CORS headers for browser WASM clients.
func CORSMiddleware(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Auth-Token")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware (S6) adds standard security headers.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=()")
		next.ServeHTTP(w, r)
	})
}

// OriginAllowed checks whether a request Origin is in the allowed set.
// A wildcard ("*") allows all origins.
func OriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return true
	}
	for _, o := range allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

// WASMNoCacheMiddleware sets no-cache headers for .wasm and .js files
// to prevent browsers from serving stale WASM binaries after updates.
func WASMNoCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") || strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		next.ServeHTTP(w, r)
	})
}
