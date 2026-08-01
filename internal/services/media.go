package services

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/yccoskun/website/internal/models"
)

const MaxUploadBytes = 20 << 20 // 20 MiB

var allowedUploadMimes = map[string]struct{}{
	"image/jpeg":      {},
	"image/png":       {},
	"image/gif":       {},
	"image/webp":      {},
	"application/pdf": {},
}

// MediaService stores uploaded files on disk and metadata in SQLite.
type MediaService struct {
	db  *sql.DB
	dir string
}

// NewMediaService constructs a MediaService writing into dir.
func NewMediaService(db *sql.DB, dir string) (*MediaService, error) {
	if dir == "" {
		dir = "data/uploads"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create uploads dir: %w", err)
	}
	return &MediaService{db: db, dir: dir}, nil
}

const mediaColumns = `id, stored_name, original_name, mime, byte_size, created_at`

func scanMedia(scanner interface {
	Scan(dest ...any) error
}) (models.MediaAsset, error) {
	var m models.MediaAsset
	err := scanner.Scan(
		&m.ID, &m.StoredName, &m.OriginalName, &m.Mime, &m.ByteSize, &m.CreatedAt,
	)
	if err != nil {
		return models.MediaAsset{}, err
	}
	m.URL = fmt.Sprintf("/media/%d", m.ID)
	return m, nil
}

// List returns all media assets, newest first.
func (s *MediaService) List() ([]models.MediaAsset, error) {
	rows, err := s.db.Query(
		`SELECT ` + mediaColumns + ` FROM media_assets ORDER BY id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.MediaAsset, 0)
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			return nil, fmt.Errorf("scan media: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media: %w", err)
	}
	return out, nil
}

// GetByID returns media metadata.
func (s *MediaService) GetByID(id int64) (models.MediaAsset, error) {
	row := s.db.QueryRow(`SELECT `+mediaColumns+` FROM media_assets WHERE id = ?`, id)
	m, err := scanMedia(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.MediaAsset{}, ErrNotFound
	}
	if err != nil {
		return models.MediaAsset{}, fmt.Errorf("get media: %w", err)
	}
	return m, nil
}

// FilePath returns the on-disk path for a media asset.
func (s *MediaService) FilePath(m models.MediaAsset) string {
	return filepath.Join(s.dir, m.StoredName)
}

// Create stores a new upload from r and inserts metadata.
// MIME type is determined by sniffing file bytes (http.DetectContentType), not
// contentType or the filename extension. contentType is retained for call-site
// compatibility but ignored for acceptance and storage.
func (s *MediaService) Create(originalName, contentType string, r io.Reader, size int64) (models.MediaAsset, error) {
	_ = contentType
	if size <= 0 {
		return models.MediaAsset{}, fmt.Errorf("%w: empty file", ErrValidation)
	}
	if size > MaxUploadBytes {
		return models.MediaAsset{}, fmt.Errorf("%w: file exceeds 20 MiB limit", ErrValidation)
	}

	peek := make([]byte, 512)
	n, err := io.ReadFull(r, peek)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return models.MediaAsset{}, fmt.Errorf("read upload: %w", err)
	}
	peek = peek[:n]
	mimeType := strings.ToLower(strings.Split(http.DetectContentType(peek), ";")[0])
	if _, ok := allowedUploadMimes[mimeType]; !ok {
		return models.MediaAsset{}, fmt.Errorf("%w: unsupported media type %q", ErrValidation, mimeType)
	}
	r = io.MultiReader(bytes.NewReader(peek), r)

	safe := SanitizeFilename(originalName)
	ext := filepath.Ext(safe)
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			ext = exts[0]
			safe += ext
		}
	}

	// Insert a placeholder row to get an id, then rename the file.
	res, err := s.db.Exec(
		`INSERT INTO media_assets (stored_name, original_name, mime, byte_size)
		 VALUES (?, ?, ?, ?)`,
		"pending", originalName, mimeType, size,
	)
	if err != nil {
		return models.MediaAsset{}, fmt.Errorf("insert media: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.MediaAsset{}, fmt.Errorf("media id: %w", err)
	}

	stored := fmt.Sprintf("%d_%s", id, safe)
	path := filepath.Join(s.dir, stored)
	f, err := os.Create(path)
	if err != nil {
		_, _ = s.db.Exec(`DELETE FROM media_assets WHERE id = ?`, id)
		return models.MediaAsset{}, fmt.Errorf("create media file: %w", err)
	}
	written, copyErr := io.Copy(f, io.LimitReader(r, MaxUploadBytes+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || written > MaxUploadBytes {
		_ = os.Remove(path)
		_, _ = s.db.Exec(`DELETE FROM media_assets WHERE id = ?`, id)
		if written > MaxUploadBytes {
			return models.MediaAsset{}, fmt.Errorf("%w: file exceeds 20 MiB limit", ErrValidation)
		}
		if copyErr != nil {
			return models.MediaAsset{}, fmt.Errorf("write media file: %w", copyErr)
		}
		return models.MediaAsset{}, fmt.Errorf("close media file: %w", closeErr)
	}

	_, err = s.db.Exec(
		`UPDATE media_assets SET stored_name = ?, byte_size = ? WHERE id = ?`,
		stored, written, id,
	)
	if err != nil {
		_ = os.Remove(path)
		_, _ = s.db.Exec(`DELETE FROM media_assets WHERE id = ?`, id)
		return models.MediaAsset{}, fmt.Errorf("finalize media: %w", err)
	}
	return s.GetByID(id)
}

// Delete removes media metadata and the on-disk file.
func (s *MediaService) Delete(id int64) error {
	m, err := s.GetByID(id)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM media_assets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete media: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete media rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	_ = os.Remove(s.FilePath(m))
	return nil
}

// URLForID returns /media/{id} if the asset exists, else empty.
func (s *MediaService) URLForID(id *int64) string {
	if id == nil || *id <= 0 {
		return ""
	}
	if _, err := s.GetByID(*id); err != nil {
		return ""
	}
	return fmt.Sprintf("/media/%d", *id)
}

// IsPubliclyReferenced reports whether media id is safe to serve anonymously:
// a published studio piece image, the resume PDF, or a digit-bounded /media/{id}
// reference in a published post's content_md or content_html.
func (s *MediaService) IsPubliclyReferenced(id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	idStr := strconv.FormatInt(id, 10)
	var n int
	err := s.db.QueryRow(`
		SELECT 1 WHERE EXISTS (
			SELECT 1 FROM studio_pieces WHERE image_media_id = ? AND published = 1
		) OR EXISTS (
			SELECT 1 FROM pages
			WHERE slug = 'resume' AND json_extract(body_json, '$.pdf_media_id') = ?
		) OR EXISTS (
			SELECT 1 FROM posts WHERE published = 1 AND (
				content_md GLOB ('*/media/' || ? || '[^0-9]*')
				OR content_md GLOB ('*/media/' || ?)
				OR content_html GLOB ('*/media/' || ? || '[^0-9]*')
				OR content_html GLOB ('*/media/' || ?)
			)
		)`, id, id, idStr, idStr, idStr, idStr,
	).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is publicly referenced: %w", err)
	}
	return true, nil
}

// SanitizeFilename returns a filesystem-safe base name for uploads and
// Content-Disposition headers.
func SanitizeFilename(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, " ", "-")
	var b strings.Builder
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" || out == "." {
		return "upload"
	}
	return out
}
