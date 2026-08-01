package handlers

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yccoskun/website/internal/config"
	"github.com/yccoskun/website/internal/database"
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
		`{"slug":"hello-feed","title":"Hello Feed","summary":"sum","content_md":"# hi","published":true}`,
	)
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/posts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Cookie", "session="+cookie)
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
	if !strings.Contains(body, "Hello Feed") || !strings.Contains(body, "/blog/hello-feed") {
		t.Fatalf("rss body = %q, want published post", body)
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
	createReq.Header.Set("Cookie", "session="+cookie)
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

// Wrong-method requests land on the /api/ catch-all (the mux never emits a
// 405 there — see the NewRouter doc comment) and must stay enveloped JSON.
func TestWrongMethodReturns404Envelope(t *testing.T) {
	rec := doRequest(t, http.MethodPost, "/api/health")
	assertEnvelope(t, rec, http.StatusNotFound, `{"data":null,"error":"not found"}`)
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
	listReq.Header.Set("Cookie", "session="+cookie)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	assertEnvelope(t, listRec, http.StatusOK, `{"data":[],"error":null}`)
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
	previewReq.Header.Set("Cookie", "session="+cookie)
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
		if c.Name == "session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		for _, h := range rec.Header().Values("Set-Cookie") {
			if strings.HasPrefix(h, "session=") &&
				(strings.Contains(h, "Max-Age=0") || strings.Contains(h, "Max-Age=-1")) {
				cleared = true
			}
		}
	}
	if !cleared {
		t.Fatalf("expected session cookie clear, Set-Cookie = %v", rec.Header().Values("Set-Cookie"))
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
	createReq.Header.Set("Cookie", "session="+cookie)
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
	putReq.Header.Set("Cookie", "session="+cookie)
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put settings status = %d body=%s", putRec.Code, putRec.Body.String())
	}

	workBody := bytes.NewBufferString(`{"name":"demo","one_liner":"x","body":"y","stack":["Go"],"status":"WIP","href":"https://example.com","sort_order":1}`)
	workReq := httptest.NewRequest(http.MethodPost, "/api/admin/work", workBody)
	workReq.Header.Set("Content-Type", "application/json")
	workReq.Header.Set("Cookie", "session="+cookie)
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

	t.Run("POST export with session", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "session="+cookie)
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

	t.Run("GET export gone", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/export", nil)
		req.Header.Set("Cookie", "session="+cookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatal("GET export must not succeed")
		}
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST export without session", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cross-site Sec-Fetch-Site forbidden", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/export", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "session="+cookie)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
	})
}

func sessionCookie(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c.Value
		}
	}
	for _, h := range rec.Header().Values("Set-Cookie") {
		if strings.HasPrefix(h, "session=") {
			part := strings.SplitN(h, ";", 2)[0]
			return strings.TrimPrefix(part, "session=")
		}
	}
	return ""
}
