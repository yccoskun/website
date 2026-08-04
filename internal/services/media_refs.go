package services

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
)

var mediaPathIDPattern = regexp.MustCompile(`/media/(\d+)`)

// ExtractMediaIDs returns the union of media IDs referenced as /media/{id}
// across the given content strings. Digits are greedy, so /media/12 is id 12,
// not id 1.
func ExtractMediaIDs(contents ...string) []int64 {
	seen := make(map[int64]struct{})
	out := make([]int64, 0)
	for _, content := range contents {
		for _, m := range mediaPathIDPattern.FindAllStringSubmatch(content, -1) {
			if len(m) < 2 {
				continue
			}
			id, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// syncMediaReferences replaces media_references rows for postID.
// Invariant: rows exist only for published posts (IsPubliclyReferenced also
// joins posts.published as defense in depth).
func syncMediaReferences(q dbQuerier, postID int64, published bool, contentMD, contentHTML string) error {
	if _, err := q.Exec(`DELETE FROM media_references WHERE post_id = ?`, postID); err != nil {
		return fmt.Errorf("clear media references: %w", err)
	}
	if !published {
		return nil
	}
	for _, mediaID := range ExtractMediaIDs(contentMD, contentHTML) {
		_, err := q.Exec(`
			INSERT INTO media_references (post_id, media_id)
			SELECT ?, id FROM media_assets WHERE id = ?`,
			postID, mediaID,
		)
		if err != nil {
			return fmt.Errorf("insert media reference: %w", err)
		}
	}
	return nil
}

// BackfillMediaReferences rebuilds media_references from all published posts.
// Safe to call repeatedly after Migrate.
func BackfillMediaReferences(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin media references backfill: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM media_references`); err != nil {
		return fmt.Errorf("clear media references: %w", err)
	}

	rows, err := tx.Query(
		`SELECT id, content_md, content_html FROM posts WHERE published = 1`,
	)
	if err != nil {
		return fmt.Errorf("list published posts for media refs: %w", err)
	}

	type postBody struct {
		id                     int64
		contentMD, contentHTML string
	}
	posts := make([]postBody, 0)
	for rows.Next() {
		var p postBody
		if err := rows.Scan(&p.id, &p.contentMD, &p.contentHTML); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan post for media refs: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate posts for media refs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close posts for media refs: %w", err)
	}

	for _, p := range posts {
		if err := syncMediaReferences(tx, p.id, true, p.contentMD, p.contentHTML); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit media references backfill: %w", err)
	}
	return nil
}
