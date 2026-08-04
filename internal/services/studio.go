package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/yccoskun/website/internal/models"
)

// StudioService manages studio catalogue pieces.
type StudioService struct {
	db *sql.DB
}

// NewStudioService constructs a StudioService.
func NewStudioService(db *sql.DB) *StudioService {
	return &StudioService{db: db}
}

// StudioInput is the writable studio piece payload.
type StudioInput struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Year         string `json:"year"`
	Medium       string `json:"medium"`
	Caption      string `json:"caption"`
	ImageMediaID *int64 `json:"image_media_id"`
	SortOrder    int    `json:"sort_order"`
	Published    bool   `json:"published"`
}

const studioColumns = `id, slug, title, year, medium, caption, image_media_id, sort_order, published`

func scanStudioPiece(scanner interface {
	Scan(dest ...any) error
}) (models.StudioPiece, error) {
	var (
		p            models.StudioPiece
		imageMediaID sql.NullInt64
		published    int
	)
	err := scanner.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Year, &p.Medium, &p.Caption,
		&imageMediaID, &p.SortOrder, &published,
	)
	if err != nil {
		return models.StudioPiece{}, err
	}
	if imageMediaID.Valid {
		v := imageMediaID.Int64
		p.ImageMediaID = &v
		p.ImageURL = fmt.Sprintf("/media/%d", v)
	}
	p.Published = published != 0
	return p, nil
}

func validateStudioInput(in StudioInput) error {
	if strings.TrimSpace(in.Slug) == "" {
		return fmt.Errorf("%w: slug is required", ErrValidation)
	}
	if strings.TrimSpace(in.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	return nil
}

// ListPublished returns published studio pieces for the public API.
func (s *StudioService) ListPublished() ([]models.StudioPiece, error) {
	rows, err := s.db.Query(
		`SELECT ` + studioColumns + ` FROM studio_pieces
		 WHERE published = 1 ORDER BY sort_order ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list published studio: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectStudioPieces(rows)
}

// AdminList returns all studio pieces.
func (s *StudioService) AdminList() ([]models.StudioPiece, error) {
	return s.adminList(s.db)
}

func (s *StudioService) adminList(q dbQuerier) ([]models.StudioPiece, error) {
	rows, err := q.Query(
		`SELECT ` + studioColumns + ` FROM studio_pieces ORDER BY sort_order ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("admin list studio: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectStudioPieces(rows)
}

// GetByID returns a studio piece by id.
func (s *StudioService) GetByID(id int64) (models.StudioPiece, error) {
	return s.getByID(s.db, id)
}

func (s *StudioService) getByID(q dbQuerier, id int64) (models.StudioPiece, error) {
	row := q.QueryRow(`SELECT `+studioColumns+` FROM studio_pieces WHERE id = ?`, id)
	p, err := scanStudioPiece(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.StudioPiece{}, ErrNotFound
	}
	if err != nil {
		return models.StudioPiece{}, fmt.Errorf("get studio piece: %w", err)
	}
	return p, nil
}

func (s *StudioService) mediaExists(q dbQuerier, id int64) error {
	var exists int
	err := q.QueryRow(`SELECT 1 FROM media_assets WHERE id = ?`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: image_media_id not found", ErrValidation)
	}
	if err != nil {
		return fmt.Errorf("check media: %w", err)
	}
	return nil
}

// Create inserts a studio piece.
func (s *StudioService) Create(in StudioInput) (models.StudioPiece, error) {
	return s.create(s.db, in)
}

func (s *StudioService) create(q dbQuerier, in StudioInput) (models.StudioPiece, error) {
	if err := validateStudioInput(in); err != nil {
		return models.StudioPiece{}, err
	}
	if in.ImageMediaID != nil {
		if err := s.mediaExists(q, *in.ImageMediaID); err != nil {
			return models.StudioPiece{}, err
		}
	}
	published := 0
	if in.Published {
		published = 1
	}
	res, err := q.Exec(
		`INSERT INTO studio_pieces (slug, title, year, medium, caption, image_media_id, sort_order, published)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(in.Slug), strings.TrimSpace(in.Title), in.Year, in.Medium, in.Caption,
		in.ImageMediaID, in.SortOrder, published,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return models.StudioPiece{}, fmt.Errorf("%w: slug already exists", ErrConflict)
		}
		return models.StudioPiece{}, fmt.Errorf("create studio piece: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.StudioPiece{}, fmt.Errorf("create studio piece id: %w", err)
	}
	return s.getByID(q, id)
}

// Update replaces a studio piece.
func (s *StudioService) Update(id int64, in StudioInput) (models.StudioPiece, error) {
	if _, err := s.GetByID(id); err != nil {
		return models.StudioPiece{}, err
	}
	if err := validateStudioInput(in); err != nil {
		return models.StudioPiece{}, err
	}
	if in.ImageMediaID != nil {
		if err := s.mediaExists(s.db, *in.ImageMediaID); err != nil {
			return models.StudioPiece{}, err
		}
	}
	published := 0
	if in.Published {
		published = 1
	}
	_, err := s.db.Exec(
		`UPDATE studio_pieces SET slug = ?, title = ?, year = ?, medium = ?, caption = ?,
		 image_media_id = ?, sort_order = ?, published = ? WHERE id = ?`,
		strings.TrimSpace(in.Slug), strings.TrimSpace(in.Title), in.Year, in.Medium, in.Caption,
		in.ImageMediaID, in.SortOrder, published, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return models.StudioPiece{}, fmt.Errorf("%w: slug already exists", ErrConflict)
		}
		return models.StudioPiece{}, fmt.Errorf("update studio piece: %w", err)
	}
	return s.GetByID(id)
}

// Delete removes a studio piece.
func (s *StudioService) Delete(id int64) error {
	return s.delete(s.db, id)
}

func (s *StudioService) delete(q dbQuerier, id int64) error {
	res, err := q.Exec(`DELETE FROM studio_pieces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete studio piece: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete studio piece rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func collectStudioPieces(rows *sql.Rows) ([]models.StudioPiece, error) {
	out := make([]models.StudioPiece, 0)
	for rows.Next() {
		p, err := scanStudioPiece(rows)
		if err != nil {
			return nil, fmt.Errorf("scan studio piece: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate studio pieces: %w", err)
	}
	return out, nil
}
