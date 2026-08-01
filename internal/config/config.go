// Package config loads server configuration from environment variables.
package config

import (
	"os"
	"strings"
)

// Config holds all runtime settings for the server.
type Config struct {
	// Addr is the listen address, e.g. "127.0.0.1:9000".
	Addr string
	// DBPath is the filesystem path to the SQLite database file.
	DBPath string
	// UploadsDir is where admin media uploads are stored on disk.
	UploadsDir string
	// StaticDir, when non-empty, serves the frontend from this directory
	// on disk instead of the embedded build. Dev-only: ignored unless
	// AllowStaticDir is also set, and must resolve to a subdirectory of
	// the process cwd (not the cwd itself).
	StaticDir string
	// AllowStaticDir opts in to honoring StaticDir. Production should leave
	// this unset so the server always serves the embedded frontend build.
	AllowStaticDir bool
	// SiteURL is the canonical public origin (no trailing slash), used in
	// robots.txt, sitemap, and RSS links.
	SiteURL string
	// AdminUsername is the expected login username. Empty disables login.
	AdminUsername string
	// AdminPasswordHash is a bcrypt hash of the admin password. Empty disables login.
	AdminPasswordHash string
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		Addr:              envOr("ADDR", "127.0.0.1:9000"),
		DBPath:            envOr("DB_PATH", "data/website.db"),
		UploadsDir:        envOr("UPLOADS_DIR", "data/uploads"),
		StaticDir:         os.Getenv("STATIC_DIR"),
		AllowStaticDir:    envBool("ALLOW_STATIC_DIR"),
		SiteURL:           strings.TrimRight(envOr("SITE_URL", "https://www.yusufcancoskun.com"), "/"),
		AdminUsername:     os.Getenv("ADMIN_USERNAME"),
		AdminPasswordHash: os.Getenv("ADMIN_PASSWORD_HASH"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool reports whether the named environment variable is set to a truthy
// value ("1", "true", or "yes", case-insensitive).
func envBool(key string) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}
