package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/yccoskun/website/internal/response"
)

const (
	loginRateLimitMax     = 10
	loginRateLimitWindow  = 15 * time.Minute
)

type loginAttempt struct {
	count   int
	resetAt time.Time
}

// LoginRateLimiter limits login attempts per client IP.
type LoginRateLimiter struct {
	mu   sync.Mutex
	hits map[string]loginAttempt
}

// NewLoginRateLimiter constructs an in-memory per-IP login rate limiter.
func NewLoginRateLimiter() *LoginRateLimiter {
	return &LoginRateLimiter{hits: make(map[string]loginAttempt)}
}

// Middleware wraps a login handler and returns 429 when the IP exceeds the limit.
func (l *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.allow(ip) {
			response.Error(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *LoginRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	a, ok := l.hits[ip]
	if !ok || now.After(a.resetAt) {
		l.hits[ip] = loginAttempt{count: 1, resetAt: now.Add(loginRateLimitWindow)}
		return true
	}
	if a.count >= loginRateLimitMax {
		return false
	}
	a.count++
	l.hits[ip] = a
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
