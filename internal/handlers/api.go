package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/config"
	"github.com/yccoskun/website/internal/middleware"
	"github.com/yccoskun/website/internal/response"
	"github.com/yccoskun/website/internal/securitylog"
	"github.com/yccoskun/website/internal/services"
)

// Deps holds handler dependencies injected at router construction.
type Deps struct {
	Posts    *services.PostService
	Resume   *services.ResumeService
	Sessions *services.SessionService
	Settings *services.SettingsService
	Pages    *services.PageService
	Work     *services.WorkService
	Studio   *services.StudioService
	Media    *services.MediaService
	Config   config.Config
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type postRequest struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	ContentMD string `json:"content_md"`
	Published bool   `json:"published"`
}

type resumeEntryRequest struct {
	SectionID int64  `json:"section_id"`
	Org       string `json:"org"`
	Role      string `json:"role"`
	Location  string `json:"location"`
	Period    string `json:"period"`
	BodyMD    string `json:"body_md"`
	Tech      string `json:"tech"`
	SortOrder int    `json:"sort_order"`
}

type previewRequest struct {
	ContentMD string `json:"content_md"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		response.Error(w, http.StatusNotFound, "not found")
	case errors.Is(err, services.ErrValidation):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrConflict):
		response.Error(w, http.StatusConflict, err.Error())
	default:
		log.Printf("handler: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal error")
	}
}

// ListPublishedPosts serves GET /api/posts.
func (d Deps) ListPublishedPosts(w http.ResponseWriter, _ *http.Request) {
	posts, err := d.Posts.ListPublished()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, posts)
}

// GetPublishedPost serves GET /api/posts/{slug}.
func (d Deps) GetPublishedPost(w http.ResponseWriter, r *http.Request) {
	post, err := d.Posts.GetBySlug(r.PathValue("slug"))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, post)
}

// GetResume serves GET /api/resume.
func (d Deps) GetResume(w http.ResponseWriter, _ *http.Request) {
	resume, err := d.Resume.GetGrouped()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, resume)
}

// dummyPasswordHash is a bcrypt hash used when admin credentials are unset or
// the username does not match, so login always pays a password-check cost.
const dummyPasswordHash = "$2a$12$cjmQmxqZD1IJVx6h08hAx.eEAOuIKCSlLuSDe9EtC32yIzVjSl3/."

// confirmAdminPassword verifies the admin password for sensitive step-up actions.
// Missing/empty password → 403 "password confirmation required" (short-circuits like
// auth.CheckPassword). Non-empty wrong/unset credentials → 403 "invalid password"
// after a bcrypt compare (dummy hash when unset) for timing parity with login.
// Returns false after writing the error response.
func (d Deps) confirmAdminPassword(w http.ResponseWriter, password string) bool {
	if password == "" {
		response.Error(w, http.StatusForbidden, "password confirmation required")
		return false
	}
	hash := dummyPasswordHash
	if d.Config.AdminPasswordHash != "" {
		hash = d.Config.AdminPasswordHash
	}
	passOK := auth.CheckPassword(hash, password)
	if d.Config.AdminPasswordHash == "" || !passOK {
		response.Error(w, http.StatusForbidden, "invalid password")
		return false
	}
	return true
}

// AdminLogin serves POST /api/admin/login.
func (d Deps) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	hash := dummyPasswordHash
	if d.Config.AdminPasswordHash != "" {
		hash = d.Config.AdminPasswordHash
	}
	passOK := auth.CheckPassword(hash, body.Password)
	userOK := d.Config.AdminUsername != "" &&
		d.Config.AdminPasswordHash != "" &&
		auth.ConstantTimeUsernameEqual(body.Username, d.Config.AdminUsername)
	if !userOK || !passOK {
		securitylog.Event(securitylog.EventLoginFailure, middleware.ClientIP(r))
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token, expires, err := d.Sessions.Create()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	auth.SetSessionCookie(w, r, token, expires)
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminLogout serves POST /api/admin/logout.
// Always clears the cookie and returns 200; destroys the session when present.
func (d Deps) AdminLogout(w http.ResponseWriter, r *http.Request) {
	if d.Sessions != nil {
		if err := d.Sessions.Destroy(auth.SessionToken(r)); err != nil {
			log.Printf("logout: destroy session: %v", err)
		}
	}
	auth.ClearSessionCookie(w, r)
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminMe serves GET /api/admin/me.
func (d Deps) AdminMe(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"username": d.Config.AdminUsername})
}

// AdminPreview serves POST /api/admin/preview — Markdown → sanitized HTML.
func (d Deps) AdminPreview(w http.ResponseWriter, r *http.Request) {
	var body previewRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	html, err := services.RenderMarkdown(body.ContentMD)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"html": html})
}

// AdminListPosts serves GET /api/admin/posts.
func (d Deps) AdminListPosts(w http.ResponseWriter, _ *http.Request) {
	posts, err := d.Posts.AdminList()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, posts)
}

// AdminGetPost serves GET /api/admin/posts/{id}.
func (d Deps) AdminGetPost(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	post, err := d.Posts.GetByID(id)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, post)
}

// AdminCreatePost serves POST /api/admin/posts.
func (d Deps) AdminCreatePost(w http.ResponseWriter, r *http.Request) {
	var body postRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	post, err := d.Posts.Create(services.PostInput{
		Slug:      body.Slug,
		Title:     body.Title,
		Summary:   body.Summary,
		ContentMD: body.ContentMD,
		Published: body.Published,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, post)
}

// AdminUpdatePost serves PUT /api/admin/posts/{id}.
func (d Deps) AdminUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body postRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	post, err := d.Posts.Update(id, services.PostInput{
		Slug:      body.Slug,
		Title:     body.Title,
		Summary:   body.Summary,
		ContentMD: body.ContentMD,
		Published: body.Published,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, post)
}

// AdminDeletePost serves DELETE /api/admin/posts/{id}.
func (d Deps) AdminDeletePost(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := d.Posts.Delete(id); err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminListResumeEntries serves GET /api/admin/resume/entries.
func (d Deps) AdminListResumeEntries(w http.ResponseWriter, _ *http.Request) {
	entries, err := d.Resume.AdminListEntries()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, entries)
}

// AdminCreateResumeEntry serves POST /api/admin/resume/entries.
func (d Deps) AdminCreateResumeEntry(w http.ResponseWriter, r *http.Request) {
	var body resumeEntryRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	entry, err := d.Resume.CreateEntry(services.ResumeEntryInput{
		SectionID: body.SectionID,
		Org:       body.Org,
		Role:      body.Role,
		Location:  body.Location,
		Period:    body.Period,
		BodyMD:    body.BodyMD,
		Tech:      body.Tech,
		SortOrder: body.SortOrder,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, entry)
}

// AdminUpdateResumeEntry serves PUT /api/admin/resume/entries/{id}.
func (d Deps) AdminUpdateResumeEntry(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body resumeEntryRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	entry, err := d.Resume.UpdateEntry(id, services.ResumeEntryInput{
		SectionID: body.SectionID,
		Org:       body.Org,
		Role:      body.Role,
		Location:  body.Location,
		Period:    body.Period,
		BodyMD:    body.BodyMD,
		Tech:      body.Tech,
		SortOrder: body.SortOrder,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, entry)
}

// AdminDeleteResumeEntry serves DELETE /api/admin/resume/entries/{id}.
func (d Deps) AdminDeleteResumeEntry(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := d.Resume.DeleteEntry(id); err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
