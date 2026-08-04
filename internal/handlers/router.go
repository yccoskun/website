package handlers

import (
	"net/http"
	"strings"

	"github.com/yccoskun/website/internal/middleware"
	"github.com/yccoskun/website/internal/response"
)

// NewRouter builds the full HTTP handler. Every /api/* route is registered on
// a nested ServeMux mounted at "/api/" behind rewriteAPIErrors, which
// guarantees /api/* always responds with the JSON envelope, never SPA HTML:
//   - a registered path hit with the wrong HTTP method returns 405 with an
//     Allow header listing the valid methods for that path
//   - an unregistered path returns 404
//
// Both cases are rewritten from the stdlib mux's plain-text body into the
// standard {"data":null,"error":...} envelope by rewriteAPIErrors.
func NewRouter(spa http.Handler, deps Deps) http.Handler {
	mux := http.NewServeMux()
	api := http.NewServeMux()

	api.HandleFunc("GET /api/health", Health)

	api.HandleFunc("GET /api/posts", deps.ListPublishedPosts)
	api.HandleFunc("GET /api/posts/{slug}", deps.GetPublishedPost)
	api.HandleFunc("GET /api/resume", deps.GetResume)
	api.HandleFunc("GET /api/settings", deps.GetSettings)
	api.HandleFunc("GET /api/pages/{slug}", deps.GetPage)
	api.HandleFunc("GET /api/work", deps.ListWork)
	api.HandleFunc("GET /api/studio", deps.ListStudio)
	mux.HandleFunc("GET /media/{id}", deps.ServeMedia)

	trust := middleware.NewProxyTrust(deps.Config.TrustedProxies)
	loginLimiter := middleware.NewLoginRateLimiter(trust)
	api.Handle("POST /api/admin/login", loginLimiter.Middleware(http.HandlerFunc(deps.AdminLogin)))

	requireAuth := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireSession(deps.Sessions, deps.Config.SessionBinding, trust, h)
	}

	api.Handle("POST /api/admin/logout", http.HandlerFunc(deps.AdminLogout))
	api.Handle("GET /api/admin/me", requireAuth(deps.AdminMe))

	api.Handle("POST /api/admin/preview", requireAuth(deps.AdminPreview))
	api.Handle("POST /api/admin/import", requireAuth(deps.AdminImport))
	api.Handle("POST /api/admin/export", requireAuth(deps.AdminExport))

	api.Handle("GET /api/admin/posts", requireAuth(deps.AdminListPosts))
	api.Handle("POST /api/admin/posts", requireAuth(deps.AdminCreatePost))
	api.Handle("GET /api/admin/posts/{id}", requireAuth(deps.AdminGetPost))
	api.Handle("PUT /api/admin/posts/{id}", requireAuth(deps.AdminUpdatePost))
	api.Handle("DELETE /api/admin/posts/{id}", requireAuth(deps.AdminDeletePost))

	api.Handle("GET /api/admin/resume/entries", requireAuth(deps.AdminListResumeEntries))
	api.Handle("POST /api/admin/resume/entries", requireAuth(deps.AdminCreateResumeEntry))
	api.Handle("PUT /api/admin/resume/entries/{id}", requireAuth(deps.AdminUpdateResumeEntry))
	api.Handle("DELETE /api/admin/resume/entries/{id}", requireAuth(deps.AdminDeleteResumeEntry))

	api.Handle("GET /api/admin/resume/sections", requireAuth(deps.AdminListResumeSections))
	api.Handle("POST /api/admin/resume/sections", requireAuth(deps.AdminCreateResumeSection))
	api.Handle("PUT /api/admin/resume/sections/{id}", requireAuth(deps.AdminUpdateResumeSection))
	api.Handle("DELETE /api/admin/resume/sections/{id}", requireAuth(deps.AdminDeleteResumeSection))

	api.Handle("GET /api/admin/settings", requireAuth(deps.AdminGetSettings))
	api.Handle("PUT /api/admin/settings", requireAuth(deps.AdminPutSettings))

	api.Handle("GET /api/admin/pages", requireAuth(deps.AdminListPages))
	api.Handle("GET /api/admin/pages/{slug}", requireAuth(deps.AdminGetPage))
	api.Handle("PUT /api/admin/pages/{slug}", requireAuth(deps.AdminPutPage))

	api.Handle("GET /api/admin/work", requireAuth(deps.AdminListWork))
	api.Handle("POST /api/admin/work", requireAuth(deps.AdminCreateWork))
	api.Handle("PUT /api/admin/work/{id}", requireAuth(deps.AdminUpdateWork))
	api.Handle("DELETE /api/admin/work/{id}", requireAuth(deps.AdminDeleteWork))

	api.Handle("GET /api/admin/studio", requireAuth(deps.AdminListStudio))
	api.Handle("POST /api/admin/studio", requireAuth(deps.AdminCreateStudio))
	api.Handle("PUT /api/admin/studio/{id}", requireAuth(deps.AdminUpdateStudio))
	api.Handle("DELETE /api/admin/studio/{id}", requireAuth(deps.AdminDeleteStudio))

	api.Handle("GET /api/admin/media", requireAuth(deps.AdminListMedia))
	api.Handle("POST /api/admin/media", requireAuth(deps.AdminUploadMedia))
	api.Handle("DELETE /api/admin/media/{id}", requireAuth(deps.AdminDeleteMedia))

	mux.HandleFunc("GET /robots.txt", deps.Robots)
	mux.HandleFunc("GET /sitemap.xml", deps.Sitemap)
	mux.HandleFunc("GET /rss.xml", deps.RSS)

	mux.Handle("/api/", rewriteAPIErrors(api))
	mux.Handle("/", spa)
	return middleware.SecurityHeaders(middleware.Recover(mux))
}

// rewriteAPIErrors wraps an API ServeMux so its built-in plain-text 404 and
// 405 responses become the standard JSON error envelope instead. Successful
// handler responses pass through untouched (status and body are forwarded
// directly, with no buffering).
func rewriteAPIErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aw := &apiErrorWriter{ResponseWriter: w}
		next.ServeHTTP(aw, r)
		aw.finish()
	})
}

// apiErrorWriter intercepts WriteHeader(404|405) calls that still look like
// the stdlib ServeMux's own fallback response (unmatched path, or wrong
// method on a matched path) and defers them: the plain-text status is not
// sent to the client yet. The first subsequent Write (or, if the mux never
// writes a body, finish once the handler returns) drops the stdlib body,
// strips its Content-Type, keeps any Allow header the mux set for 405s, and
// emits the JSON envelope.
//
// Handler-emitted 404s (e.g. mapServiceError / response.Error) already set
// Content-Type to application/json before calling WriteHeader, so they are
// identified by that header and passed through untouched — only the stdlib
// mux's empty/text-plain Content-Type is rewritten.
type apiErrorWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	deferred    bool
	allow       string
}

func (w *apiErrorWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	if (status == http.StatusNotFound || status == http.StatusMethodNotAllowed) && looksLikeMuxFallback(w.ResponseWriter) {
		w.deferred = true
		if status == http.StatusMethodNotAllowed {
			w.allow = w.ResponseWriter.Header().Get("Allow")
		}
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap exposes the underlying ResponseWriter so helpers like http.ResponseController
// (used by response writer type assertions e.g. for Flush/Hijack) can reach it.
func (w *apiErrorWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// looksLikeMuxFallback reports whether the response's Content-Type still
// matches what http.ServeMux sets before its own 404/405 fallback body
// (empty, since headers haven't been touched, or the "text/plain" it sets
// explicitly) rather than a handler that already wrote a JSON envelope.
func looksLikeMuxFallback(w http.ResponseWriter) bool {
	ct := w.Header().Get("Content-Type")
	return ct == "" || strings.HasPrefix(ct, "text/plain")
}

func (w *apiErrorWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.deferred {
		w.emitEnvelope()
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}

// finish emits the JSON envelope for a deferred 404/405 that never received
// a Write call (the stdlib mux normally does write a plain-text body, but
// nothing guarantees that).
func (w *apiErrorWriter) finish() {
	if w.deferred {
		w.emitEnvelope()
	}
}

func (w *apiErrorWriter) emitEnvelope() {
	w.deferred = false
	w.Header().Del("Content-Type")
	if w.allow != "" {
		w.Header().Set("Allow", w.allow)
	}
	msg := "not found"
	if w.status == http.StatusMethodNotAllowed {
		msg = "method not allowed"
	}
	response.Error(w.ResponseWriter, w.status, msg)
}
