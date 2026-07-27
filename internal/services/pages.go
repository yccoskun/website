package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yccoskun/website/internal/models"
)

// Known page slugs.
const (
	PageHome     = "home"
	PageWork     = "work"
	PageStudio   = "studio"
	PageNotes    = "notes"
	PageResume   = "resume"
	PageNotFound = "not_found"
)

var knownPageSlugs = map[string]struct{}{
	PageHome: {}, PageWork: {}, PageStudio: {},
	PageNotes: {}, PageResume: {}, PageNotFound: {},
}

// PageService manages CMS page documents.
type PageService struct {
	db *sql.DB
}

// NewPageService constructs a PageService.
func NewPageService(db *sql.DB) *PageService {
	return &PageService{db: db}
}

// PageInput is the writable page payload.
type PageInput struct {
	Title           string
	MetaDescription string
	BodyJSON        string
}

const pageColumns = `slug, title, meta_description, body_json`

func scanPage(scanner interface {
	Scan(dest ...any) error
}) (models.Page, error) {
	var p models.Page
	err := scanner.Scan(&p.Slug, &p.Title, &p.MetaDescription, &p.BodyJSON)
	return p, err
}

// Get returns a page by slug. Missing pages return an empty shell (not an error)
// so public rendering can show honest empty states.
func (s *PageService) Get(slug string) (models.Page, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return models.Page{}, fmt.Errorf("%w: slug is required", ErrValidation)
	}
	row := s.db.QueryRow(`SELECT `+pageColumns+` FROM pages WHERE slug = ?`, slug)
	p, err := scanPage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Page{Slug: slug, BodyJSON: "{}"}, nil
	}
	if err != nil {
		return models.Page{}, fmt.Errorf("get page: %w", err)
	}
	return p, nil
}

// List returns all stored pages.
func (s *PageService) List() ([]models.Page, error) {
	rows, err := s.db.Query(`SELECT ` + pageColumns + ` FROM pages ORDER BY slug ASC`)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.Page, 0)
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages: %w", err)
	}
	return out, nil
}

// Upsert creates or replaces a page by slug.
func (s *PageService) Upsert(slug string, in PageInput) (models.Page, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return models.Page{}, fmt.Errorf("%w: slug is required", ErrValidation)
	}
	if _, ok := knownPageSlugs[slug]; !ok {
		return models.Page{}, fmt.Errorf("%w: unknown page slug %q", ErrValidation, slug)
	}
	body := strings.TrimSpace(in.BodyJSON)
	if body == "" {
		body = "{}"
	}
	if !json.Valid([]byte(body)) {
		return models.Page{}, fmt.Errorf("%w: body_json must be valid JSON", ErrValidation)
	}
	if err := validatePageBody(slug, body); err != nil {
		return models.Page{}, err
	}

	_, err := s.db.Exec(
		`INSERT INTO pages (slug, title, meta_description, body_json) VALUES (?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		   title = excluded.title,
		   meta_description = excluded.meta_description,
		   body_json = excluded.body_json`,
		slug, strings.TrimSpace(in.Title), in.MetaDescription, body,
	)
	if err != nil {
		return models.Page{}, fmt.Errorf("upsert page: %w", err)
	}
	return s.Get(slug)
}

// HomeBody is the typed home page body.
type HomeBody struct {
	Eyebrow   string       `json:"eyebrow"`
	Headline  string       `json:"headline"`
	Intro     string       `json:"intro"`
	Domains   []HomeDomain `json:"domains"`
	Now       string       `json:"now"`
	Accordion bool         `json:"accordion"`
}

// HomeDomain is a focus block on the home page.
type HomeDomain struct {
	Title  string          `json:"title"`
	Blurb  string          `json:"blurb"`
	Offset string          `json:"offset"`
	Link   *HomeDomainLink `json:"link"`
}

// HomeDomainLink is an optional quiet link under a domain.
type HomeDomainLink struct {
	To    string `json:"to"`
	Label string `json:"label"`
}

// WorkPageBody is work list chrome.
type WorkPageBody struct {
	Eyebrow      string `json:"eyebrow"`
	Headline     string `json:"headline"`
	Intro        string `json:"intro"`
	EmptyMessage string `json:"empty_message"`
	Accordion    bool   `json:"accordion"`
}

// StudioPageBody is studio page chrome.
type StudioPageBody struct {
	Eyebrow      string `json:"eyebrow"`
	Headline     string `json:"headline"`
	Intro        string `json:"intro"`
	ToolsLine    string `json:"tools_line"`
	EmptyMessage string `json:"empty_message"`
}

// NotesPageBody is notes/blog chrome.
type NotesPageBody struct {
	Eyebrow      string `json:"eyebrow"`
	Headline     string `json:"headline"`
	Intro        string `json:"intro"`
	EmptyMessage string `json:"empty_message"`
}

// ResumePageBody is resume header chrome.
type ResumePageBody struct {
	Eyebrow    string `json:"eyebrow"`
	Headline   string `json:"headline"`
	Blurb      string `json:"blurb"`
	PDFMediaID *int64 `json:"pdf_media_id"`
}

// NotFoundPageBody is 404 chrome.
type NotFoundPageBody struct {
	Eyebrow  string `json:"eyebrow"`
	Headline string `json:"headline"`
	Body     string `json:"body"`
}

func validatePageBody(slug, body string) error {
	switch slug {
	case PageHome:
		var b HomeBody
		if err := json.Unmarshal([]byte(body), &b); err != nil {
			return fmt.Errorf("%w: home body: %v", ErrValidation, err)
		}
	case PageWork:
		var b WorkPageBody
		if err := json.Unmarshal([]byte(body), &b); err != nil {
			return fmt.Errorf("%w: work body: %v", ErrValidation, err)
		}
	case PageStudio:
		var b StudioPageBody
		if err := json.Unmarshal([]byte(body), &b); err != nil {
			return fmt.Errorf("%w: studio body: %v", ErrValidation, err)
		}
	case PageNotes:
		var b NotesPageBody
		if err := json.Unmarshal([]byte(body), &b); err != nil {
			return fmt.Errorf("%w: notes body: %v", ErrValidation, err)
		}
	case PageResume:
		var b ResumePageBody
		if err := json.Unmarshal([]byte(body), &b); err != nil {
			return fmt.Errorf("%w: resume body: %v", ErrValidation, err)
		}
	case PageNotFound:
		var b NotFoundPageBody
		if err := json.Unmarshal([]byte(body), &b); err != nil {
			return fmt.Errorf("%w: not_found body: %v", ErrValidation, err)
		}
	}
	return nil
}

// ParseResumeHeader extracts resume header fields from the resume page body.
func ParseResumeHeader(bodyJSON string, pdfURL string) models.ResumeHeader {
	var b ResumePageBody
	_ = json.Unmarshal([]byte(bodyJSON), &b)
	h := models.ResumeHeader{
		Eyebrow:    b.Eyebrow,
		Headline:   b.Headline,
		Blurb:      b.Blurb,
		PDFMediaID: b.PDFMediaID,
		PDFURL:     pdfURL,
	}
	return h
}
