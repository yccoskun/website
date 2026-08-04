package services

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yccoskun/website/internal/models"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned on unique constraint violations (e.g. slug).
var ErrConflict = errors.New("conflict")

// ErrValidation is returned for invalid input that should map to HTTP 400.
var ErrValidation = errors.New("validation")

// PostService manages blog posts.
type PostService struct {
	db *sql.DB
}

// NewPostService constructs a PostService backed by db.
func NewPostService(db *sql.DB) *PostService {
	return &PostService{db: db}
}

// PostInput is the writable subset of a post for create/update.
type PostInput struct {
	Slug      string
	Title     string
	Summary   string
	ContentMD string
	Published bool
}

const postColumns = `id, slug, title, summary, content_md, content_html, published,
	created_at, updated_at, published_at`

func scanPost(scanner interface {
	Scan(dest ...any) error
}) (models.Post, error) {
	var (
		p           models.Post
		published   int
		publishedAt sql.NullString
	)
	err := scanner.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Summary, &p.ContentMD, &p.ContentHTML,
		&published, &p.CreatedAt, &p.UpdatedAt, &publishedAt,
	)
	if err != nil {
		return models.Post{}, err
	}
	p.Published = published != 0
	if publishedAt.Valid {
		v := publishedAt.String
		p.PublishedAt = &v
	}
	return p, nil
}

var postSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const maxPostSlugLen = 100

func validatePostInput(in PostInput) error {
	if strings.TrimSpace(in.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		return fmt.Errorf("%w: slug is required", ErrValidation)
	}
	if len(slug) > maxPostSlugLen || !postSlugPattern.MatchString(slug) {
		return fmt.Errorf("%w: slug must be lowercase letters, digits, and single hyphens (max 100)", ErrValidation)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed")
}

const postSummaryColumns = `id, slug, title, summary, created_at, updated_at, published_at`

// ListPublished returns published post summaries (no content bodies), newest first.
func (s *PostService) ListPublished() ([]models.PostSummary, error) {
	rows, err := s.db.Query(
		`SELECT `+postSummaryColumns+` FROM posts
		 WHERE published = 1
		 ORDER BY COALESCE(published_at, created_at) DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list published posts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.PostSummary, 0)
	for rows.Next() {
		var (
			sum         models.PostSummary
			publishedAt sql.NullString
		)
		if err := rows.Scan(
			&sum.ID, &sum.Slug, &sum.Title, &sum.Summary,
			&sum.CreatedAt, &sum.UpdatedAt, &publishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan post summary: %w", err)
		}
		if publishedAt.Valid {
			v := publishedAt.String
			sum.PublishedAt = &v
		}
		out = append(out, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post summaries: %w", err)
	}
	return out, nil
}

// GetBySlug returns a published post by slug.
func (s *PostService) GetBySlug(slug string) (models.Post, error) {
	row := s.db.QueryRow(
		`SELECT `+postColumns+` FROM posts WHERE slug = ? AND published = 1`, slug,
	)
	p, err := scanPost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Post{}, ErrNotFound
	}
	if err != nil {
		return models.Post{}, fmt.Errorf("get post by slug: %w", err)
	}
	return p, nil
}

// AdminList returns all posts, newest first.
func (s *PostService) AdminList() ([]models.Post, error) {
	rows, err := s.db.Query(
		`SELECT `+postColumns+` FROM posts ORDER BY created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("admin list posts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectPosts(rows)
}

// GetByID returns any post by id.
func (s *PostService) GetByID(id int64) (models.Post, error) {
	row := s.db.QueryRow(`SELECT `+postColumns+` FROM posts WHERE id = ?`, id)
	p, err := scanPost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Post{}, ErrNotFound
	}
	if err != nil {
		return models.Post{}, fmt.Errorf("get post by id: %w", err)
	}
	return p, nil
}

// Create inserts a new post. Renders markdown and sets published_at when published.
func (s *PostService) Create(in PostInput) (models.Post, error) {
	if err := validatePostInput(in); err != nil {
		return models.Post{}, err
	}
	html, err := RenderMarkdown(in.ContentMD)
	if err != nil {
		return models.Post{}, err
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	published := 0
	var publishedAt any
	if in.Published {
		published = 1
		publishedAt = now
	}

	tx, err := s.db.Begin()
	if err != nil {
		return models.Post{}, fmt.Errorf("begin create post: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`INSERT INTO posts (slug, title, summary, content_md, content_html, published, created_at, updated_at, published_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(in.Slug), strings.TrimSpace(in.Title), in.Summary,
		in.ContentMD, html, published, now, now, publishedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return models.Post{}, fmt.Errorf("%w: slug already exists", ErrConflict)
		}
		return models.Post{}, fmt.Errorf("create post: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Post{}, fmt.Errorf("create post id: %w", err)
	}
	if err := syncMediaReferences(tx, id, in.Published, in.ContentMD, html); err != nil {
		return models.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Post{}, fmt.Errorf("commit create post: %w", err)
	}
	return s.GetByID(id)
}

// Update replaces a post. Sets published_at when transitioning to published.
func (s *PostService) Update(id int64, in PostInput) (models.Post, error) {
	if err := validatePostInput(in); err != nil {
		return models.Post{}, err
	}
	existing, err := s.GetByID(id)
	if err != nil {
		return models.Post{}, err
	}

	html, err := RenderMarkdown(in.ContentMD)
	if err != nil {
		return models.Post{}, err
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	published := 0
	if in.Published {
		published = 1
	}

	var publishedAt any
	switch {
	case in.Published && !existing.Published:
		publishedAt = now
	case in.Published && existing.Published && existing.PublishedAt != nil:
		publishedAt = *existing.PublishedAt
	case in.Published && existing.Published:
		publishedAt = now
	default:
		publishedAt = nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return models.Post{}, fmt.Errorf("begin update post: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`UPDATE posts SET slug = ?, title = ?, summary = ?, content_md = ?, content_html = ?,
		 published = ?, updated_at = ?, published_at = ? WHERE id = ?`,
		strings.TrimSpace(in.Slug), strings.TrimSpace(in.Title), in.Summary,
		in.ContentMD, html, published, now, publishedAt, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return models.Post{}, fmt.Errorf("%w: slug already exists", ErrConflict)
		}
		return models.Post{}, fmt.Errorf("update post: %w", err)
	}
	if err := syncMediaReferences(tx, id, in.Published, in.ContentMD, html); err != nil {
		return models.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Post{}, fmt.Errorf("commit update post: %w", err)
	}
	return s.GetByID(id)
}

// Delete removes a post by id. media_references rows cascade via FK.
func (s *PostService) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM posts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete post rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func collectPosts(rows *sql.Rows) ([]models.Post, error) {
	out := make([]models.Post, 0)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return out, nil
}
