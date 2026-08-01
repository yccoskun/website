package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginRateLimitSameIP(t *testing.T) {
	limiter := NewLoginRateLimiter()
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < loginRateLimitMax; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
		req.RemoteAddr = "203.0.113.10:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	req.RemoteAddr = "203.0.113.10:54322"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}

	var body struct {
		Data  any     `json:"data"`
		Error *string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data != nil {
		t.Fatalf("data = %#v, want null", body.Data)
	}
	if body.Error == nil || *body.Error != "too many login attempts" {
		t.Fatalf("error = %v, want %q", body.Error, "too many login attempts")
	}
}

func TestLoginRateLimitDifferentCFConnectingIP(t *testing.T) {
	limiter := NewLoginRateLimiter()
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < loginRateLimitMax; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
		req.RemoteAddr = "127.0.0.1:8080"
		req.Header.Set("CF-Connecting-IP", "198.51.100.1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("ip1 attempt %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	// Different real client IP behind the same trusted loopback peer.
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("CF-Connecting-IP", "198.51.100.2")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ip2 status = %d, want 200 (independent bucket)", rec.Code)
	}

	// Original IP still blocked.
	blocked := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	blocked.RemoteAddr = "127.0.0.1:8080"
	blocked.Header.Set("CF-Connecting-IP", "198.51.100.1")
	blockedRec := httptest.NewRecorder()
	handler.ServeHTTP(blockedRec, blocked)
	if blockedRec.Code != http.StatusTooManyRequests {
		t.Fatalf("ip1 status = %d, want 429", blockedRec.Code)
	}
}

func TestLoginRateLimitSpoofedHeadersIgnored(t *testing.T) {
	limiter := NewLoginRateLimiter()
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Non-loopback peer: spoofed CF-Connecting-IP and X-Forwarded-For must be ignored.
	for i := 0; i < loginRateLimitMax; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
		req.RemoteAddr = "203.0.113.50:4000"
		req.Header.Set("CF-Connecting-IP", "198.51.100.99")
		req.Header.Set("X-Forwarded-For", "198.51.100.99")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	// Same peer still limited even with a different spoofed "client" IP.
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	req.RemoteAddr = "203.0.113.50:4001"
	req.Header.Set("CF-Connecting-IP", "203.0.113.1")
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (spoofed headers ignored)", rec.Code)
	}

	// A different real peer is independent.
	other := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	other.RemoteAddr = "203.0.113.51:4000"
	other.Header.Set("CF-Connecting-IP", "198.51.100.99")
	otherRec := httptest.NewRecorder()
	handler.ServeHTTP(otherRec, other)
	if otherRec.Code != http.StatusOK {
		t.Fatalf("other peer status = %d, want 200", otherRec.Code)
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		cfIP       string
		xff        string
		want       string
	}{
		{
			name:       "loopback with CF-Connecting-IP",
			remoteAddr: "127.0.0.1:1234",
			cfIP:       "198.51.100.7",
			want:       "198.51.100.7",
		},
		{
			name:       "ipv6 loopback with CF-Connecting-IP",
			remoteAddr: "[::1]:1234",
			cfIP:       "2001:db8::1",
			want:       "2001:db8::1",
		},
		{
			name:       "loopback without header falls back",
			remoteAddr: "127.0.0.1:1234",
			want:       "127.0.0.1",
		},
		{
			name:       "loopback ignores X-Forwarded-For",
			remoteAddr: "127.0.0.1:1234",
			xff:        "198.51.100.8",
			want:       "127.0.0.1",
		},
		{
			name:       "loopback CF wins over X-Forwarded-For",
			remoteAddr: "127.0.0.1:1234",
			cfIP:       "198.51.100.7",
			xff:        "203.0.113.1",
			want:       "198.51.100.7",
		},
		{
			name:       "non-loopback ignores CF-Connecting-IP",
			remoteAddr: "203.0.113.9:9999",
			cfIP:       "198.51.100.9",
			want:       "203.0.113.9",
		},
		{
			name:       "invalid CF-Connecting-IP falls back",
			remoteAddr: "127.0.0.1:1234",
			cfIP:       "not-an-ip",
			want:       "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.cfIP != "" {
				req.Header.Set("CF-Connecting-IP", tt.cfIP)
			}
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			got := clientIP(req)
			if got != tt.want {
				t.Fatalf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPruneExpiredEntries(t *testing.T) {
	limiter := NewLoginRateLimiter()
	now := time.Now()
	limiter.hits["expired"] = loginAttempt{count: 5, resetAt: now.Add(-time.Minute)}
	limiter.hits["active"] = loginAttempt{count: 1, resetAt: now.Add(time.Minute)}

	limiter.mu.Lock()
	limiter.pruneExpired(now)
	limiter.mu.Unlock()

	if _, ok := limiter.hits["expired"]; ok {
		t.Fatal("expired entry was not pruned")
	}
	if _, ok := limiter.hits["active"]; !ok {
		t.Fatal("active entry was pruned")
	}
}

func TestAllowEvictsWhenFull(t *testing.T) {
	limiter := NewLoginRateLimiter()
	now := time.Now()
	limiter.mu.Lock()
	for i := 0; i < loginRateLimitMaxEntries; i++ {
		limiter.hits[fmt.Sprintf("ip-%d", i)] = loginAttempt{count: 1, resetAt: now.Add(time.Hour)}
	}
	limiter.hits["ip-0"] = loginAttempt{count: 1, resetAt: now.Add(time.Minute)}
	limiter.mu.Unlock()

	if !limiter.allow("new-client") {
		t.Fatal("allow new-client should succeed")
	}
	if got := len(limiter.hits); got > loginRateLimitMaxEntries {
		t.Fatalf("size = %d, want <= %d", got, loginRateLimitMaxEntries)
	}
	if _, ok := limiter.hits["ip-0"]; ok {
		t.Fatal("earliest-reset entry ip-0 should have been evicted")
	}
	if _, ok := limiter.hits["new-client"]; !ok {
		t.Fatal("new-client should be present")
	}
}
