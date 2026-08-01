package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/models"
	"github.com/yccoskun/website/internal/response"
	"github.com/yccoskun/website/internal/services"
)

type settingsRequest struct {
	Settings map[string]string `json:"settings"`
}

type pageRequest struct {
	Title           string `json:"title"`
	MetaDescription string `json:"meta_description"`
	BodyJSON        string `json:"body_json"`
}

type workRequest struct {
	Name      string   `json:"name"`
	OneLiner  string   `json:"one_liner"`
	Body      string   `json:"body"`
	Stack     []string `json:"stack"`
	Status    string   `json:"status"`
	Href      string   `json:"href"`
	SortOrder int      `json:"sort_order"`
}

type studioRequest struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Year         string `json:"year"`
	Medium       string `json:"medium"`
	Caption      string `json:"caption"`
	ImageMediaID *int64 `json:"image_media_id"`
	SortOrder    int    `json:"sort_order"`
	Published    bool   `json:"published"`
}

type resumeSectionRequest struct {
	Kind      models.ResumeSectionKind `json:"kind"`
	Title     string                   `json:"title"`
	SortOrder int                      `json:"sort_order"`
	Accordion bool                     `json:"accordion"`
}

// GetSettings serves GET /api/settings.
func (d Deps) GetSettings(w http.ResponseWriter, _ *http.Request) {
	if d.Settings == nil {
		response.JSON(w, http.StatusOK, models.PublicSettings{Nav: []models.NavItem{}})
		return
	}
	settings, err := d.Settings.Public()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, settings)
}

// GetPage serves GET /api/pages/{slug}.
func (d Deps) GetPage(w http.ResponseWriter, r *http.Request) {
	page, err := d.Pages.Get(r.PathValue("slug"))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, page)
}

// ListWork serves GET /api/work.
func (d Deps) ListWork(w http.ResponseWriter, _ *http.Request) {
	items, err := d.Work.List()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// ListStudio serves GET /api/studio.
func (d Deps) ListStudio(w http.ResponseWriter, _ *http.Request) {
	items, err := d.Studio.ListPublished()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// ServeMedia serves GET /media/{id}.
// Public assets (published studio, resume PDF, published post refs) are cacheable.
// Protected assets require a valid admin session; anonymous callers get 404.
func (d Deps) ServeMedia(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		mediaNotFound(w, r)
		return
	}
	if d.Media == nil {
		mediaNotFound(w, r)
		return
	}
	m, err := d.Media.GetByID(id)
	if err != nil {
		mediaNotFound(w, r)
		return
	}

	public, err := d.Media.IsPubliclyReferenced(id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	cacheControl := "public, max-age=300, must-revalidate"
	if !public {
		if d.Sessions == nil {
			mediaNotFound(w, r)
			return
		}
		ok, err := d.Sessions.Validate(auth.SessionToken(r))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			mediaNotFound(w, r)
			return
		}
		cacheControl = "private, no-store"
	}

	path := d.Media.FilePath(m)
	f, err := os.Open(path)
	if err != nil {
		mediaNotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	stat, err := f.Stat()
	if err != nil {
		mediaNotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", m.Mime)
	w.Header().Set("Cache-Control", cacheControl)
	http.ServeContent(w, r, m.OriginalName, stat.ModTime(), f)
}

// mediaNotFound returns 404 with a non-cacheable Cache-Control so intermediaries
// do not retain a negative response that would outlive a later publish.
func mediaNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	http.NotFound(w, r)
}

// AdminGetSettings serves GET /api/admin/settings.
func (d Deps) AdminGetSettings(w http.ResponseWriter, _ *http.Request) {
	all, err := d.Settings.GetAll()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, all)
}

// AdminPutSettings serves PUT /api/admin/settings.
func (d Deps) AdminPutSettings(w http.ResponseWriter, r *http.Request) {
	var body settingsRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Settings == nil {
		response.Error(w, http.StatusBadRequest, "settings object required")
		return
	}
	all, err := d.Settings.Upsert(body.Settings)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, all)
}

// AdminListPages serves GET /api/admin/pages.
func (d Deps) AdminListPages(w http.ResponseWriter, _ *http.Request) {
	pages, err := d.Pages.List()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, pages)
}

// AdminGetPage serves GET /api/admin/pages/{slug}.
func (d Deps) AdminGetPage(w http.ResponseWriter, r *http.Request) {
	page, err := d.Pages.Get(r.PathValue("slug"))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, page)
}

// AdminPutPage serves PUT /api/admin/pages/{slug}.
func (d Deps) AdminPutPage(w http.ResponseWriter, r *http.Request) {
	var body pageRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	page, err := d.Pages.Upsert(r.PathValue("slug"), services.PageInput{
		Title:           body.Title,
		MetaDescription: body.MetaDescription,
		BodyJSON:        body.BodyJSON,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, page)
}

// AdminListWork serves GET /api/admin/work.
func (d Deps) AdminListWork(w http.ResponseWriter, _ *http.Request) {
	items, err := d.Work.List()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// AdminCreateWork serves POST /api/admin/work.
func (d Deps) AdminCreateWork(w http.ResponseWriter, r *http.Request) {
	var body workRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := d.Work.Create(services.WorkInput{
		Name: body.Name, OneLiner: body.OneLiner, Body: body.Body,
		Stack: body.Stack, Status: body.Status, Href: body.Href, SortOrder: body.SortOrder,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, item)
}

// AdminUpdateWork serves PUT /api/admin/work/{id}.
func (d Deps) AdminUpdateWork(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body workRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := d.Work.Update(id, services.WorkInput{
		Name: body.Name, OneLiner: body.OneLiner, Body: body.Body,
		Stack: body.Stack, Status: body.Status, Href: body.Href, SortOrder: body.SortOrder,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, item)
}

// AdminDeleteWork serves DELETE /api/admin/work/{id}.
func (d Deps) AdminDeleteWork(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := d.Work.Delete(id); err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminListStudio serves GET /api/admin/studio.
func (d Deps) AdminListStudio(w http.ResponseWriter, _ *http.Request) {
	items, err := d.Studio.AdminList()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// AdminCreateStudio serves POST /api/admin/studio.
func (d Deps) AdminCreateStudio(w http.ResponseWriter, r *http.Request) {
	var body studioRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := d.Studio.Create(services.StudioInput{
		Slug: body.Slug, Title: body.Title, Year: body.Year, Medium: body.Medium,
		Caption: body.Caption, ImageMediaID: body.ImageMediaID,
		SortOrder: body.SortOrder, Published: body.Published,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, item)
}

// AdminUpdateStudio serves PUT /api/admin/studio/{id}.
func (d Deps) AdminUpdateStudio(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body studioRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := d.Studio.Update(id, services.StudioInput{
		Slug: body.Slug, Title: body.Title, Year: body.Year, Medium: body.Medium,
		Caption: body.Caption, ImageMediaID: body.ImageMediaID,
		SortOrder: body.SortOrder, Published: body.Published,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, item)
}

// AdminDeleteStudio serves DELETE /api/admin/studio/{id}.
func (d Deps) AdminDeleteStudio(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := d.Studio.Delete(id); err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminListResumeSections serves GET /api/admin/resume/sections.
func (d Deps) AdminListResumeSections(w http.ResponseWriter, _ *http.Request) {
	sections, err := d.Resume.ListSections()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, sections)
}

// AdminCreateResumeSection serves POST /api/admin/resume/sections.
func (d Deps) AdminCreateResumeSection(w http.ResponseWriter, r *http.Request) {
	var body resumeSectionRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	sec, err := d.Resume.CreateSection(services.ResumeSectionInput{
		Kind: body.Kind, Title: body.Title, SortOrder: body.SortOrder, Accordion: body.Accordion,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, sec)
}

// AdminUpdateResumeSection serves PUT /api/admin/resume/sections/{id}.
func (d Deps) AdminUpdateResumeSection(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body resumeSectionRequest
	if err := decodeJSON(w, r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	sec, err := d.Resume.UpdateSection(id, services.ResumeSectionInput{
		Kind: body.Kind, Title: body.Title, SortOrder: body.SortOrder, Accordion: body.Accordion,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, sec)
}

// AdminDeleteResumeSection serves DELETE /api/admin/resume/sections/{id}.
func (d Deps) AdminDeleteResumeSection(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := d.Resume.DeleteSection(id); err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminListMedia serves GET /api/admin/media.
func (d Deps) AdminListMedia(w http.ResponseWriter, _ *http.Request) {
	items, err := d.Media.List()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// AdminUploadMedia serves POST /api/admin/media (multipart field "file").
func (d Deps) AdminUploadMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, services.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(services.MaxUploadBytes + 1<<20); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "file field required")
		return
	}
	defer func() { _ = file.Close() }()

	ct := header.Header.Get("Content-Type")
	asset, err := d.Media.Create(header.Filename, ct, file, header.Size)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, asset)
}

// AdminDeleteMedia serves DELETE /api/admin/media/{id}.
func (d Deps) AdminDeleteMedia(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := d.Media.Delete(id); err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminImport serves POST /api/admin/import.
func (d Deps) AdminImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	var dump services.ContentImport
	if err := json.Unmarshal(raw, &dump); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	importer := &services.ImportService{
		Settings: d.Settings,
		Pages:    d.Pages,
		Work:     d.Work,
		Studio:   d.Studio,
		Resume:   d.Resume,
	}
	result, err := importer.Apply(dump)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// AdminExport serves POST /api/admin/export — full content dump for transfer/import.
func (d Deps) AdminExport(w http.ResponseWriter, _ *http.Request) {
	exporter := &services.ImportService{
		Settings: d.Settings,
		Pages:    d.Pages,
		Work:     d.Work,
		Studio:   d.Studio,
		Resume:   d.Resume,
	}
	dump, err := exporter.Export()
	if err != nil {
		mapServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, dump)
}