package security

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// EnvTrustedProxies is a comma-separated list of CIDRs whose XFF/CF headers we trust.
	EnvTrustedProxies = "FVF_TRUSTED_PROXIES"
	// EnvRateLimit is max API requests per window per client (default 60).
	EnvRateLimit = "FVF_RATE_LIMIT"
	// EnvRateWindow is the fixed window duration, e.g. "1m", "60s" (default 1m).
	EnvRateWindow = "FVF_RATE_WINDOW"
)

// RateLimiter is a fixed-window per-key rate limiter.
type RateLimiter struct {
	mu             sync.Mutex
	window         time.Duration
	maxHits        int
	visitors       map[string]*visitor
	trustedProxies []*net.IPNet
	stop           chan struct{}
}

type visitor struct {
	hits  int
	start time.Time
}

// NewRateLimiter allows maxHits requests per window per key.
func NewRateLimiter(maxHits int, window time.Duration) *RateLimiter {
	return NewRateLimiterWithProxies(maxHits, window, nil)
}

// NewRateLimiterWithProxies creates a limiter that only honors proxy headers when
// the direct peer is in trustedProxies.
func NewRateLimiterWithProxies(maxHits int, window time.Duration, trustedProxies []*net.IPNet) *RateLimiter {
	if maxHits <= 0 {
		maxHits = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	rl := &RateLimiter{
		window:         window,
		maxHits:        maxHits,
		visitors:       make(map[string]*visitor),
		trustedProxies: trustedProxies,
		stop:           make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// RateLimiterFromEnv builds a limiter from FVF_RATE_LIMIT, FVF_RATE_WINDOW, FVF_TRUSTED_PROXIES.
func RateLimiterFromEnv() *RateLimiter {
	maxHits := 60
	if v := strings.TrimSpace(os.Getenv(EnvRateLimit)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxHits = n
		}
	}
	window := time.Minute
	if v := strings.TrimSpace(os.Getenv(EnvRateWindow)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			window = d
		}
	}
	return NewRateLimiterWithProxies(maxHits, window, ParseTrustedProxies(os.Getenv(EnvTrustedProxies)))
}

// ParseTrustedProxies parses comma-separated CIDRs or single IPs.
func ParseTrustedProxies(raw string) []*net.IPNet {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []*net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			if ip := net.ParseIP(part); ip != nil {
				if ip.To4() != nil {
					part += "/32"
				} else {
					part += "/128"
				}
			}
		}
		_, network, err := net.ParseCIDR(part)
		if err != nil {
			continue
		}
		out = append(out, network)
	}
	return out
}

// Stop ends the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	if rl == nil || rl.stop == nil {
		return
	}
	select {
	case <-rl.stop:
	default:
		close(rl.stop)
	}
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
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
}

// Allow reports whether key may proceed and consumes one unit if so.
func (rl *RateLimiter) Allow(key string) bool {
	_, ok := rl.AllowWithRetry(key)
	return ok
}

// AllowWithRetry is like Allow but also returns suggested Retry-After seconds.
func (rl *RateLimiter) AllowWithRetry(key string) (retryAfterSec int, allowed bool) {
	if key == "" {
		key = "unknown"
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	v, ok := rl.visitors[key]
	if !ok || now.Sub(v.start) >= rl.window {
		rl.visitors[key] = &visitor{hits: 1, start: now}
		return 0, true
	}
	if v.hits >= rl.maxHits {
		remaining := rl.window - now.Sub(v.start)
		sec := int(remaining.Seconds()) + 1
		if sec < 1 {
			sec = 1
		}
		return sec, false
	}
	v.hits++
	return 0, true
}

// Middleware rate-limits API paths by client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if retry, ok := rl.AllowWithRetry(rl.ClientIP(r)); !ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP extracts the client IP. Proxy headers are only used when the direct
// peer is in the configured trusted proxy set.
func (rl *RateLimiter) ClientIP(r *http.Request) string {
	remote := directRemoteIP(r)
	if rl == nil || !rl.isTrustedProxy(remote) {
		return remote
	}
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" && net.ParseIP(cf) != nil {
		return cf
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" && net.ParseIP(xr) != nil {
		return xr
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Left-most is the original client when each hop appends.
		for _, part := range strings.Split(xff, ",") {
			ip := strings.TrimSpace(part)
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	return remote
}

// ClientIP is the package-level helper used when no limiter instance is available.
func ClientIP(r *http.Request) string {
	return directRemoteIP(r)
}

func directRemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (rl *RateLimiter) isTrustedProxy(ipStr string) bool {
	if len(rl.trustedProxies) == 0 {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range rl.trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
