package security

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a simple per-key sliding window limiter.
type RateLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	maxHits  int
	visitors map[string]*visitor
}

type visitor struct {
	hits  int
	start time.Time
}

// NewRateLimiter allows maxHits requests per window per key.
func NewRateLimiter(maxHits int, window time.Duration) *RateLimiter {
	if maxHits <= 0 {
		maxHits = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	rl := &RateLimiter{
		window:   window,
		maxHits:  maxHits,
		visitors: make(map[string]*visitor),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for k, v := range rl.visitors {
			if now.Sub(v.start) > rl.window*2 {
				delete(rl.visitors, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow reports whether key may proceed and consumes one unit if so.
func (rl *RateLimiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	v, ok := rl.visitors[key]
	if !ok || now.Sub(v.start) >= rl.window {
		rl.visitors[key] = &visitor{hits: 1, start: now}
		return true
	}
	if v.hits >= rl.maxHits {
		return false
	}
	v.hits++
	return true
}

// Middleware rate-limits API paths by client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if !rl.Allow(ClientIP(r)) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP extracts a best-effort client IP (direct remote addr only — no trusting XFF by default).
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
