package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yccoskun/website/internal/models"
)

// ResumeService manages resume sections and entries.
type ResumeService struct {
	db    *sql.DB
	pages *PageService
	media *MediaService
}

// NewResumeService constructs a ResumeService backed by db.
func NewResumeService(db *sql.DB) *ResumeService {
	return &ResumeService{db: db}
}

// WithPages attaches page/media services so GetGrouped can include header chrome.
func (s *ResumeService) WithPages(pages *PageService, media *MediaService) *ResumeService {
	s.pages = pages
	s.media = media
	return s
}

// ResumeEntryInput is the writable subset of a resume entry.
type ResumeEntryInput struct {
	SectionID int64  `json:"section_id"`
	Org       string `json:"org"`
	Role      string `json:"role"`
	Location  string `json:"location"`
	Period    string `json:"period"`
	BodyMD    string `json:"body_md"`
	Tech      string `json:"tech"`
	SortOrder int    `json:"sort_order"`
}

// ResumeSectionInput is the writable subset of a resume section.
type ResumeSectionInput struct {
	Kind      models.ResumeSectionKind `json:"kind"`
	Title     string                   `json:"title"`
	SortOrder int                      `json:"sort_order"`
	Accordion bool                     `json:"accordion"`
}

const resumeEntryColumns = `id, section_id, org, role, location, period, body_md, body_html, tech, sort_order`
const resumeSectionColumns = `id, kind, title, sort_order, accordion`

func scanResumeEntry(scanner interface {
	Scan(dest ...any) error
}) (models.ResumeEntry, error) {
	var e models.ResumeEntry
	err := scanner.Scan(
		&e.ID, &e.SectionID, &e.Org, &e.Role, &e.Location,
		&e.Period, &e.BodyMD, &e.BodyHTML, &e.Tech, &e.SortOrder,
	)
	return e, err
}

func scanResumeSection(scanner interface {
	Scan(dest ...any) error
}) (models.ResumeSection, error) {
	var (
		sec       models.ResumeSection
		accordion int
	)
	err := scanner.Scan(&sec.ID, &sec.Kind, &sec.Title, &sec.SortOrder, &accordion)
	if err != nil {
		return models.ResumeSection{}, err
	}
	sec.Accordion = accordion != 0
	sec.Entries = []models.ResumeEntry{}
	return sec, nil
}

func accordionInt(on bool) int {
	if on {
		return 1
	}
	return 0
}

// GetGrouped returns all sections with their entries for the public resume API.
func (s *ResumeService) GetGrouped() (models.Resume, error) {
	secRows, err := s.db.Query(
		`SELECT ` + resumeSectionColumns + ` FROM resume_sections ORDER BY sort_order ASC, id ASC`,
	)
	if err != nil {
		return models.Resume{}, fmt.Errorf("list resume sections: %w", err)
	}
	defer func() { _ = secRows.Close() }()

	sections := make([]models.ResumeSection, 0)
	for secRows.Next() {
		sec, err := scanResumeSection(secRows)
		if err != nil {
			return models.Resume{}, fmt.Errorf("scan resume section: %w", err)
		}
		sections = append(sections, sec)
	}
	if err := secRows.Err(); err != nil {
		return models.Resume{}, fmt.Errorf("iterate resume sections: %w", err)
	}

	entryRows, err := s.db.Query(
		`SELECT ` + resumeEntryColumns + ` FROM resume_entries ORDER BY sort_order ASC, id ASC`,
	)
	if err != nil {
		return models.Resume{}, fmt.Errorf("list resume entries: %w", err)
	}
	defer func() { _ = entryRows.Close() }()

	bySection := make(map[int64][]models.ResumeEntry)
	for entryRows.Next() {
		e, err := scanResumeEntry(entryRows)
		if err != nil {
			return models.Resume{}, fmt.Errorf("scan resume entry: %w", err)
		}
		bySection[e.SectionID] = append(bySection[e.SectionID], e)
	}
	if err := entryRows.Err(); err != nil {
		return models.Resume{}, fmt.Errorf("iterate resume entries: %w", err)
	}

	for i := range sections {
		if entries, ok := bySection[sections[i].ID]; ok {
			sections[i].Entries = entries
		}
	}

	header := models.ResumeHeader{}
	if s.pages != nil {
		page, err := s.pages.Get(PageResume)
		if err != nil {
			return models.Resume{}, err
		}
		pdfURL := ""
		var body ResumePageBody
		_ = jsonUnmarshal(page.BodyJSON, &body)
		if s.media != nil {
			pdfURL = s.media.URLForID(body.PDFMediaID)
		}
		header = ParseResumeHeader(page.BodyJSON, pdfURL)
	}

	return models.Resume{Header: header, Sections: sections}, nil
}

func jsonUnmarshal(raw string, dst any) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), dst)
}

// AdminListEntries returns all resume entries ordered for admin display.
func (s *ResumeService) AdminListEntries() ([]models.ResumeEntry, error) {
	rows, err := s.db.Query(
		`SELECT ` + resumeEntryColumns + ` FROM resume_entries
		 ORDER BY section_id ASC, sort_order ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("admin list resume entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.ResumeEntry, 0)
	for rows.Next() {
		e, err := scanResumeEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan resume entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resume entries: %w", err)
	}
	return out, nil
}

// GetEntryByID returns a single resume entry.
func (s *ResumeService) GetEntryByID(id int64) (models.ResumeEntry, error) {
	return s.getEntryByID(s.db, id)
}

func (s *ResumeService) getEntryByID(q dbQuerier, id int64) (models.ResumeEntry, error) {
	row := q.QueryRow(`SELECT `+resumeEntryColumns+` FROM resume_entries WHERE id = ?`, id)
	e, err := scanResumeEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ResumeEntry{}, ErrNotFound
	}
	if err != nil {
		return models.ResumeEntry{}, fmt.Errorf("get resume entry: %w", err)
	}
	return e, nil
}

func (s *ResumeService) sectionExists(q dbQuerier, id int64) error {
	var exists int
	err := q.QueryRow(`SELECT 1 FROM resume_sections WHERE id = ?`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: section not found", ErrValidation)
	}
	if err != nil {
		return fmt.Errorf("check resume section: %w", err)
	}
	return nil
}

// CreateEntry inserts a resume entry under a section.
func (s *ResumeService) CreateEntry(in ResumeEntryInput) (models.ResumeEntry, error) {
	return s.createEntry(s.db, in)
}

func (s *ResumeService) createEntry(q dbQuerier, in ResumeEntryInput) (models.ResumeEntry, error) {
	if in.SectionID <= 0 {
		return models.ResumeEntry{}, fmt.Errorf("%w: section_id is required", ErrValidation)
	}
	if err := s.sectionExists(q, in.SectionID); err != nil {
		return models.ResumeEntry{}, err
	}

	html, err := RenderMarkdown(in.BodyMD)
	if err != nil {
		return models.ResumeEntry{}, err
	}

	res, err := q.Exec(
		`INSERT INTO resume_entries (section_id, org, role, location, period, body_md, body_html, tech, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.SectionID, in.Org, in.Role, in.Location, in.Period, in.BodyMD, html, in.Tech, in.SortOrder,
	)
	if err != nil {
		return models.ResumeEntry{}, fmt.Errorf("create resume entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.ResumeEntry{}, fmt.Errorf("create resume entry id: %w", err)
	}
	return s.getEntryByID(q, id)
}

// UpdateEntry replaces a resume entry.
func (s *ResumeService) UpdateEntry(id int64, in ResumeEntryInput) (models.ResumeEntry, error) {
	if _, err := s.GetEntryByID(id); err != nil {
		return models.ResumeEntry{}, err
	}
	if in.SectionID <= 0 {
		return models.ResumeEntry{}, fmt.Errorf("%w: section_id is required", ErrValidation)
	}
	if err := s.sectionExists(s.db, in.SectionID); err != nil {
		return models.ResumeEntry{}, err
	}

	html, err := RenderMarkdown(in.BodyMD)
	if err != nil {
		return models.ResumeEntry{}, err
	}

	_, err = s.db.Exec(
		`UPDATE resume_entries SET section_id = ?, org = ?, role = ?, location = ?,
		 period = ?, body_md = ?, body_html = ?, tech = ?, sort_order = ? WHERE id = ?`,
		in.SectionID, in.Org, in.Role, in.Location, in.Period, in.BodyMD, html, in.Tech, in.SortOrder, id,
	)
	if err != nil {
		return models.ResumeEntry{}, fmt.Errorf("update resume entry: %w", err)
	}
	return s.GetEntryByID(id)
}

// DeleteEntry removes a resume entry by id.
func (s *ResumeService) DeleteEntry(id int64) error {
	res, err := s.db.Exec(`DELETE FROM resume_entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete resume entry: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete resume entry rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func validateSectionKind(kind models.ResumeSectionKind) error {
	switch kind {
	case models.ResumeKindExperience, models.ResumeKindEducation, models.ResumeKindActivity:
		return nil
	default:
		return fmt.Errorf("%w: kind must be experience, education, or activity", ErrValidation)
	}
}

// ListSections returns all resume sections (no entries).
func (s *ResumeService) ListSections() ([]models.ResumeSection, error) {
	return s.listSections(s.db)
}

func (s *ResumeService) listSections(q dbQuerier) ([]models.ResumeSection, error) {
	rows, err := q.Query(
		`SELECT ` + resumeSectionColumns + ` FROM resume_sections ORDER BY sort_order ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list resume sections: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.ResumeSection, 0)
	for rows.Next() {
		sec, err := scanResumeSection(rows)
		if err != nil {
			return nil, fmt.Errorf("scan resume section: %w", err)
		}
		out = append(out, sec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resume sections: %w", err)
	}
	return out, nil
}

// GetSectionByID returns a section without entries.
func (s *ResumeService) GetSectionByID(id int64) (models.ResumeSection, error) {
	return s.getSectionByID(s.db, id)
}

func (s *ResumeService) getSectionByID(q dbQuerier, id int64) (models.ResumeSection, error) {
	sec, err := scanResumeSection(q.QueryRow(
		`SELECT `+resumeSectionColumns+` FROM resume_sections WHERE id = ?`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return models.ResumeSection{}, ErrNotFound
	}
	if err != nil {
		return models.ResumeSection{}, fmt.Errorf("get resume section: %w", err)
	}
	return sec, nil
}

// CreateSection inserts a resume section.
func (s *ResumeService) CreateSection(in ResumeSectionInput) (models.ResumeSection, error) {
	return s.createSection(s.db, in)
}

func (s *ResumeService) createSection(q dbQuerier, in ResumeSectionInput) (models.ResumeSection, error) {
	if err := validateSectionKind(in.Kind); err != nil {
		return models.ResumeSection{}, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return models.ResumeSection{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	res, err := q.Exec(
		`INSERT INTO resume_sections (kind, title, sort_order, accordion) VALUES (?, ?, ?, ?)`,
		in.Kind, strings.TrimSpace(in.Title), in.SortOrder, accordionInt(in.Accordion),
	)
	if err != nil {
		return models.ResumeSection{}, fmt.Errorf("create resume section: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.ResumeSection{}, fmt.Errorf("create resume section id: %w", err)
	}
	return s.getSectionByID(q, id)
}

// UpdateSection replaces a resume section.
func (s *ResumeService) UpdateSection(id int64, in ResumeSectionInput) (models.ResumeSection, error) {
	if _, err := s.GetSectionByID(id); err != nil {
		return models.ResumeSection{}, err
	}
	if err := validateSectionKind(in.Kind); err != nil {
		return models.ResumeSection{}, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return models.ResumeSection{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	_, err := s.db.Exec(
		`UPDATE resume_sections SET kind = ?, title = ?, sort_order = ?, accordion = ? WHERE id = ?`,
		in.Kind, strings.TrimSpace(in.Title), in.SortOrder, accordionInt(in.Accordion), id,
	)
	if err != nil {
		return models.ResumeSection{}, fmt.Errorf("update resume section: %w", err)
	}
	return s.GetSectionByID(id)
}

// DeleteSection removes a section and cascades entries.
func (s *ResumeService) DeleteSection(id int64) error {
	return s.deleteSection(s.db, id)
}

func (s *ResumeService) deleteSection(q dbQuerier, id int64) error {
	res, err := q.Exec(`DELETE FROM resume_sections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete resume section: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete resume section rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
