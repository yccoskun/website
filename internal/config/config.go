// Package config loads server configuration from environment variables.
package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// TrustedProxies is the parsed TRUSTED_PROXIES allowlist. Empty means no
// proxy is trusted (secure default): CF-Connecting-IP is ignored.
type TrustedProxies struct {
	Nets []*net.IPNet
	Unix bool
}

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
	// SessionBinding enables soft session binding (UA + IP prefix hashes).
	// Off by default; set SESSION_BINDING=1|true|yes to enable.
	SessionBinding bool
	// TrustedProxies is the allowlist of peers permitted to present
	// CF-Connecting-IP. Empty means no proxy trust.
	TrustedProxies TrustedProxies
}

// Load reads configuration from the environment, applying defaults.
// Invalid TRUSTED_PROXIES entries cause a non-nil error.
func Load() (Config, error) {
	tp, err := ParseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, err
	}
	return Config{
		Addr:              envOr("ADDR", "127.0.0.1:9000"),
		DBPath:            envOr("DB_PATH", "data/website.db"),
		UploadsDir:        envOr("UPLOADS_DIR", "data/uploads"),
		StaticDir:         os.Getenv("STATIC_DIR"),
		AllowStaticDir:    envBool("ALLOW_STATIC_DIR"),
		SiteURL:           strings.TrimRight(envOr("SITE_URL", "https://www.yusufcancoskun.com"), "/"),
		AdminUsername:     os.Getenv("ADMIN_USERNAME"),
		AdminPasswordHash: os.Getenv("ADMIN_PASSWORD_HASH"),
		SessionBinding:    envBool("SESSION_BINDING"),
		TrustedProxies:    tp,
	}, nil
}

// ParseTrustedProxies parses a comma-separated TRUSTED_PROXIES value.
// Tokens are CIDRs (e.g. 127.0.0.1/32, 127.0.0.0/8, ::1/128), bare IPs
// (normalized to /32 or /128), or the literal "unix" for Unix-domain peers.
// Empty input yields an empty allowlist (no proxy trust).
func ParseTrustedProxies(s string) (TrustedProxies, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return TrustedProxies{}, nil
	}

	var out TrustedProxies
	for _, raw := range strings.Split(s, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			return TrustedProxies{}, fmt.Errorf("trusted proxies: empty token in %q", s)
		}
		if strings.EqualFold(tok, "unix") {
			out.Unix = true
			continue
		}

		cidr := tok
		if !strings.Contains(tok, "/") {
			ip := net.ParseIP(tok)
			if ip == nil {
				return TrustedProxies{}, fmt.Errorf("trusted proxies: invalid IP %q", tok)
			}
			if ip.To4() != nil {
				cidr = ip.String() + "/32"
			} else {
				cidr = ip.String() + "/128"
			}
		}

		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return TrustedProxies{}, fmt.Errorf("trusted proxies: invalid CIDR %q: %w", tok, err)
		}
		out.Nets = append(out.Nets, network)
	}
	return out, nil
}

// ShouldWarnMissingSessionBinding reports whether trusted proxies are
// configured (Nets non-empty or Unix) while session binding is off.
func ShouldWarnMissingSessionBinding(sessionBinding bool, tp TrustedProxies) bool {
	return !sessionBinding && (len(tp.Nets) > 0 || tp.Unix)
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
