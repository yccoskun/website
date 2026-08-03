package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/config"
	"github.com/yccoskun/website/internal/database"
	"github.com/yccoskun/website/internal/securitylog"
	"github.com/yccoskun/website/internal/services"
	"golang.org/x/crypto/bcrypt"
)

func newTestRouter() http.Handler {
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html>spa"))
	})
	return NewRouter(spa, Deps{})
}

func doRequest(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)
	return rec
}

func assertEnvelope(t *testing.T, rec *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func TestHealthReturnsEnvelope(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/api/health")
	assertEnvelope(t, rec, http.StatusOK, `{"data":{"status":"ok"},"error":null}`)
	assertSecurityHeaders(t, rec)
}

func assertSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	}
	for k, want := range checks {
		if got := rec.Header().Get(k); got != want {
			t.Fatalf("%s = %q, want %q", k, got, want)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header missing")
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("Content-Security-Policy = %q, want script-src 'self'", csp)
	}
	if !strings.Contains(csp, "sha256-") {
		t.Fatalf("Content-Security-Policy = %q, want sha256- token for theme boot", csp)
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Fatalf("Content-Security-Policy = %q, must not include unsafe-inline", csp)
	}
	if strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("Content-Security-Policy = %q, must not include unsafe-eval", csp)
	}
	// HSTS belongs at Cloudflare edge — never on the Go app (localhost / non-edge).
	if sts := rec.Header().Get("Strict-Transport-Security"); sts != "" {
		t.Fatalf("Strict-Transport-Security = %q, want absent (edge-owned)", sts)
	}
}

func TestSPAHasSecurityHeaders(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertSecurityHeaders(t, rec)
}

func TestRobotsTxt(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{
		SiteURL: "https://www.yusufcancoskun.com",
	})
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Disallow: /admin") {
		t.Fatalf("body = %q, want Disallow: /admin", body)
	}
	if !strings.Contains(body, "Disallow: /api/admin") {
		t.Fatalf("body = %q, want Disallow: /api/admin", body)
	}
	if !strings.Contains(body, "Sitemap: https://www.yusufcancoskun.com/sitemap.xml") {
		t.Fatalf("body = %q, want Sitemap URL", body)
	}
	assertSecurityHeaders(t, rec)
}

func TestSitemapAndRSSEmptyPosts(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{
		SiteURL: "https://www.yusufcancoskun.com",
	})

	sitemapReq := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	sitemapRec := httptest.NewRecorder()
	router.ServeHTTP(sitemapRec, sitemapReq)
	if sitemapRec.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d, want 200", sitemapRec.Code)
	}
	if ct := sitemapRec.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Fatalf("sitemap content-type = %q, want xml", ct)
	}
	if !strings.Contains(sitemapRec.Body.String(), "<urlset") {
		t.Fatalf("sitemap body = %q, want <urlset", sitemapRec.Body.String())
	}
	if !strings.Contains(sitemapRec.Body.String(), "https://www.yusufcancoskun.com/blog") {
		t.Fatalf("sitemap missing /blog URL")
	}
	if !strings.Contains(sitemapRec.Body.String(), "https://www.yusufcancoskun.com/work") {
		t.Fatalf("sitemap missing /work URL")
	}
	if !strings.Contains(sitemapRec.Body.String(), "https://www.yusufcancoskun.com/studio") {
		t.Fatalf("sitemap missing /studio URL")
	}
	assertSecurityHeaders(t, sitemapRec)

	rssReq := httptest.NewRequest(http.MethodGet, "/rss.xml", nil)
	rssRec := httptest.NewRecorder()
	router.ServeHTTP(rssRec, rssReq)
	if rssRec.Code != http.StatusOK {
		t.Fatalf("rss status = %d, want 200", rssRec.Code)
	}
	if ct := rssRec.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Fatalf("rss content-type = %q, want xml", ct)
	}
	if !strings.Contains(rssRec.Body.String(), "<rss") {
		t.Fatalf("rss body = %q, want <rss", rssRec.Body.String())
	}
}

func TestFeedSiteURLTrimsTrailingSlash(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{
		SiteURL: "https://www.yusufcancoskun.com/",
	})

	sitemapRec := httptest.NewRecorder()
	router.ServeHTTP(sitemapRec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	body := sitemapRec.Body.String()
	if strings.Contains(body, "//blog") || strings.Contains(body, "com//") {
		t.Fatalf("sitemap has doubled slashes: %s", body)
	}
	if !strings.Contains(body, "https://www.yusufcancoskun.com/blog") {
		t.Fatalf("sitemap body = %q, want trimmed site URL", body)
	}

	rssRec := httptest.NewRecorder()
	router.ServeHTTP(rssRec, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
	rssBody := rssRec.Body.String()
	if strings.Contains(rssBody, "com//") {
		t.Fatalf("rss has doubled slashes: %s", rssBody)
	}
}

func TestSitemapIncludesPublishedPost(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		SiteURL:           "https://www.yusufcancoskun.com",
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	createBody := bytes.NewBufferString(
		`{"slug":"hello-feed","title":"Hello & Feed","summary":"sum & more","content_md":"# hi","published":true}`,
	)
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/posts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	sitemapRec := httptest.NewRecorder()
	router.ServeHTTP(sitemapRec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if !strings.Contains(sitemapRec.Body.String(), "/blog/hello-feed") {
		t.Fatalf("sitemap body = %q, want post slug", sitemapRec.Body.String())
	}

	rssRec := httptest.NewRecorder()
	router.ServeHTTP(rssRec, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
	body := rssRec.Body.String()
	if !strings.Contains(body, "Hello &amp; Feed") || !strings.Contains(body, "sum &amp; more") {
		t.Fatalf("rss body = %q, want XML-escaped & in title/summary", body)
	}
	if !strings.Contains(body, "/blog/hello-feed") {
		t.Fatalf("rss body = %q, want published post", body)
	}
	if strings.Contains(body, "Hello & Feed") || strings.Contains(body, "sum & more") {
		t.Fatalf("rss body = %q, raw & must be escaped", body)
	}
}

func TestAdminCreatePostRejectsBadSlug(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		SiteURL:           "https://www.yusufcancoskun.com",
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	createBody := bytes.NewBufferString(
		`{"slug":"bad slug","title":"Bad","summary":"s","content_md":"# hi","published":false}`,
	)
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/posts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, want 400; body = %s", createRec.Code, createRec.Body.String())
	}
	body := createRec.Body.String()
	if !strings.Contains(body, "validation:") {
		t.Fatalf("body = %q, want validation: in error", body)
	}
	if !strings.Contains(body, "slug must be lowercase letters, digits, and single hyphens (max 100)") {
		t.Fatalf("body = %q, want slug rule message", body)
	}
}

func TestUnpublishedPostHiddenFromFeeds(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		SiteURL:           "https://www.yusufcancoskun.com",
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	createBody := bytes.NewBufferString(
		`{"slug":"draft-feed","title":"Draft Feed","summary":"s","content_md":"# hi","published":false}`,
	)
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/posts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	sitemapRec := httptest.NewRecorder()
	router.ServeHTTP(sitemapRec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if strings.Contains(sitemapRec.Body.String(), "draft-feed") {
		t.Fatalf("sitemap leaked draft: %s", sitemapRec.Body.String())
	}

	rssRec := httptest.NewRecorder()
	router.ServeHTTP(rssRec, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
	if strings.Contains(rssRec.Body.String(), "draft-feed") || strings.Contains(rssRec.Body.String(), "Draft Feed") {
		t.Fatalf("rss leaked draft: %s", rssRec.Body.String())
	}
}

func TestUnknownAPIPathReturns404Envelope(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/api/nope")
	assertEnvelope(t, rec, http.StatusNotFound, `{"data":null,"error":"not found"}`)
}

// A wrong-method request against a registered API path must return an
// enveloped 405 with an Allow header listing the valid methods.
func TestWrongMethodReturns405Envelope(t *testing.T) {
	rec := doRequest(t, http.MethodPost, "/api/health")
	assertEnvelope(t, rec, http.StatusMethodNotAllowed, `{"data":null,"error":"method not allowed"}`)
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) {
		t.Fatalf("Allow = %q, want it to contain GET", allow)
	}
}

// /api/admin/posts registers both GET and POST; a third method must list
// both in Allow, confirming the header isn't just echoing a single method.
func TestWrongMethodOnMultiMethodPathListsAllAllowedMethods(t *testing.T) {
	rec := doRequest(t, http.MethodPut, "/api/admin/posts")
	assertEnvelope(t, rec, http.StatusMethodNotAllowed, `{"data":null,"error":"method not allowed"}`)
	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, http.MethodGet) || !strings.Contains(allow, http.MethodPost) {
		t.Fatalf("Allow = %q, want it to contain both GET and POST", allow)
	}
}

func TestNonAPIPathReachesSPA(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/blog")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "spa") {
		t.Fatalf("body = %q, want SPA content", rec.Body.String())
	}
}

func TestAdminPostsWithoutCookieReturns401(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/posts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertEnvelope(t, rec, http.StatusUnauthorized, `{"data":null,"error":"unauthorized"}`)
}

func TestLoginWithEmptyConfigReturns401(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{})

	body := bytes.NewBufferString(`{"username":"admin","password":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertEnvelope(t, rec, http.StatusUnauthorized, `{"data":null,"error":"unauthorized"}`)
}

func TestLoginCookieAndAdminList(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected Set-Cookie session")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/posts", nil)
	listReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	assertEnvelope(t, listRec, http.StatusOK, `{"data":[],"error":null}`)
}

func loginWithHost(t *testing.T, router http.Handler, host string) *httptest.ResponseRecorder {
	t.Helper()
	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Host = host
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	return loginRec
}

func TestLoginCookie_publicHostIsSecure(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginRec := loginWithHost(t, router, "www.yusufcancoskun.com")

	found := sessionCookieFull(loginRec)
	if found == nil {
		t.Fatal("expected session cookie")
	}
	if found.Name != auth.SessionCookieName {
		t.Errorf("cookie name = %q, want %q", found.Name, auth.SessionCookieName)
	}
	if !found.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
	if found.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", found.SameSite)
	}
	if !found.Secure {
		t.Error("expected Secure=true for public Host")
	}
	if found.Path != "/" {
		t.Errorf("Path = %q, want /", found.Path)
	}
	if found.Domain != "" {
		t.Errorf("Domain = %q, want empty", found.Domain)
	}
	if !legacySessionCleared(loginRec) {
		t.Fatalf("expected legacy session cookie clear, Set-Cookie = %v", loginRec.Header().Values("Set-Cookie"))
	}
}

func TestLoginCookie_localhostIsSecure(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginRec := loginWithHost(t, router, "localhost")

	found := sessionCookieFull(loginRec)
	if found == nil {
		t.Fatal("expected session cookie")
	}
	if found.Name != auth.SessionCookieName {
		t.Errorf("cookie name = %q, want %q", found.Name, auth.SessionCookieName)
	}
	if !found.Secure {
		t.Error("expected Secure=true for localhost (__Host- requirement)")
	}
	if found.Path != "/" {
		t.Errorf("Path = %q, want /", found.Path)
	}
	if found.Domain != "" {
		t.Errorf("Domain = %q, want empty", found.Domain)
	}
	if !legacySessionCleared(loginRec) {
		t.Fatalf("expected legacy session cookie clear, Set-Cookie = %v", loginRec.Header().Values("Set-Cookie"))
	}
}

func TestAdminMe_legacySessionCookieRejected(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginRec := loginWithHost(t, router, "localhost")
	token := sessionCookie(loginRec)
	if token == "" {
		t.Fatal("expected session cookie")
	}

	legacyReq := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	legacyReq.Header.Set("Cookie", "session="+token)
	legacyRec := httptest.NewRecorder()
	router.ServeHTTP(legacyRec, legacyReq)
	assertEnvelope(t, legacyRec, http.StatusUnauthorized, `{"data":null,"error":"unauthorized"}`)

	hostReq := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	hostReq.Header.Set("Cookie", auth.SessionCookieName+"="+token)
	hostRec := httptest.NewRecorder()
	router.ServeHTTP(hostRec, hostReq)
	if hostRec.Code != http.StatusOK {
		t.Fatalf("me with %s status = %d, body = %s", auth.SessionCookieName, hostRec.Code, hostRec.Body.String())
	}
}

func TestAdminPreviewWithoutCookieReturns401(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{})

	body := bytes.NewBufferString(`{"content_md":"# Hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/preview", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertEnvelope(t, rec, http.StatusUnauthorized, `{"data":null,"error":"unauthorized"}`)
}

func TestAdminPreviewWithSessionReturnsHTML(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	previewBody := bytes.NewBufferString(`{"content_md":"# Hi"}`)
	previewReq := httptest.NewRequest(http.MethodPost, "/api/admin/preview", previewBody)
	previewReq.Header.Set("Content-Type", "application/json")
	previewReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	previewRec := httptest.NewRecorder()
	router.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", previewRec.Code, previewRec.Body.String())
	}
	if ct := previewRec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	got := previewRec.Body.String()
	// encoding/json escapes < to \u003c; unescaped <h1 would also be fine.
	if !strings.Contains(got, `"html"`) ||
		(!strings.Contains(got, `\u003ch1`) && !strings.Contains(got, "<h1")) {
		t.Fatalf("body = %q, want html containing h1", got)
	}
	if !strings.Contains(got, `"error":null`) {
		t.Fatalf("body = %q, want error:null envelope", got)
	}
}

func TestLogoutWithoutCookieReturns200(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertEnvelope(t, rec, http.StatusOK, `{"data":{"ok":true},"error":null}`)
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		prefix := auth.SessionCookieName + "="
		for _, h := range rec.Header().Values("Set-Cookie") {
			if strings.HasPrefix(h, prefix) &&
				(strings.Contains(h, "Max-Age=0") || strings.Contains(h, "Max-Age=-1")) {
				cleared = true
			}
		}
	}
	if !cleared {
		t.Fatalf("expected session cookie clear, Set-Cookie = %v", rec.Header().Values("Set-Cookie"))
	}
	if !legacySessionCleared(rec) {
		t.Fatalf("expected legacy session cookie clear, Set-Cookie = %v", rec.Header().Values("Set-Cookie"))
	}
}

func TestSessionBindingLoginThenMe(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
		SessionBinding:    true,
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("User-Agent", "BindingTest/1.0")
	loginReq.RemoteAddr = "127.0.0.1:4242"
	loginReq.Header.Set("CF-Connecting-IP", "198.51.100.10")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie after login")
	}

	t.Run("same hints ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.Header.Set("User-Agent", "BindingTest/1.0")
		req.RemoteAddr = "127.0.0.1:4242"
		req.Header.Set("CF-Connecting-IP", "198.51.100.99")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("me status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("spoofed UA reauth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.Header.Set("User-Agent", "Spoofed/9.0")
		req.RemoteAddr = "127.0.0.1:4242"
		req.Header.Set("CF-Connecting-IP", "198.51.100.10")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusUnauthorized, `{"data":null,"error":"reauth_required"}`)
	})
}

func TestSessionBindingSpoofedIPAfterLogin(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
		SessionBinding:    true,
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("User-Agent", "BindingTest/1.0")
	loginReq.RemoteAddr = "127.0.0.1:4242"
	loginReq.Header.Set("CF-Connecting-IP", "198.51.100.10")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	cookie := sessionCookie(loginRec)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	req.Header.Set("User-Agent", "BindingTest/1.0")
	req.RemoteAddr = "127.0.0.1:4242"
	req.Header.Set("CF-Connecting-IP", "203.0.113.1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertEnvelope(t, rec, http.StatusUnauthorized, `{"data":null,"error":"reauth_required"}`)
}

func TestSessionBindingLoginRejectsEmptyUA(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
		SessionBinding:    true,
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	// Intentionally no User-Agent — fail closed when binding is on.
	loginReq.RemoteAddr = "127.0.0.1:4242"
	loginReq.Header.Set("CF-Connecting-IP", "198.51.100.10")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusBadRequest {
		t.Fatalf("login status = %d, want 400; body = %s", loginRec.Code, loginRec.Body.String())
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("sessions = %d, want 0 (no unbound session)", n)
	}
}

func TestSessionBindingProtectedMediaMismatch(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
		SessionBinding:    true,
	}
	uploads := filepath.Join(t.TempDir(), "uploads")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}
	pages := services.NewPageService(db)
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router := NewRouter(spa, Deps{
		Posts:    services.NewPostService(db),
		Resume:   services.NewResumeService(db).WithPages(pages, media),
		Sessions: services.NewSessionService(db),
		Settings: services.NewSettingsService(db),
		Pages:    pages,
		Work:     services.NewWorkService(db),
		Studio:   services.NewStudioService(db),
		Media:    media,
		Config:   cfg,
	})

	png := []byte("\x89PNG\r\n\x1a\n")
	orphan, err := media.Create("orphan.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("orphan: %v", err)
	}

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("User-Agent", "BindingTest/1.0")
	loginReq.RemoteAddr = "127.0.0.1:4242"
	loginReq.Header.Set("CF-Connecting-IP", "198.51.100.10")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	cookie := sessionCookie(loginRec)

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(orphan.ID, 10), nil)
	req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	req.Header.Set("User-Agent", "Spoofed/9.0")
	req.RemoteAddr = "127.0.0.1:4242"
	req.Header.Set("CF-Connecting-IP", "198.51.100.10")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertEnvelope(t, rec, http.StatusUnauthorized, `{"data":null,"error":"reauth_required"}`)

	cleared := false
	for _, h := range rec.Header().Values("Set-Cookie") {
		c, err := http.ParseSetCookie(h)
		if err != nil {
			continue
		}
		if c.Name == auth.SessionCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("expected session cookie cleared, Set-Cookie = %v", rec.Header().Values("Set-Cookie"))
	}
}

func TestUnpublishedPostHiddenFromPublicAPI(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	createBody := bytes.NewBufferString(
		`{"slug":"draft-post","title":"Draft","summary":"s","content_md":"# hi","published":false}`,
	)
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/posts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/posts", nil))
	assertEnvelope(t, listRec, http.StatusOK, `{"data":[],"error":null}`)

	slugRec := httptest.NewRecorder()
	router.ServeHTTP(slugRec, httptest.NewRequest(http.MethodGet, "/api/posts/draft-post", nil))
	assertEnvelope(t, slugRec, http.StatusNotFound, `{"data":null,"error":"not found"}`)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newIntegrationRouter(t *testing.T, db *sql.DB, cfg config.Config) http.Handler {
	t.Helper()
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	uploads := filepath.Join(t.TempDir(), "uploads")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media service: %v", err)
	}
	pages := services.NewPageService(db)
	resume := services.NewResumeService(db).WithPages(pages, media)
	return NewRouter(spa, Deps{
		Posts:    services.NewPostService(db),
		Resume:   resume,
		Sessions: services.NewSessionService(db),
		Settings: services.NewSettingsService(db),
		Pages:    pages,
		Work:     services.NewWorkService(db),
		Studio:   services.NewStudioService(db),
		Media:    media,
		Config:   cfg,
	})
}

func TestCMSPublicAndAdminSettings(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	pubRec := httptest.NewRecorder()
	router.ServeHTTP(pubRec, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if pubRec.Code != http.StatusOK {
		t.Fatalf("settings status = %d", pubRec.Code)
	}
	if !strings.Contains(pubRec.Body.String(), `"nav"`) {
		t.Fatalf("settings body = %s", pubRec.Body.String())
	}

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginRec.Code)
	}
	cookie := sessionCookie(loginRec)

	putBody := bytes.NewBufferString(`{"settings":{"site_name":"CMS Test","rss_title":"CMS RSS"}}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/admin/settings", putBody)
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put settings status = %d body=%s", putRec.Code, putRec.Body.String())
	}

	workBody := bytes.NewBufferString(`{"name":"demo","one_liner":"x","body":"y","stack":["Go"],"status":"WIP","href":"https://example.com","sort_order":1}`)
	workReq := httptest.NewRequest(http.MethodPost, "/api/admin/work", workBody)
	workReq.Header.Set("Content-Type", "application/json")
	workReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	workRec := httptest.NewRecorder()
	router.ServeHTTP(workRec, workReq)
	if workRec.Code != http.StatusCreated {
		t.Fatalf("create work status = %d body=%s", workRec.Code, workRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/work", nil))
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "demo") {
		t.Fatalf("list work = %d %s", listRec.Code, listRec.Body.String())
	}

	rssRec := httptest.NewRecorder()
	router.ServeHTTP(rssRec, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
	if !strings.Contains(rssRec.Body.String(), "CMS RSS") {
		t.Fatalf("rss missing settings title: %s", rssRec.Body.String())
	}
}

func TestCMSRejectsDangerousURLs(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginRec.Code)
	}
	cookie := sessionCookie(loginRec)

	workBody := bytes.NewBufferString(`{"name":"demo","href":"javascript:alert(1)","sort_order":1}`)
	workReq := httptest.NewRequest(http.MethodPost, "/api/admin/work", workBody)
	workReq.Header.Set("Content-Type", "application/json")
	workReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	workRec := httptest.NewRecorder()
	router.ServeHTTP(workRec, workReq)
	if workRec.Code != http.StatusBadRequest {
		t.Fatalf("bad work href status = %d body=%s", workRec.Code, workRec.Body.String())
	}

	navBody := bytes.NewBufferString(`{"settings":{"nav":"[{\"label\":\"X\",\"path\":\"//evil.com\"}]"}}`)
	navReq := httptest.NewRequest(http.MethodPut, "/api/admin/settings", navBody)
	navReq.Header.Set("Content-Type", "application/json")
	navReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	navRec := httptest.NewRecorder()
	router.ServeHTTP(navRec, navReq)
	if navRec.Code != http.StatusBadRequest {
		t.Fatalf("bad nav status = %d body=%s", navRec.Code, navRec.Body.String())
	}
}

func TestAdminAPISecFetchSite(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	router := newIntegrationRouter(t, db, config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	})
	cookie := loginTestAdmin(t, router)

	t.Run("cross-site forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/posts", nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusForbidden, `{"data":null,"error":"forbidden"}`)
		body := rec.Body.String()
		if strings.Contains(body, `"data":[`) || strings.Contains(body, `"error":null`) {
			t.Fatalf("body looks like CMS list JSON: %s", body)
		}
	})

	t.Run("same-origin allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/posts", nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusOK, `{"data":[],"error":null}`)
	})

	t.Run("missing Sec-Fetch-Site allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/posts", nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusOK, `{"data":[],"error":null}`)
	})
}

func TestAdminExportCSRF(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginRec.Code)
	}
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	t.Run("POST export with session and password", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		out := rec.Body.String()
		for _, key := range []string{`"settings"`, `"pages"`, `"work"`, `"studio"`} {
			if !strings.Contains(out, key) {
				t.Fatalf("export body missing %s: %s", key, out)
			}
		}
	})

	t.Run("POST export missing password", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "password confirmation required") {
			t.Fatalf("body = %s, want password confirmation required", rec.Body.String())
		}
	})

	t.Run("POST export wrong password", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"wrong"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "invalid password") {
			t.Fatalf("body = %s, want invalid password", rec.Body.String())
		}
	})

	t.Run("POST import missing password", func(t *testing.T) {
		body := bytes.NewBufferString(`{"dump":{}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "password confirmation required") {
			t.Fatalf("body = %s, want password confirmation required", rec.Body.String())
		}
	})

	t.Run("POST import wrong password", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"wrong","dump":{}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "invalid password") {
			t.Fatalf("body = %s, want invalid password", rec.Body.String())
		}
	})

	t.Run("POST import with session and password", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","dump":{"settings":{}}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET export gone", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/export", nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatal("GET export must not succeed")
		}
		assertEnvelope(t, rec, http.StatusMethodNotAllowed, `{"data":null,"error":"method not allowed"}`)
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, http.MethodPost) {
			t.Fatalf("Allow = %q, want it to contain POST", allow)
		}
	})

	t.Run("POST export without session", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cross-site Sec-Fetch-Site forbidden", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("after logout export and import unauthorized", func(t *testing.T) {
		logoutReq := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
		logoutReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		logoutRec := httptest.NewRecorder()
		router.ServeHTTP(logoutRec, logoutReq)
		if logoutRec.Code != http.StatusOK {
			t.Fatalf("logout status = %d", logoutRec.Code)
		}

		exportBody := bytes.NewBufferString(`{"password":"testpass"}`)
		exportReq := httptest.NewRequest(http.MethodPost, "/api/admin/export", exportBody)
		exportReq.Header.Set("Content-Type", "application/json")
		exportReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		exportRec := httptest.NewRecorder()
		router.ServeHTTP(exportRec, exportReq)
		if exportRec.Code != http.StatusUnauthorized {
			t.Fatalf("export after logout status = %d, want 401; body = %s", exportRec.Code, exportRec.Body.String())
		}

		importBody := bytes.NewBufferString(`{"password":"testpass","dump":{}}`)
		importReq := httptest.NewRequest(http.MethodPost, "/api/admin/import", importBody)
		importReq.Header.Set("Content-Type", "application/json")
		importReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		importRec := httptest.NewRecorder()
		router.ServeHTTP(importRec, importReq)
		if importRec.Code != http.StatusUnauthorized {
			t.Fatalf("import after logout status = %d, want 401; body = %s", importRec.Code, importRec.Body.String())
		}
	})
}

func TestAdminImportIntegrity(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)
	cookie := loginTestAdmin(t, router)

	workBody := bytes.NewBufferString(`{"name":"keep-me","one_liner":"x","body":"","stack":[],"status":"shipped","href":"https://example.com","sort_order":1}`)
	workReq := httptest.NewRequest(http.MethodPost, "/api/admin/work", workBody)
	workReq.Header.Set("Content-Type", "application/json")
	workReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	workRec := httptest.NewRecorder()
	router.ServeHTTP(workRec, workReq)
	if workRec.Code != http.StatusCreated {
		t.Fatalf("create work status = %d, body = %s", workRec.Code, workRec.Body.String())
	}

	studioBody := bytes.NewBufferString(`{"slug":"keep-studio","title":"Keep Studio","year":"2024","medium":"print","caption":"c","sort_order":1,"published":true}`)
	studioReq := httptest.NewRequest(http.MethodPost, "/api/admin/studio", studioBody)
	studioReq.Header.Set("Content-Type", "application/json")
	studioReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	studioRec := httptest.NewRecorder()
	router.ServeHTTP(studioRec, studioReq)
	if studioRec.Code != http.StatusCreated {
		t.Fatalf("create studio status = %d, body = %s", studioRec.Code, studioRec.Body.String())
	}

	secBody := bytes.NewBufferString(`{"kind":"experience","title":"Keep Section","sort_order":1,"accordion":false}`)
	secReq := httptest.NewRequest(http.MethodPost, "/api/admin/resume/sections", secBody)
	secReq.Header.Set("Content-Type", "application/json")
	secReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	secRec := httptest.NewRecorder()
	router.ServeHTTP(secRec, secReq)
	if secRec.Code != http.StatusCreated {
		t.Fatalf("create resume section status = %d, body = %s", secRec.Code, secRec.Body.String())
	}

	t.Run("replace_work without confirm returns 400 and keeps work", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","dump":{"replace_work":true,"work":[]}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "replace confirmation required") {
			t.Fatalf("body = %s, want replace confirmation required", rec.Body.String())
		}

		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/work", nil))
		if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "keep-me") {
			t.Fatalf("work wiped without confirm; list = %d %s", listRec.Code, listRec.Body.String())
		}
	})

	t.Run("replace_studio without confirm returns 400 and keeps studio", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","dump":{"replace_studio":true,"studio":[]}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "replace confirmation required") {
			t.Fatalf("body = %s, want replace confirmation required", rec.Body.String())
		}

		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/studio", nil))
		if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "keep-studio") {
			t.Fatalf("studio wiped without confirm; list = %d %s", listRec.Code, listRec.Body.String())
		}
	})

	t.Run("replace_resume without confirm returns 400 and keeps resume", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","dump":{"replace_resume":true,"resume_sections":[],"resume_entries":[]}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "replace confirmation required") {
			t.Fatalf("body = %s, want replace confirmation required", rec.Body.String())
		}

		listReq := httptest.NewRequest(http.MethodGet, "/api/admin/resume/sections", nil)
		listReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, listReq)
		if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "Keep Section") {
			t.Fatalf("resume wiped without confirm; list = %d %s", listRec.Code, listRec.Body.String())
		}
	})

	t.Run("matching confirms and password succeed", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","confirm_replace_work":true,"confirm_replace_studio":true,"confirm_replace_resume":true,"dump":{"replace_work":true,"replace_studio":true,"replace_resume":true,"work":[{"name":"imported","one_liner":"y","body":"","stack":[],"status":"shipped","href":"https://example.com/imported","sort_order":1}]}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}

		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/work", nil))
		if listRec.Code != http.StatusOK {
			t.Fatalf("list status = %d", listRec.Code)
		}
		out := listRec.Body.String()
		if !strings.Contains(out, "imported") {
			t.Fatalf("list missing imported work: %s", out)
		}
		if strings.Contains(out, "keep-me") {
			t.Fatalf("list still has wiped work: %s", out)
		}
	})

	t.Run("javascript href in dump returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","confirm_replace_work":true,"confirm_replace_studio":true,"confirm_replace_resume":true,"dump":{"work":[{"name":"evil","one_liner":"","body":"","stack":[],"status":"","href":"javascript:alert(1)","sort_order":1}]}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "disallowed url scheme") {
			t.Fatalf("body = %s, want disallowed url scheme", rec.Body.String())
		}
	})

	t.Run("unknown envelope field returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","evil":true,"dump":{}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusBadRequest, `{"data":null,"error":"invalid json"}`)
	})

	t.Run("unknown dump field returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","dump":{"not_a_field":1}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusBadRequest, `{"data":null,"error":"invalid json"}`)
	})

	t.Run("nested unknown dump field returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password":"testpass","dump":{"work":[{"name":"x","one_liner":"","body":"","stack":[],"status":"","href":"https://example.com","sort_order":1,"evil":1}]}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusBadRequest, `{"data":null,"error":"invalid json"}`)
	})

	t.Run("export then import with confirms succeeds", func(t *testing.T) {
		exportBody := bytes.NewBufferString(`{"password":"testpass"}`)
		exportReq := httptest.NewRequest(http.MethodPost, "/api/admin/export", exportBody)
		exportReq.Header.Set("Content-Type", "application/json")
		exportReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		exportRec := httptest.NewRecorder()
		router.ServeHTTP(exportRec, exportReq)
		if exportRec.Code != http.StatusOK {
			t.Fatalf("export status = %d, body = %s", exportRec.Code, exportRec.Body.String())
		}
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(exportRec.Body.Bytes(), &env); err != nil || len(env.Data) == 0 {
			t.Fatalf("unwrap export: %v body=%s", err, exportRec.Body.String())
		}
		payload := `{"password":"testpass","confirm_replace_work":true,"confirm_replace_studio":true,"confirm_replace_resume":true,"dump":` + string(env.Data) + `}`
		importReq := httptest.NewRequest(http.MethodPost, "/api/admin/import", strings.NewReader(payload))
		importReq.Header.Set("Content-Type", "application/json")
		importReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		importRec := httptest.NewRecorder()
		router.ServeHTTP(importRec, importReq)
		if importRec.Code != http.StatusOK {
			t.Fatalf("re-import status = %d, body = %s", importRec.Code, importRec.Body.String())
		}
	})
}

func sessionCookie(rec *httptest.ResponseRecorder) string {
	if c := sessionCookieFull(rec); c != nil {
		return c.Value
	}
	prefix := auth.SessionCookieName + "="
	for _, h := range rec.Header().Values("Set-Cookie") {
		if strings.HasPrefix(h, prefix) {
			part := strings.SplitN(h, ";", 2)[0]
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

// sessionCookieFull returns the parsed session cookie (with flags such as
// HttpOnly/SameSite/Secure) from a response, or nil if not present.
func sessionCookieFull(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	return nil
}

func legacySessionCleared(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" && c.MaxAge < 0 {
			return true
		}
	}
	for _, h := range rec.Header().Values("Set-Cookie") {
		if strings.HasPrefix(h, "session=") &&
			(strings.Contains(h, "Max-Age=0") || strings.Contains(h, "Max-Age=-1")) {
			return true
		}
	}
	return false
}

func TestMediaAccessControl(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	uploads := filepath.Join(t.TempDir(), "uploads")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}
	pages := services.NewPageService(db)
	studio := services.NewStudioService(db)
	posts := services.NewPostService(db)
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router := NewRouter(spa, Deps{
		Posts:    posts,
		Resume:   services.NewResumeService(db).WithPages(pages, media),
		Sessions: services.NewSessionService(db),
		Settings: services.NewSettingsService(db),
		Pages:    pages,
		Work:     services.NewWorkService(db),
		Studio:   studio,
		Media:    media,
		Config:   cfg,
	})

	png := []byte("\x89PNG\r\n\x1a\n")
	orphan, err := media.Create("orphan.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("orphan: %v", err)
	}
	draftAsset, err := media.Create("draft.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("draft asset: %v", err)
	}
	_, err = studio.Create(services.StudioInput{
		Slug: "draft-still", Title: "Draft", ImageMediaID: &draftAsset.ID, Published: false,
	})
	if err != nil {
		t.Fatalf("draft studio: %v", err)
	}
	pubAsset, err := media.Create("pub.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("pub asset: %v", err)
	}
	_, err = studio.Create(services.StudioInput{
		Slug: "pub-still", Title: "Pub", ImageMediaID: &pubAsset.ID, Published: true,
	})
	if err != nil {
		t.Fatalf("pub studio: %v", err)
	}
	evilAsset, err := media.Create("evil\"\r\n../../inject.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("evil asset: %v", err)
	}
	_, err = studio.Create(services.StudioInput{
		Slug: "evil-still", Title: "Evil", ImageMediaID: &evilAsset.ID, Published: true,
	})
	if err != nil {
		t.Fatalf("evil studio: %v", err)
	}
	pdf := []byte("%PDF-1.4 resume")
	resumePDF, err := media.Create("cv.pdf", "application/pdf", bytes.NewReader(pdf), int64(len(pdf)))
	if err != nil {
		t.Fatalf("resume pdf: %v", err)
	}
	_, err = pages.Upsert("resume", services.PageInput{
		Title: "Resume",
		BodyJSON: `{"eyebrow":"CV","headline":"Resume","blurb":"Hi","pdf_media_id":` +
			strconv.FormatInt(resumePDF.ID, 10) + `}`,
	})
	if err != nil {
		t.Fatalf("resume page: %v", err)
	}

	postAsset, err := media.Create("post.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("post asset: %v", err)
	}
	_, err = posts.Create(services.PostInput{
		Slug: "pub-media-post", Title: "Pub Media", Summary: "s",
		ContentMD: "![x](/media/" + strconv.FormatInt(postAsset.ID, 10) + ")",
		Published: true,
	})
	if err != nil {
		t.Fatalf("published post: %v", err)
	}
	draftPostAsset, err := media.Create("draft-post.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("draft post asset: %v", err)
	}
	_, err = posts.Create(services.PostInput{
		Slug: "draft-media-post", Title: "Draft Media", Summary: "s",
		ContentMD: "![x](/media/" + strconv.FormatInt(draftPostAsset.ID, 10) + ")",
		Published: false,
	})
	if err != nil {
		t.Fatalf("draft post: %v", err)
	}

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginRec.Code)
	}
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	assertAnonMediaDenied := func(t *testing.T, id int64) {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(id, 10), nil))
		if rec.Code == http.StatusUnauthorized {
			t.Fatal("must not leak existence via 401")
		}
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (not 401)", rec.Code)
		}
		cc := rec.Header().Get("Cache-Control")
		if !strings.Contains(cc, "no-store") {
			t.Fatalf("Cache-Control = %q, want no-store on denial 404", cc)
		}
	}

	t.Run("anonymous orphan 404", func(t *testing.T) {
		assertAnonMediaDenied(t, orphan.ID)
	})

	t.Run("anonymous draft studio 404", func(t *testing.T) {
		assertAnonMediaDenied(t, draftAsset.ID)
	})

	t.Run("anonymous draft post ref 404", func(t *testing.T) {
		assertAnonMediaDenied(t, draftPostAsset.ID)
	})

	t.Run("anonymous published studio 200 public cache", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(pubAsset.ID, 10), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		cc := rec.Header().Get("Cache-Control")
		if !strings.Contains(cc, "public") || !strings.Contains(cc, "max-age=300") {
			t.Fatalf("Cache-Control = %q, want public max-age=300", cc)
		}
		wantCD := `inline; filename="pub.png"`
		if cd := rec.Header().Get("Content-Disposition"); cd != wantCD {
			t.Fatalf("Content-Disposition = %q, want %q", cd, wantCD)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("X-Content-Type-Options = %q, want nosniff", rec.Header().Get("X-Content-Type-Options"))
		}
		if !bytes.Equal(rec.Body.Bytes(), png) {
			t.Fatalf("body = %q", rec.Body.Bytes())
		}
	})

	t.Run("anonymous evil filename sanitized disposition", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(evilAsset.ID, 10), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		wantCD := `inline; filename="inject.png"`
		cd := rec.Header().Get("Content-Disposition")
		if cd != wantCD {
			t.Fatalf("Content-Disposition = %q, want %q", cd, wantCD)
		}
		if strings.Contains(cd, "\r") || strings.Contains(cd, "\n") || strings.Contains(cd, "..") {
			t.Fatalf("Content-Disposition still unsafe: %q", cd)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("X-Content-Type-Options = %q, want nosniff", rec.Header().Get("X-Content-Type-Options"))
		}
	})

	t.Run("anonymous resume pdf 200 public cache", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(resumePDF.ID, 10), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		cc := rec.Header().Get("Cache-Control")
		if !strings.Contains(cc, "public") || !strings.Contains(cc, "max-age=300") {
			t.Fatalf("Cache-Control = %q, want public max-age=300", cc)
		}
		wantCD := `attachment; filename="cv.pdf"`
		if cd := rec.Header().Get("Content-Disposition"); cd != wantCD {
			t.Fatalf("Content-Disposition = %q, want %q", cd, wantCD)
		}
		if rec.Header().Get("Content-Type") != "application/pdf" {
			t.Fatalf("Content-Type = %q, want application/pdf", rec.Header().Get("Content-Type"))
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("X-Content-Type-Options = %q, want nosniff", rec.Header().Get("X-Content-Type-Options"))
		}
	})

	t.Run("anonymous published post ref 200 public cache", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(postAsset.ID, 10), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		cc := rec.Header().Get("Cache-Control")
		if !strings.Contains(cc, "public") || !strings.Contains(cc, "max-age=300") {
			t.Fatalf("Cache-Control = %q, want public max-age=300", cc)
		}
	})

	t.Run("admin session draft media 200 private", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(draftAsset.ID, 10), nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		cc := rec.Header().Get("Cache-Control")
		if !strings.Contains(cc, "private") || !strings.Contains(cc, "no-store") {
			t.Fatalf("Cache-Control = %q, want private, no-store", cc)
		}
	})

	t.Run("admin list upload delete", func(t *testing.T) {
		listReq := httptest.NewRequest(http.MethodGet, "/api/admin/media", nil)
		listReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, listReq)
		if listRec.Code != http.StatusOK {
			t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
		}
		if !strings.Contains(listRec.Body.String(), "orphan.png") {
			t.Fatalf("list missing orphan: %s", listRec.Body.String())
		}

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		part, err := mw.CreateFormFile("file", "upload.png")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := part.Write(png); err != nil {
			t.Fatalf("write part: %v", err)
		}
		if err := mw.Close(); err != nil {
			t.Fatalf("close multipart: %v", err)
		}
		upReq := httptest.NewRequest(http.MethodPost, "/api/admin/media", &buf)
		upReq.Header.Set("Content-Type", mw.FormDataContentType())
		upReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		upRec := httptest.NewRecorder()
		router.ServeHTTP(upRec, upReq)
		if upRec.Code != http.StatusCreated {
			t.Fatalf("upload status = %d body=%s", upRec.Code, upRec.Body.String())
		}
		uploadedID := int64(0)
		items, err := media.List()
		if err != nil {
			t.Fatalf("list after upload: %v", err)
		}
		for _, item := range items {
			if item.OriginalName == "upload.png" {
				uploadedID = item.ID
				break
			}
		}
		if uploadedID == 0 {
			t.Fatal("uploaded asset not found")
		}

		delReq := httptest.NewRequest(http.MethodDelete, "/api/admin/media/"+strconv.FormatInt(uploadedID, 10), nil)
		delReq.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		delRec := httptest.NewRecorder()
		router.ServeHTTP(delRec, delReq)
		if delRec.Code != http.StatusOK {
			t.Fatalf("delete status = %d body=%s", delRec.Code, delRec.Body.String())
		}
		if _, err := media.GetByID(uploadedID); !errors.Is(err, services.ErrNotFound) {
			t.Fatalf("after delete GetByID err = %v, want ErrNotFound", err)
		}
	})
}

func captureLogOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func assertLogNoSecrets(t *testing.T, logOutput, secret string) {
	t.Helper()
	if strings.Contains(logOutput, secret) {
		t.Fatalf("log must not contain %q; got %q", secret, logOutput)
	}
}

func assertLogNoEvent(t *testing.T, logOutput, event string) {
	t.Helper()
	if strings.Contains(logOutput, "security event="+event) {
		t.Fatalf("log must not contain %q event; got %q", event, logOutput)
	}
}

func TestSecurityEventLogging(t *testing.T) {
	securitylog.Default = securitylog.NewAlertTracker()
	t.Cleanup(func() { securitylog.Default = securitylog.NewAlertTracker() })

	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}
	router := newIntegrationRouter(t, db, cfg)

	t.Run("failed login emits login_failure without password", func(t *testing.T) {
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"username":"admin","password":"leaked-secret"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.50:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "security event=login_failure") {
			t.Fatalf("log = %q, want login_failure event", logOut)
		}
		if !strings.Contains(logOut, "ip=203.0.113.50") {
			t.Fatalf("log = %q, want ip=203.0.113.50", logOut)
		}
		if !strings.Contains(logOut, "route=/api/admin/login") {
			t.Fatalf("log = %q, want route=/api/admin/login", logOut)
		}
		if strings.Contains(logOut, "alert=1") {
			t.Fatalf("single login_failure must not alert; got %q", logOut)
		}
		assertLogNoSecrets(t, logOut, "leaked-secret")
		if strings.Contains(logOut, "username=") {
			t.Fatalf("log must not contain username field; got %q", logOut)
		}
	})

	t.Run("successful login emits login_success without alert", func(t *testing.T) {
		securitylog.Default = securitylog.NewAlertTracker()
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		logOut := buf.String()
		if strings.Contains(logOut, "security event=login_failure") {
			t.Fatalf("successful login must not log login_failure; got %q", logOut)
		}
		if !strings.Contains(logOut, "security event=login_success") {
			t.Fatalf("log = %q, want login_success event", logOut)
		}
		if !strings.Contains(logOut, "route=/api/admin/login") {
			t.Fatalf("log = %q, want route=/api/admin/login", logOut)
		}
		if strings.Contains(logOut, "alert=1") {
			t.Fatalf("login_success without prior failures must not alert; got %q", logOut)
		}
		assertLogNoSecrets(t, logOut, "testpass")
		sessionToken := sessionCookie(rec)
		if sessionToken == "" {
			t.Fatal("expected session cookie")
		}
		assertLogNoSecrets(t, logOut, sessionToken)
	})

	t.Run("login failure burst alerts once", func(t *testing.T) {
		securitylog.Default = securitylog.NewAlertTracker()
		buf := captureLogOutput(t)
		const ip = "203.0.113.60"
		for i := 0; i < 5; i++ {
			body := bytes.NewBufferString(`{"username":"admin","password":"wrong-pass"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = ip + ":12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d: status = %d, want 401", i+1, rec.Code)
			}
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "alert=1") || !strings.Contains(logOut, "reason=login_failure_burst") {
			t.Fatalf("5th failure should alert: %q", logOut)
		}
		assertLogNoSecrets(t, logOut, "wrong-pass")

		buf.Reset()
		body := bytes.NewBufferString(`{"username":"admin","password":"wrong-pass"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if strings.Contains(buf.String(), "alert=1") {
			t.Fatalf("6th failure must not re-alert: %q", buf.String())
		}
	})

	t.Run("login success after failure burst alerts", func(t *testing.T) {
		securitylog.Default = securitylog.NewAlertTracker()
		const ip = "203.0.113.61"
		for i := 0; i < 5; i++ {
			body := bytes.NewBufferString(`{"username":"admin","password":"wrong-pass"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = ip + ":12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d: status = %d, want 401", i+1, rec.Code)
			}
		}
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "security event=login_success") {
			t.Fatalf("want login_success: %q", logOut)
		}
		if !strings.Contains(logOut, "alert=1") ||
			!strings.Contains(logOut, "reason=login_success_after_failures") ||
			!strings.Contains(logOut, "failures=5") {
			t.Fatalf("success after burst should alert: %q", logOut)
		}
		assertLogNoSecrets(t, logOut, "testpass")
	})

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginRec.Code)
	}
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	t.Run("export emits event with alert without dump body", func(t *testing.T) {
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"password":"testpass"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.RemoteAddr = "203.0.113.51:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "security event=export") {
			t.Fatalf("log = %q, want export event", logOut)
		}
		if !strings.Contains(logOut, "route=/api/admin/export") {
			t.Fatalf("log = %q, want route=/api/admin/export", logOut)
		}
		if !strings.Contains(logOut, "alert=1") || !strings.Contains(logOut, "reason=export") {
			t.Fatalf("log = %q, want alert=1 reason=export", logOut)
		}
		assertLogNoSecrets(t, logOut, "testpass")
		assertLogNoSecrets(t, logOut, cookie)
		if strings.Contains(logOut, `"settings"`) || strings.Contains(logOut, `"pages"`) {
			t.Fatalf("log must not contain dump body; got %q", logOut)
		}
	})

	t.Run("export wrong password does not emit export event", func(t *testing.T) {
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"password":"wrong"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		assertLogNoEvent(t, buf.String(), "export")
	})

	t.Run("import with replace emits alert", func(t *testing.T) {
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"password":"testpass","confirm_replace_work":true,"confirm_replace_studio":true,"confirm_replace_resume":true,"dump":{"settings":{"site_title":"Logged Title"},"replace_work":true,"replace_studio":true,"replace_resume":true}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.RemoteAddr = "203.0.113.52:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "security event=import") {
			t.Fatalf("log = %q, want import event", logOut)
		}
		if !strings.Contains(logOut, "route=/api/admin/import") {
			t.Fatalf("log = %q, want route=/api/admin/import", logOut)
		}
		for _, field := range []string{
			"settings_upserted=", "pages_upserted=", "work_created=",
			"studio_created=", "sections_created=", "entries_created=",
			"replace_work=true", "replace_studio=true", "replace_resume=true",
			"alert=1", "reason=import_replace",
		} {
			if !strings.Contains(logOut, field) {
				t.Fatalf("log = %q, want %q", logOut, field)
			}
		}
		assertLogNoSecrets(t, logOut, "testpass")
		assertLogNoSecrets(t, logOut, cookie)
		assertLogNoSecrets(t, logOut, "Logged Title")
		assertLogNoSecrets(t, logOut, "site_title")
	})

	t.Run("import without replace emits event without alert", func(t *testing.T) {
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"password":"testpass","dump":{"settings":{"site_title":"No Replace Title"}}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.RemoteAddr = "203.0.113.54:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "security event=import") {
			t.Fatalf("log = %q, want import event", logOut)
		}
		if !strings.Contains(logOut, "route=/api/admin/import") {
			t.Fatalf("log = %q, want route=/api/admin/import", logOut)
		}
		if strings.Contains(logOut, "alert=1") {
			t.Fatalf("non-replace import must not alert; got %q", logOut)
		}
		assertLogNoSecrets(t, logOut, "No Replace Title")
	})

	t.Run("import wrong password does not emit import event", func(t *testing.T) {
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`{"password":"wrong","dump":{}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		assertLogNoEvent(t, buf.String(), "import")
	})

	t.Run("import invalid json does not emit import event", func(t *testing.T) {
		buf := captureLogOutput(t)
		body := bytes.NewBufferString(`not-json`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		assertLogNoEvent(t, buf.String(), "import")
	})

	t.Run("media upload emits media_upload without alert", func(t *testing.T) {
		securitylog.Default = securitylog.NewAlertTracker()
		buf := captureLogOutput(t)
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("file", "logged-upload.png")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := part.Write([]byte("\x89PNG\r\n\x1a\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := mw.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/admin/media", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.RemoteAddr = "203.0.113.55:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "security event=media_upload") {
			t.Fatalf("log = %q, want media_upload event", logOut)
		}
		if !strings.Contains(logOut, "route=/api/admin/media") {
			t.Fatalf("log = %q, want route=/api/admin/media", logOut)
		}
		if strings.Contains(logOut, "alert=1") {
			t.Fatalf("single media_upload must not alert; got %q", logOut)
		}
		assertLogNoSecrets(t, logOut, cookie)
	})

	t.Run("media upload spike alerts once", func(t *testing.T) {
		securitylog.Default = securitylog.NewAlertTracker()
		buf := captureLogOutput(t)
		png := []byte("\x89PNG\r\n\x1a\n")
		const ip = "203.0.113.56"
		for i := 0; i < 15; i++ {
			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			part, err := mw.CreateFormFile("file", fmt.Sprintf("spike-%d.png", i))
			if err != nil {
				t.Fatalf("form file: %v", err)
			}
			if _, err := part.Write(png); err != nil {
				t.Fatalf("write: %v", err)
			}
			if err := mw.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/admin/media", &body)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
			req.RemoteAddr = ip + ":12345"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated {
				t.Fatalf("upload %d: status = %d, body = %s", i+1, rec.Code, rec.Body.String())
			}
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "alert=1") ||
			!strings.Contains(logOut, "reason=media_upload_spike") ||
			!strings.Contains(logOut, "count=15") {
			t.Fatalf("15th upload should alert: %q", logOut)
		}

		buf.Reset()
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("file", "spike-extra.png")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := part.Write(png); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := mw.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/admin/media", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("extra upload status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(buf.String(), "alert=1") {
			t.Fatalf("16th upload must not re-alert: %q", buf.String())
		}
	})

	t.Run("media delete emits event with id", func(t *testing.T) {
		uploads := filepath.Join(t.TempDir(), "uploads-delete")
		mediaSvc, err := services.NewMediaService(db, uploads)
		if err != nil {
			t.Fatalf("media: %v", err)
		}
		png := []byte("\x89PNG\r\n\x1a\n")
		asset, err := mediaSvc.Create("delete-me.png", "image/png", bytes.NewReader(png), int64(len(png)))
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		buf := captureLogOutput(t)
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/media/"+strconv.FormatInt(asset.ID, 10), nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		req.RemoteAddr = "203.0.113.53:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("delete status = %d, body = %s", rec.Code, rec.Body.String())
		}
		logOut := buf.String()
		wantID := "id=" + strconv.FormatInt(asset.ID, 10)
		if !strings.Contains(logOut, "security event=media_delete") {
			t.Fatalf("log = %q, want media_delete event", logOut)
		}
		if !strings.Contains(logOut, wantID) {
			t.Fatalf("log = %q, want %q", logOut, wantID)
		}
		if !strings.Contains(logOut, "route=") {
			t.Fatalf("log = %q, want route=", logOut)
		}
		assertLogNoSecrets(t, logOut, cookie)
	})

	t.Run("delete missing media does not emit media_delete event", func(t *testing.T) {
		buf := captureLogOutput(t)
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/media/999999", nil)
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("delete status = %d, want 404; body = %s", rec.Code, rec.Body.String())
		}
		assertLogNoEvent(t, buf.String(), "media_delete")
	})
}

func loginTestAdmin(t *testing.T, router http.Handler) string {
	t.Helper()
	loginBody := bytes.NewBufferString(`{"username":"admin","password":"testpass"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	cookie := sessionCookie(loginRec)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}
	return cookie
}

func assertSafeAPIErrorBody(t *testing.T, body string) {
	t.Helper()
	for _, leak := range []string{
		"goroutine", ".go:", "/home/", "/tmp/", "uploads/",
		"sqlite", "CONSTRAINT", "SELECT", "INSERT",
	} {
		if strings.Contains(body, leak) {
			t.Fatalf("body must not contain path/stack leak %q; got %q", leak, body)
		}
	}
}

func TestOversizedJSONBodyReturns413(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	router := newIntegrationRouter(t, db, config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	})
	cookie := loginTestAdmin(t, router)

	payload := `{"slug":"` + strings.Repeat("a", 1<<20) + `","title":"t","summary":"s","content_md":"c","published":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/posts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assertEnvelope(t, rec, http.StatusRequestEntityTooLarge, `{"data":null,"error":"request body too large"}`)
	assertSafeAPIErrorBody(t, rec.Body.String())
}

func TestOversizedImportBodyReturns413(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	router := newIntegrationRouter(t, db, config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	})
	cookie := loginTestAdmin(t, router)

	body := strings.NewReader(strings.Repeat("x", 8<<20+1))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/import", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assertEnvelope(t, rec, http.StatusRequestEntityTooLarge, `{"data":null,"error":"request body too large"}`)
	assertSafeAPIErrorBody(t, rec.Body.String())
}

type fillReader byte

func (b fillReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}

func TestOversizedMultipartUploadSafeError(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	router := newIntegrationRouter(t, db, config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	})
	cookie := loginTestAdmin(t, router)

	// Valid multipart with a file larger than MaxBytesReader so ParseMultipartForm
	// hits *http.MaxBytesError (413), not a soft parse failure (400).
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()
	go func() {
		defer func() { _ = pw.Close() }()
		part, err := mw.CreateFormFile("file", "huge.bin")
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_, copyErr := io.Copy(part, io.LimitReader(fillReader('x'), int64(services.MaxUploadBytes+1<<20+1)))
		closeErr := mw.Close()
		if copyErr != nil {
			_ = pw.CloseWithError(copyErr)
			return
		}
		if closeErr != nil {
			_ = pw.CloseWithError(closeErr)
		}
	}()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/media", pr)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assertEnvelope(t, rec, http.StatusRequestEntityTooLarge, `{"data":null,"error":"request body too large"}`)
	assertSafeAPIErrorBody(t, rec.Body.String())
}

func TestMalformedMultipartOrMissingFileReturns400(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	router := newIntegrationRouter(t, db, config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	})
	cookie := loginTestAdmin(t, router)

	t.Run("missing file field", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if err := mw.WriteField("name", "no-file"); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
		if err := mw.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/admin/media", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertEnvelope(t, rec, http.StatusBadRequest, `{"data":null,"error":"file field required"}`)
		assertSafeAPIErrorBody(t, rec.Body.String())
	})

	t.Run("malformed multipart", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/media", strings.NewReader("not-a-multipart-body"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=----bound")
		req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		got := rec.Body.String()
		if !strings.Contains(got, "invalid multipart form or file too large") &&
			!strings.Contains(got, "file field required") {
			t.Fatalf("body = %q, want multipart/file error", got)
		}
		assertSafeAPIErrorBody(t, got)
	})
}

func TestDBFailureReturnsGenericInternalError(t *testing.T) {
	db := openTestDB(t)
	router := newIntegrationRouter(t, db, config.Config{})
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts", nil))
	assertEnvelope(t, rec, http.StatusInternalServerError, `{"data":null,"error":"internal error"}`)
	assertSafeAPIErrorBody(t, rec.Body.String())
}

func TestInvalidPostIDReturns400(t *testing.T) {
	db := openTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	router := newIntegrationRouter(t, db, config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	})
	cookie := loginTestAdmin(t, router)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/posts/abc", nil)
	req.Header.Set("Cookie", auth.SessionCookieName+"="+cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertEnvelope(t, rec, http.StatusBadRequest, `{"data":null,"error":"invalid id"}`)
}

func TestUnknownAdminPathWithoutCookieReturns404(t *testing.T) {
	rec := doRequest(t, http.MethodPost, "/api/admin/nonexistent")
	assertEnvelope(t, rec, http.StatusNotFound, `{"data":null,"error":"not found"}`)
}
