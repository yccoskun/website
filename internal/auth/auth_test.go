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

func setSessionCookiesForHost(t *testing.T, host string) (session, legacy *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, req, "sometoken", time.Now().Add(time.Hour))
	return findSessionAndLegacy(t, rec)
}

func clearSessionCookiesForHost(t *testing.T, host string) (session, legacy *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, req)
	return findSessionAndLegacy(t, rec)
}

func findSessionAndLegacy(t *testing.T, rec *httptest.ResponseRecorder) (session, legacy *http.Cookie) {
	t.Helper()
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies (session + legacy clear), got %d: %v", len(cookies), cookieNames(cookies))
	}
	for _, c := range cookies {
		switch c.Name {
		case SessionCookieName:
			session = c
		case legacySessionCookieName:
			legacy = c
		}
	}
	if session == nil {
		t.Fatalf("missing %s cookie among %v", SessionCookieName, cookieNames(cookies))
	}
	if legacy == nil {
		t.Fatalf("missing legacy %s cookie among %v", legacySessionCookieName, cookieNames(cookies))
	}
	return session, legacy
}

func cookieNames(cookies []*http.Cookie) []string {
	names := make([]string, len(cookies))
	for i, c := range cookies {
		names[i] = c.Name
	}
	return names
}

func assertHostSessionFlags(t *testing.T, c *http.Cookie) {
	t.Helper()
	if c.Name != SessionCookieName {
		t.Errorf("Name = %q, want %q", c.Name, SessionCookieName)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
	if c.Domain != "" {
		t.Errorf("Domain = %q, want empty (no Domain for __Host-)", c.Domain)
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if !c.Secure {
		t.Error("expected Secure=true (__Host- requirement)")
	}
}

func assertLegacyClear(t *testing.T, c *http.Cookie, wantSecure bool) {
	t.Helper()
	if c.Name != legacySessionCookieName {
		t.Errorf("legacy Name = %q, want %q", c.Name, legacySessionCookieName)
	}
	if c.Path != "/" {
		t.Errorf("legacy Path = %q, want /", c.Path)
	}
	// MaxAge < 0 is the clear signal; Go's parser may normalize Max-Age, so
	// also accept a past Expires.
	if c.MaxAge >= 0 && !c.Expires.Before(time.Now()) {
		t.Errorf("legacy MaxAge = %d, Expires = %v, want expiry clear", c.MaxAge, c.Expires)
	}
	if !c.HttpOnly {
		t.Error("expected legacy HttpOnly=true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("legacy SameSite = %v, want Lax", c.SameSite)
	}
	if c.Secure != wantSecure {
		t.Errorf("legacy Secure = %v, want %v", c.Secure, wantSecure)
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
			session, legacy := setSessionCookiesForHost(t, host)
			assertHostSessionFlags(t, session)
			if session.Value != "sometoken" {
				t.Errorf("Value = %q, want sometoken", session.Value)
			}
			assertLegacyClear(t, legacy, true)
		})
	}
}

func TestSetSessionCookie_loopbackHostIsSecure(t *testing.T) {
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
			session, legacy := setSessionCookiesForHost(t, host)
			assertHostSessionFlags(t, session)
			assertLegacyClear(t, legacy, false)
		})
	}
}

func TestClearSessionCookie_publicHostIsSecure(t *testing.T) {
	session, legacy := clearSessionCookiesForHost(t, "www.example.com")
	assertHostSessionFlags(t, session)
	if session.MaxAge >= 0 && !session.Expires.Before(time.Now()) {
		t.Error("expected session cookie clear (MaxAge < 0 or past Expires)")
	}
	assertLegacyClear(t, legacy, true)
}

func TestClearSessionCookie_loopbackHostIsSecure(t *testing.T) {
	session, legacy := clearSessionCookiesForHost(t, "localhost")
	assertHostSessionFlags(t, session)
	if session.MaxAge >= 0 && !session.Expires.Before(time.Now()) {
		t.Error("expected session cookie clear (MaxAge < 0 or past Expires)")
	}
	assertLegacyClear(t, legacy, false)
}

func TestSessionToken_readsOnlyHostSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: legacySessionCookieName, Value: "legacy-token"})
	if got := SessionToken(req); got != "" {
		t.Fatalf("SessionToken with only legacy cookie = %q, want empty", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "host-token"})
	if got := SessionToken(req); got != "host-token" {
		t.Fatalf("SessionToken = %q, want host-token", got)
	}
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
