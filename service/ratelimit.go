package service

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a fixed-window in-memory limiter per client IP,
// mirroring express-rate-limit with standardHeaders (RateLimit-*).
type RateLimiter struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	hits   map[string][]time.Time
	now    func() time.Time
}

func NewRateLimiter(window time.Duration, max int) *RateLimiter {
	return &RateLimiter{
		window: window,
		max:    max,
		hits:   make(map[string][]time.Time),
		now:    time.Now,
	}
}

func clientIP(r *http.Request) string {
	// Behind proxies X-Forwarded-For is "client, proxy1, ...": the
	// client is always the first entry. Using the whole header (or
	// the proxy RemoteAddr) collapses every user into one bucket.
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip, _, _ := strings.Cut(fwd, ","); strings.TrimSpace(ip) != "" {
			return strings.TrimSpace(ip)
		}
	}
	if real := r.Header.Get("X-Real-Ip"); real != "" {
		return strings.TrimSpace(real)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Allow records a hit and reports whether it fits the window.
// It always sets the RateLimit-* headers on w.
func (l *RateLimiter) Allow(w http.ResponseWriter, r *http.Request) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	key := clientIP(r)
	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	allowed := len(kept) < l.max
	if allowed {
		kept = append(kept, now)
	}
	l.hits[key] = kept
	remaining := l.max - len(kept)
	if remaining < 0 {
		remaining = 0
	}
	w.Header().Set("RateLimit-Limit", strconv.Itoa(l.max))
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("RateLimit-Reset", strconv.FormatInt(now.Add(l.window).Unix(), 10))
	return allowed
}

// Middleware wraps h, answering 429 with message once the quota ends.
func (l *RateLimiter) Middleware(message string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(w, r) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":` + strconv.Quote(message) + `}`))
			return
		}
		h.ServeHTTP(w, r)
	})
}
