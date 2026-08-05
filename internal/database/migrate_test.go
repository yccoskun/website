package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migrate.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestPublishedIndexesMigration(t *testing.T) {
	const migrationName = "migrations/0009_published_indexes.sql"

	db := openTestDB(t)

	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, migrationName,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_migrations count for %s = %d, want 1", migrationName, count)
	}

	cases := []struct {
		name  string
		query string
		index string
	}{
		{
			name: "posts published list",
			query: `SELECT id FROM posts WHERE published = 1
				ORDER BY COALESCE(published_at, created_at) DESC, id DESC`,
			index: "idx_posts_published_list",
		},
		{
			name: "studio pieces published list",
			query: `SELECT id FROM studio_pieces WHERE published = 1
				ORDER BY sort_order ASC, id ASC`,
			index: "idx_studio_pieces_published_list",
		},
		{
			name:  "studio pieces image media",
			query: `SELECT 1 FROM studio_pieces WHERE image_media_id = 1 AND published = 1`,
			index: "idx_studio_pieces_image_media",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := explainQueryPlan(t, db, tc.query)
			if !strings.Contains(plan, tc.index) {
				t.Fatalf("EXPLAIN QUERY PLAN missing %q\nplan:\n%s", tc.index, plan)
			}
		})
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	err = db.QueryRow(
		`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, migrationName,
	).Scan(&count)
	if err != nil {
		t.Fatalf("requery schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("after second migrate, schema_migrations count for %s = %d, want 1", migrationName, count)
	}
}

func explainQueryPlan(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN " + query)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		parts = append(parts, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	return strings.Join(parts, "\n")
}
