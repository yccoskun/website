package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yccoskun/website/internal/response"
	"github.com/yccoskun/website/internal/securitylog"
)

const (
	loginRateLimitMax        = 10
	loginRateLimitWindow     = 15 * time.Minute
	loginRateLimitMaxEntries = 10_000
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
// Every request counts (failed and successful) because the limiter runs before the handler.
func (l *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r)
		if !l.allow(ip) {
			securitylog.Default.RateLimit(ip, "/api/admin/login")
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
	l.pruneExpired(now)

	a, ok := l.hits[ip]
	if !ok || now.After(a.resetAt) {
		if !ok {
			l.evictIfFull()
		}
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

func (l *LoginRateLimiter) pruneExpired(now time.Time) {
	for ip, a := range l.hits {
		if now.After(a.resetAt) {
			delete(l.hits, ip)
		}
	}
}

// evictIfFull drops the entry with the earliest resetAt when the map is at capacity.
func (l *LoginRateLimiter) evictIfFull() {
	if len(l.hits) < loginRateLimitMaxEntries {
		return
	}
	var victim string
	var earliest time.Time
	first := true
	for ip, a := range l.hits {
		if first || a.resetAt.Before(earliest) {
			victim = ip
			earliest = a.resetAt
			first = false
		}
	}
	if victim != "" {
		delete(l.hits, victim)
	}
}

// ClientIP returns the real client IP when the peer is a trusted loopback proxy
// and Cloudflare's CF-Connecting-IP is present and valid. Otherwise it uses
// RemoteAddr. X-Forwarded-For is never trusted.
func ClientIP(r *http.Request) string {
	peer := peerHost(r.RemoteAddr)
	if isTrustedProxy(peer) {
		if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
			if ip := net.ParseIP(cf); ip != nil {
				return ip.String()
			}
		}
	}
	return peer
}

func peerHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func isTrustedProxy(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
