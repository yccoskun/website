package services

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/yccoskun/website/internal/models"
)

// ResumeService manages resume sections and entries.
type ResumeService struct {
	db *sql.DB
}

// NewResumeService constructs a ResumeService backed by db.
func NewResumeService(db *sql.DB) *ResumeService {
	return &ResumeService{db: db}
}

// ResumeEntryInput is the writable subset of a resume entry.
type ResumeEntryInput struct {
	SectionID int64
	Org       string
	Role      string
	Location  string
	Period    string
	BodyMD    string
	Tech      string
	SortOrder int
}

const resumeEntryColumns = `id, section_id, org, role, location, period, body_md, body_html, tech, sort_order`

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

// GetGrouped returns all sections with their entries for the public resume API.
func (s *ResumeService) GetGrouped() (models.Resume, error) {
	secRows, err := s.db.Query(
		`SELECT id, kind, title, sort_order FROM resume_sections ORDER BY sort_order ASC, id ASC`,
	)
	if err != nil {
		return models.Resume{}, fmt.Errorf("list resume sections: %w", err)
	}
	defer func() { _ = secRows.Close() }()

	sections := make([]models.ResumeSection, 0)
	for secRows.Next() {
		var sec models.ResumeSection
		if err := secRows.Scan(&sec.ID, &sec.Kind, &sec.Title, &sec.SortOrder); err != nil {
			return models.Resume{}, fmt.Errorf("scan resume section: %w", err)
		}
		sec.Entries = []models.ResumeEntry{}
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
	return models.Resume{Sections: sections}, nil
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
	row := s.db.QueryRow(`SELECT `+resumeEntryColumns+` FROM resume_entries WHERE id = ?`, id)
	e, err := scanResumeEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ResumeEntry{}, ErrNotFound
	}
	if err != nil {
		return models.ResumeEntry{}, fmt.Errorf("get resume entry: %w", err)
	}
	return e, nil
}

func (s *ResumeService) sectionExists(id int64) error {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM resume_sections WHERE id = ?`, id).Scan(&exists)
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
	if in.SectionID <= 0 {
		return models.ResumeEntry{}, fmt.Errorf("%w: section_id is required", ErrValidation)
	}
	if err := s.sectionExists(in.SectionID); err != nil {
		return models.ResumeEntry{}, err
	}

	html, err := RenderMarkdown(in.BodyMD)
	if err != nil {
		return models.ResumeEntry{}, err
	}

	res, err := s.db.Exec(
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
	return s.GetEntryByID(id)
}

// UpdateEntry replaces a resume entry.
func (s *ResumeService) UpdateEntry(id int64, in ResumeEntryInput) (models.ResumeEntry, error) {
	if _, err := s.GetEntryByID(id); err != nil {
		return models.ResumeEntry{}, err
	}
	if in.SectionID <= 0 {
		return models.ResumeEntry{}, fmt.Errorf("%w: section_id is required", ErrValidation)
	}
	if err := s.sectionExists(in.SectionID); err != nil {
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
