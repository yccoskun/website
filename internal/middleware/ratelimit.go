package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yccoskun/website/internal/config"
	"github.com/yccoskun/website/internal/response"
	"github.com/yccoskun/website/internal/securitylog"
)

const (
	loginRateLimitMax          = 10
	loginRateLimitWindow       = 15 * time.Minute
	loginRateLimitMaxEntries   = 10_000
	loginRateLimitPruneDefault = time.Minute
)

// Overridable in tests via t.Cleanup restore.
var loginRateLimitPruneInterval = loginRateLimitPruneDefault

type loginAttempt struct {
	count   int
	resetAt time.Time
}

// ProxyTrust decides whether a peer may present CF-Connecting-IP.
// The zero value trusts nothing (secure default).
type ProxyTrust struct {
	nets []*net.IPNet
	unix bool
}

// NewProxyTrust builds a ProxyTrust from parsed config allowlist entries.
func NewProxyTrust(tp config.TrustedProxies) ProxyTrust {
	return ProxyTrust{nets: tp.Nets, unix: tp.Unix}
}

// LoginRateLimiter limits login attempts per client IP.
type LoginRateLimiter struct {
	mu    sync.Mutex
	hits  map[string]loginAttempt
	trust ProxyTrust
}

// NewLoginRateLimiter constructs an in-memory per-IP login rate limiter.
func NewLoginRateLimiter(trust ProxyTrust) *LoginRateLimiter {
	return &LoginRateLimiter{hits: make(map[string]loginAttempt), trust: trust}
}

// RunPruneLoop periodically removes expired IP entries until ctx is cancelled.
// Not started by NewLoginRateLimiter; callers must start it when desired.
func (l *LoginRateLimiter) RunPruneLoop(ctx context.Context) {
	ticker := time.NewTicker(loginRateLimitPruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			l.mu.Lock()
			l.pruneExpired(now)
			l.mu.Unlock()
		}
	}
}

// Middleware wraps a login handler and returns 429 when the IP exceeds the limit.
// Every request counts (failed and successful) because the limiter runs before the handler.
// Trusted peers without a valid CF-Connecting-IP get 400 and are not counted.
func (l *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := l.trust.ClientIP(r)
		if ip == "" {
			response.Error(w, http.StatusBadRequest, "missing client ip")
			return
		}
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

	a, ok := l.hits[ip]
	if !ok || now.After(a.resetAt) {
		if !ok {
			if len(l.hits) >= loginRateLimitMaxEntries {
				l.pruneExpired(now)
				l.evictIfFull()
			}
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

// ClientIP returns the real client IP when the peer is on the trusted-proxy
// allowlist and Cloudflare's CF-Connecting-IP is present and valid.
// Trusted peers with a missing or invalid header return "" (fail closed).
// Untrusted peers use RemoteAddr and ignore CF-Connecting-IP.
// X-Forwarded-For is never trusted.
func (t ProxyTrust) ClientIP(r *http.Request) string {
	peer := peerHost(r.RemoteAddr)
	if !t.trusted(r.RemoteAddr) {
		return peer
	}
	cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
	if cf == "" {
		return ""
	}
	ip := net.ParseIP(cf)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func (t ProxyTrust) trusted(remoteAddr string) bool {
	peer := peerHost(remoteAddr)
	if t.unix && isUnixRemote(peer) {
		return true
	}
	ip := net.ParseIP(peer)
	if ip == nil {
		return false
	}
	for _, n := range t.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func peerHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// isUnixRemote reports whether addr looks like a Unix-domain peer as seen by
// net/http (abstract "@…", bare "@", or a filesystem path).
func isUnixRemote(addr string) bool {
	if addr == "@" || strings.HasPrefix(addr, "@") {
		return true
	}
	return strings.Contains(addr, "/")
}
