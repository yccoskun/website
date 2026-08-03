package middleware

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/database"
	"github.com/yccoskun/website/internal/services"
)

func assertPrivateNoStore(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "no-store") || !strings.Contains(cc, "private") {
		t.Fatalf("Cache-Control = %q, want private, no-store", cc)
	}
}

func openSessionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func sessionCleared(rec *httptest.ResponseRecorder) bool {
	for _, h := range rec.Header().Values("Set-Cookie") {
		c, err := http.ParseSetCookie(h)
		if err != nil {
			continue
		}
		if c.Name == auth.SessionCookieName && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error *string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, rec.Body.String())
	}
	if body.Error == nil {
		return ""
	}
	return *body.Error
}

func TestRequireSessionRejectsDisallowedSecFetchSite(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	token, _, err := sessions.Create("", "", false)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireSession(sessions, false, next)

	cases := []struct {
		name   string
		site   string
		cookie bool
	}{
		{name: "cross-site", site: "cross-site", cookie: true},
		{name: "Cross-Site case-insensitive", site: "Cross-Site", cookie: true},
		{name: "cross-site without cookie", site: "cross-site", cookie: false},
		{name: "unknown evil", site: "evil", cookie: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
			if tc.cookie {
				req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
			}
			req.Header.Set("Sec-Fetch-Site", tc.site)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
			}
			assertPrivateNoStore(t, rec)
		})
	}
}

func TestRequireSessionAllowsSameOriginAndMissing(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	token, _, err := sessions.Create("", "", false)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireSession(sessions, false, next)

	cases := []struct {
		name string
		site string
	}{
		{name: "missing", site: ""},
		{name: "same-origin", site: "same-origin"},
		{name: "SAME-ORIGIN case-insensitive", site: "SAME-ORIGIN"},
		{name: "same-site", site: "same-site"},
		{name: "none", site: "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
			if tc.site != "" {
				req.Header.Set("Sec-Fetch-Site", tc.site)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			assertPrivateNoStore(t, rec)
		})
	}
}

func TestRequireSessionUnauthorizedWithoutCookie(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	handler := RequireSession(sessions, false, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertPrivateNoStore(t, rec)
}

func TestRequireSessionBindingMismatchClearsCookie(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	ua := "GoodAgent/1.0"
	ip := "198.51.100.10"
	token, _, err := sessions.Create(ua, ip, true)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	handler := RequireSession(sessions, true, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("different UA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
		req.RemoteAddr = "127.0.0.1:9"
		req.Header.Set("User-Agent", "EvilAgent/9.0")
		req.Header.Set("CF-Connecting-IP", ip)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if got := errorMessage(t, rec); got != "reauth_required" {
			t.Fatalf("error = %q, want reauth_required", got)
		}
		if !sessionCleared(rec) {
			t.Fatal("expected session cookie cleared")
		}
		assertPrivateNoStore(t, rec)
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		if n != 0 {
			t.Fatalf("sessions after mismatch = %d, want 0 (destroyed)", n)
		}
	})

	// Recreate after destroy from previous subtest.
	token, _, err = sessions.Create(ua, ip, true)
	if err != nil {
		t.Fatalf("recreate session: %v", err)
	}

	t.Run("different /24", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
		req.RemoteAddr = "127.0.0.1:9"
		req.Header.Set("User-Agent", ua)
		req.Header.Set("CF-Connecting-IP", "203.0.113.1")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if got := errorMessage(t, rec); got != "reauth_required" {
			t.Fatalf("error = %q, want reauth_required", got)
		}
		if !sessionCleared(rec) {
			t.Fatal("expected session cookie cleared")
		}
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		if n != 0 {
			t.Fatalf("sessions after mismatch = %d, want 0 (destroyed)", n)
		}
	})
}

func TestRequireSessionBindingMatchOK(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	ua := "GoodAgent/1.0"
	token, _, err := sessions.Create(ua, "198.51.100.10", true)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	handler := RequireSession(sessions, true, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.Header.Set("User-Agent", ua)
	req.Header.Set("CF-Connecting-IP", "198.51.100.99") // same /24
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestRequireSessionBindingFlagOffIgnoresMismatch(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	token, _, err := sessions.Create("GoodAgent/1.0", "198.51.100.10", true)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	handler := RequireSession(sessions, false, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.Header.Set("User-Agent", "EvilAgent")
	req.Header.Set("CF-Connecting-IP", "203.0.113.1")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with binding off", rec.Code)
	}
}
