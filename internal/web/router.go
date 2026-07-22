package web

import (
	"net/http"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
)

// NewMux wires API routes and static assets with security middleware.
func NewMux(handler *Handler, staticFS http.FileSystem) http.Handler {
	return NewMuxWithLimiter(handler, staticFS, security.NewRateLimiter(60, time.Minute))
}

// NewMuxWithLimiter allows tests to inject a custom rate limiter (or nil to disable).
func NewMuxWithLimiter(handler *Handler, staticFS http.FileSystem, limiter *security.RateLimiter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/preview", handler.Preview)
	mux.HandleFunc("/api/download", handler.Download)
	mux.HandleFunc("/api/proxy", handler.Proxy)
	mux.Handle("/", http.FileServer(staticFS))

	var h http.Handler = mux
	h = security.SecurityHeaders(h)
	if limiter != nil {
		h = limiter.Middleware(h)
	}
	return h
}
