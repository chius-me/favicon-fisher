package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type ctxKey int

const requestIDKey ctxKey = 1

// RequestIDFromContext returns the request ID if set by middleware.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// ContextWithRequestID attaches a request ID to ctx.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// SetupJSONLogger configures the default slog logger for structured JSON logs.
func SetupJSONLogger() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(h))
}

// Observability wraps next with request IDs and structured access logs.
func Observability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		ww.Header().Set("X-Request-ID", id)
		ctx := ContextWithRequestID(r.Context(), id)
		r = r.WithContext(ctx)

		next.ServeHTTP(ww, r)

		// Skip noisy static asset noise partially: always log /api/, log others only on error.
		if stringsHasPrefixAPI(r.URL.Path) || ww.status >= 400 {
			level := slog.LevelInfo
			if ww.status >= 500 {
				level = slog.LevelError
			} else if ww.status >= 400 {
				level = slog.LevelWarn
			}
			slog.Log(ctx, level, "request",
				"request_id", id,
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote", ClientIP(r),
			)
		}
	})
}

func stringsHasPrefixAPI(path string) bool {
	return len(path) >= 4 && path[:4] == "/api"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return hex.EncodeToString(b[:])
}
