// Package database opens the SQLite database and runs schema migrations.
package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (creating if necessary) the SQLite database at path.
//
// Pragmas are passed as DSN parameters so that every pooled connection gets
// them: journal_mode is persistent in the file, but busy_timeout,
// synchronous and foreign_keys are per-connection.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	pragmas := url.Values{}
	pragmas.Add("_pragma", "journal_mode(WAL)")
	pragmas.Add("_pragma", "busy_timeout(5000)")
	pragmas.Add("_pragma", "synchronous(NORMAL)")
	pragmas.Add("_pragma", "foreign_keys(ON)")

	dsn := fmt.Sprintf("file:%s?%s", path, pragmas.Encode())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	// Personal-site traffic: one connection serializes access and eliminates
	// SQLITE_BUSY paths. WAL + busy_timeout remain as a safety net. Revisit
	// with a read pool if concurrent reads ever matter.
	db.SetMaxOpenConns(1)
	return db, nil
}
