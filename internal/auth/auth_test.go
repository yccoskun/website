package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestConstantTimeUsernameEqual_match(t *testing.T) {
	if !ConstantTimeUsernameEqual("admin", "admin") {
		t.Fatal("expected equal usernames to match")
	}
}

func TestConstantTimeUsernameEqual_mismatch(t *testing.T) {
	if ConstantTimeUsernameEqual("admin", "other") {
		t.Fatal("expected different usernames to not match")
	}
}

func TestConstantTimeUsernameEqual_empty(t *testing.T) {
	if !ConstantTimeUsernameEqual("", "") {
		t.Fatal("expected two empty strings to match")
	}
	if ConstantTimeUsernameEqual("", "admin") {
		t.Fatal("expected empty vs non-empty to not match")
	}
	if ConstantTimeUsernameEqual("admin", "") {
		t.Fatal("expected non-empty vs empty to not match")
	}
}

func TestConstantTimeUsernameEqual_unequalLengths(t *testing.T) {
	if ConstantTimeUsernameEqual("admin", "adm") {
		t.Fatal("expected prefix mismatch to not match")
	}
	if ConstantTimeUsernameEqual("a", "ab") {
		t.Fatal("expected unequal lengths to not match")
	}
}

func TestConstantTimeUsernameEqual_overMaxLength(t *testing.T) {
	long := strings.Repeat("a", maxUsernameLength+1)
	ok := strings.Repeat("a", maxUsernameLength)

	if ConstantTimeUsernameEqual(long, long) {
		t.Fatal("expected over-max username to not match")
	}
	if ConstantTimeUsernameEqual(long, ok) {
		t.Fatal("expected over-max vs valid length to not match")
	}
	if ConstantTimeUsernameEqual(ok, long) {
		t.Fatal("expected valid length vs over-max to not match")
	}
}

func setSessionCookieForHost(t *testing.T, host string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, req, "sometoken", time.Now().Add(time.Hour))
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	return cookies[0]
}

func clearSessionCookieForHost(t *testing.T, host string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, req)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	return cookies[0]
}

func assertCookieFlags(t *testing.T, c *http.Cookie, wantSecure bool) {
	t.Helper()
	if !c.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.Secure != wantSecure {
		t.Errorf("Secure = %v, want %v", c.Secure, wantSecure)
	}
}

func TestSetSessionCookie_publicHostIsSecure(t *testing.T) {
	hosts := []string{
		"www.example.com",
		"www.example.com:443",
		"abc123.trycloudflare.com",
	}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			assertCookieFlags(t, setSessionCookieForHost(t, host), true)
		})
	}
}

func TestSetSessionCookie_loopbackHostIsNotSecure(t *testing.T) {
	hosts := []string{
		"localhost",
		"localhost:9000",
		"127.0.0.1",
		"127.0.0.1:9000",
		"[::1]",
		"[::1]:9000",
	}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			assertCookieFlags(t, setSessionCookieForHost(t, host), false)
		})
	}
}

func TestClearSessionCookie_publicHostIsSecure(t *testing.T) {
	assertCookieFlags(t, clearSessionCookieForHost(t, "www.example.com"), true)
}

func TestClearSessionCookie_loopbackHostIsNotSecure(t *testing.T) {
	assertCookieFlags(t, clearSessionCookieForHost(t, "localhost"), false)
}

func TestCheckPassword_emptyInputsFail(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if CheckPassword("", "secret") {
		t.Error("expected empty hash to fail")
	}
	if CheckPassword(string(hash), "") {
		t.Error("expected empty password to fail")
	}
	if CheckPassword("", "") {
		t.Error("expected empty hash and password to fail")
	}
}

func TestCheckPassword_matchAndMismatch(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if !CheckPassword(string(hash), "correct horse") {
		t.Error("expected matching password to pass CheckPassword")
	}
	if CheckPassword(string(hash), "wrong password") {
		t.Error("expected mismatched password to fail CheckPassword")
	}
}
