package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/yccoskun/website/internal/auth"
)

// DefaultSessionTTL is the absolute lifetime of an admin session (no idle refresh).
const DefaultSessionTTL = 24 * time.Hour

// SessionService manages admin session tokens.
type SessionService struct {
	db  *sql.DB
	ttl time.Duration
}

// NewSessionService constructs a SessionService with DefaultSessionTTL.
func NewSessionService(db *sql.DB) *SessionService {
	return NewSessionServiceWithTTL(db, DefaultSessionTTL)
}

// NewSessionServiceWithTTL constructs a SessionService with a custom absolute TTL.
// Useful in tests (short or already-expired TTLs).
func NewSessionServiceWithTTL(db *sql.DB, ttl time.Duration) *SessionService {
	return &SessionService{db: db, ttl: ttl}
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Create issues a new session and returns the raw cookie token.
// When bindingEnabled is false, ua_hash and ip_prefix_hash are stored as NULL
// (expiry-only sessions). When true, both SHA-256 hex hints are required and
// stored; missing UA or unparseable client IP fails closed with ErrValidation.
func (s *SessionService) Create(ua, clientIP string, bindingEnabled bool) (rawToken string, expiresAt time.Time, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	raw := hex.EncodeToString(buf)
	expires := time.Now().UTC().Add(s.ttl)
	expiresStr := expires.Format("2006-01-02T15:04:05.000Z")

	var uaArg, ipArg any
	if bindingEnabled {
		uaHash := auth.HashUA(ua)
		ipHash := auth.HashIPPrefix(clientIP)
		if uaHash == "" || ipHash == "" {
			return "", time.Time{}, fmt.Errorf("%w: session binding requires user-agent and client IP", ErrValidation)
		}
		uaArg = uaHash
		ipArg = ipHash
	}

	_, err = s.db.Exec(
		`INSERT INTO sessions (token_hash, expires_at, ua_hash, ip_prefix_hash) VALUES (?, ?, ?, ?)`,
		hashToken(raw), expiresStr, uaArg, ipArg,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return raw, expires, nil
}

// Validate checks that the raw cookie token maps to a non-expired session.
// When bindingEnabled and both stored binding hashes are present, either
// UA or IP-prefix mismatch yields ok=false, mismatch=true. Flag off or
// NULL/empty stored hashes → expiry-only (legacy behavior).
func (s *SessionService) Validate(rawToken, ua, clientIP string, bindingEnabled bool) (ok bool, mismatch bool, err error) {
	if rawToken == "" {
		return false, false, nil
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	var expiresAt string
	var storedUA, storedIP sql.NullString
	scanErr := s.db.QueryRow(
		`SELECT expires_at, ua_hash, ip_prefix_hash FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		hashToken(rawToken), now,
	).Scan(&expiresAt, &storedUA, &storedIP)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return false, false, nil
	}
	if scanErr != nil {
		return false, false, fmt.Errorf("validate session: %w", scanErr)
	}

	if !bindingEnabled || !storedUA.Valid || storedUA.String == "" || !storedIP.Valid || storedIP.String == "" {
		return true, false, nil
	}

	wantUA := auth.HashUA(ua)
	wantIP := auth.HashIPPrefix(clientIP)
	uaOK := subtle.ConstantTimeCompare([]byte(wantUA), []byte(storedUA.String)) == 1
	ipOK := subtle.ConstantTimeCompare([]byte(wantIP), []byte(storedIP.String)) == 1
	if !uaOK || !ipOK {
		return false, true, nil
	}
	return true, false, nil
}

// Destroy removes the session for the given raw token.
func (s *SessionService) Destroy(rawToken string) error {
	if rawToken == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(rawToken))
	if err != nil {
		return fmt.Errorf("destroy session: %w", err)
	}
	return nil
}

// DestroyExpired deletes sessions past their expiry.
func (s *SessionService) DestroyExpired() error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now)
	if err != nil {
		return fmt.Errorf("destroy expired sessions: %w", err)
	}
	return nil
}
