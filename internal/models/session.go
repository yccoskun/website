package models

// Session is a server-side admin session row. The cookie carries the raw
// token; only its SHA-256 hex hash is stored.
type Session struct {
	TokenHash string `json:"-"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}
