package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yccoskun/website/internal/database"
	"github.com/yccoskun/website/internal/services"
)

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

func TestRequireSessionRejectsDisallowedSecFetchSite(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	token, _, err := sessions.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireSession(sessions, next)

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
				req.AddCookie(&http.Cookie{Name: "session", Value: token})
			}
			req.Header.Set("Sec-Fetch-Site", tc.site)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRequireSessionAllowsSameOriginAndMissing(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	token, _, err := sessions.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireSession(sessions, next)

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
			req.AddCookie(&http.Cookie{Name: "session", Value: token})
			if tc.site != "" {
				req.Header.Set("Sec-Fetch-Site", tc.site)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRequireSessionUnauthorizedWithoutCookie(t *testing.T) {
	db := openSessionTestDB(t)
	sessions := services.NewSessionService(db)
	handler := RequireSession(sessions, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
