package services

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
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
func (s *SessionService) Create() (rawToken string, expiresAt time.Time, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	raw := hex.EncodeToString(buf)
	expires := time.Now().UTC().Add(s.ttl)
	expiresStr := expires.Format("2006-01-02T15:04:05.000Z")

	_, err = s.db.Exec(
		`INSERT INTO sessions (token_hash, expires_at) VALUES (?, ?)`,
		hashToken(raw), expiresStr,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return raw, expires, nil
}

// Validate checks that the raw cookie token maps to a non-expired session.
func (s *SessionService) Validate(rawToken string) (bool, error) {
	if rawToken == "" {
		return false, nil
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	var expiresAt string
	err := s.db.QueryRow(
		`SELECT expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		hashToken(rawToken), now,
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("validate session: %w", err)
	}
	return true, nil
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
