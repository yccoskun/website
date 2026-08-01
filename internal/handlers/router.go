package handlers

import (
	"net/http"

	"github.com/yccoskun/website/internal/middleware"
	"github.com/yccoskun/website/internal/response"
)

// NewRouter builds the full HTTP handler: API routes first, with a catch-all
// so nothing under /api/ ever falls through to the SPA handler.
//
// Deliberate behavior note: because the methodless "/api/" pattern always
// matches, the Go 1.22 mux never generates a 405 under /api/ — a
// wrong-method request such as POST /api/health also lands on the catch-all
// and gets the 404 envelope. The trade-off buys a hard guarantee that every
// response under /api/ is a JSON envelope, never SPA HTML.
func NewRouter(spa http.Handler, deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", Health)

	mux.HandleFunc("GET /api/posts", deps.ListPublishedPosts)
	mux.HandleFunc("GET /api/posts/{slug}", deps.GetPublishedPost)
	mux.HandleFunc("GET /api/resume", deps.GetResume)
	mux.HandleFunc("GET /api/settings", deps.GetSettings)
	mux.HandleFunc("GET /api/pages/{slug}", deps.GetPage)
	mux.HandleFunc("GET /api/work", deps.ListWork)
	mux.HandleFunc("GET /api/studio", deps.ListStudio)
	mux.HandleFunc("GET /media/{id}", deps.ServeMedia)

	loginLimiter := middleware.NewLoginRateLimiter()
	mux.Handle("POST /api/admin/login", loginLimiter.Middleware(http.HandlerFunc(deps.AdminLogin)))

	requireAuth := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireSession(deps.Sessions, h)
	}

	mux.Handle("POST /api/admin/logout", http.HandlerFunc(deps.AdminLogout))
	mux.Handle("GET /api/admin/me", requireAuth(deps.AdminMe))

	mux.Handle("POST /api/admin/preview", requireAuth(deps.AdminPreview))
	mux.Handle("POST /api/admin/import", requireAuth(deps.AdminImport))
	mux.Handle("POST /api/admin/export", requireAuth(deps.AdminExport))

	mux.Handle("GET /api/admin/posts", requireAuth(deps.AdminListPosts))
	mux.Handle("POST /api/admin/posts", requireAuth(deps.AdminCreatePost))
	mux.Handle("GET /api/admin/posts/{id}", requireAuth(deps.AdminGetPost))
	mux.Handle("PUT /api/admin/posts/{id}", requireAuth(deps.AdminUpdatePost))
	mux.Handle("DELETE /api/admin/posts/{id}", requireAuth(deps.AdminDeletePost))

	mux.Handle("GET /api/admin/resume/entries", requireAuth(deps.AdminListResumeEntries))
	mux.Handle("POST /api/admin/resume/entries", requireAuth(deps.AdminCreateResumeEntry))
	mux.Handle("PUT /api/admin/resume/entries/{id}", requireAuth(deps.AdminUpdateResumeEntry))
	mux.Handle("DELETE /api/admin/resume/entries/{id}", requireAuth(deps.AdminDeleteResumeEntry))

	mux.Handle("GET /api/admin/resume/sections", requireAuth(deps.AdminListResumeSections))
	mux.Handle("POST /api/admin/resume/sections", requireAuth(deps.AdminCreateResumeSection))
	mux.Handle("PUT /api/admin/resume/sections/{id}", requireAuth(deps.AdminUpdateResumeSection))
	mux.Handle("DELETE /api/admin/resume/sections/{id}", requireAuth(deps.AdminDeleteResumeSection))

	mux.Handle("GET /api/admin/settings", requireAuth(deps.AdminGetSettings))
	mux.Handle("PUT /api/admin/settings", requireAuth(deps.AdminPutSettings))

	mux.Handle("GET /api/admin/pages", requireAuth(deps.AdminListPages))
	mux.Handle("GET /api/admin/pages/{slug}", requireAuth(deps.AdminGetPage))
	mux.Handle("PUT /api/admin/pages/{slug}", requireAuth(deps.AdminPutPage))

	mux.Handle("GET /api/admin/work", requireAuth(deps.AdminListWork))
	mux.Handle("POST /api/admin/work", requireAuth(deps.AdminCreateWork))
	mux.Handle("PUT /api/admin/work/{id}", requireAuth(deps.AdminUpdateWork))
	mux.Handle("DELETE /api/admin/work/{id}", requireAuth(deps.AdminDeleteWork))

	mux.Handle("GET /api/admin/studio", requireAuth(deps.AdminListStudio))
	mux.Handle("POST /api/admin/studio", requireAuth(deps.AdminCreateStudio))
	mux.Handle("PUT /api/admin/studio/{id}", requireAuth(deps.AdminUpdateStudio))
	mux.Handle("DELETE /api/admin/studio/{id}", requireAuth(deps.AdminDeleteStudio))

	mux.Handle("GET /api/admin/media", requireAuth(deps.AdminListMedia))
	mux.Handle("POST /api/admin/media", requireAuth(deps.AdminUploadMedia))
	mux.Handle("DELETE /api/admin/media/{id}", requireAuth(deps.AdminDeleteMedia))

	mux.HandleFunc("GET /robots.txt", deps.Robots)
	mux.HandleFunc("GET /sitemap.xml", deps.Sitemap)
	mux.HandleFunc("GET /rss.xml", deps.RSS)

	mux.HandleFunc("/api/", NotFound)
	mux.Handle("/", spa)
	return middleware.SecurityHeaders(mux)
}

// NotFound is the catch-all for API requests matching no registered route.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	response.Error(w, http.StatusNotFound, "not found")
}
