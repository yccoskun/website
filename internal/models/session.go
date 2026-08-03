package models

// Session is a server-side admin session row. The cookie carries the raw
// token; only its SHA-256 hex hash is stored. UAHash and IPPrefixHash are
// optional soft-binding hints (SHA-256 hex); empty means unbound / legacy.
type Session struct {
	TokenHash    string `json:"-"`
	UAHash       string `json:"-"`
	IPPrefixHash string `json:"-"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
}
