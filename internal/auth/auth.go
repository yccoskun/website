// Package auth provides password checking and session cookie helpers.
package auth

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const maxUsernameLength = 256

// SessionCookieName is the HTTP cookie that carries the raw session token.
const SessionCookieName = "session"

// ConstantTimeUsernameEqual reports whether two usernames match without leaking
// timing information via early returns on content (over-max inputs return false).
func ConstantTimeUsernameEqual(a, b string) bool {
	if len(a) > maxUsernameLength || len(b) > maxUsernameLength {
		return false
	}

	lenOK := constantTimeIntEq(len(a), len(b))

	var bufA, bufB [maxUsernameLength]byte
	copy(bufA[:], a)
	copy(bufB[:], b)

	cmpOK := subtle.ConstantTimeCompare(bufA[:], bufB[:])
	return subtle.ConstantTimeSelect(lenOK, cmpOK, 0) == 1
}

func constantTimeIntEq(x, y int) int {
	xb := [2]byte{byte(x), byte(x >> 8)}
	yb := [2]byte{byte(y), byte(y >> 8)}
	return subtle.ConstantTimeCompare(xb[:], yb[:])
}

// CheckPassword compares a bcrypt hash against a plaintext password.
func CheckPassword(hash, password string) bool {
	if hash == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SetSessionCookie writes the session cookie on the response.
// SameSite=Lax blocks cross-site POSTs. Authenticated admin APIs also reject
// Sec-Fetch-Site: cross-site; sensitive export is POST-only.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		Expires:  expires,
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	})
}

// SessionToken reads the raw session token from the request cookie.
func SessionToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func cookieSecure(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return host != "localhost" && host != "127.0.0.1" && host != "::1"
}
