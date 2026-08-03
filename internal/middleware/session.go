package middleware

import (
	"net/http"
	"strings"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/response"
	"github.com/yccoskun/website/internal/securitylog"
	"github.com/yccoskun/website/internal/services"
)

// allowedSecFetchSite reports whether Sec-Fetch-Site is permitted for
// authenticated admin APIs. Empty (missing) is allowed for non-browser
// clients (curl, tests, scripts). Browsers send one of same-origin,
// same-site, or none for legitimate same-site navigation/fetch; any other
// value (including cross-site and garbage) is rejected.
func allowedSecFetchSite(site string) bool {
	switch strings.ToLower(site) {
	case "", "same-origin", "same-site", "none":
		return true
	default:
		return false
	}
}

// RequireSession rejects requests without a valid admin session cookie.
// Sets Cache-Control: private, no-store on every response (including
// 401/403) before session checks. Sec-Fetch-Site must be missing or one
// of: same-origin, same-site, none (case-insensitive). Any other value
// (including cross-site) yields 403 before session validation. Export and
// other requireAuth routes share this check; there is no second Sec-Fetch
// wrapper on export. Login and logout are not wrapped by RequireSession.
//
// When bindingEnabled is true and the session was created with both UA and
// IP-prefix hashes, a mismatch destroys the session, clears the cookie, and
// returns 401 with message reauth_required.
func RequireSession(sessions *services.SessionService, bindingEnabled bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authenticated admin JSON must not be stored by shared caches.
		w.Header().Set("Cache-Control", "private, no-store")
		if !allowedSecFetchSite(r.Header.Get("Sec-Fetch-Site")) {
			response.Error(w, http.StatusForbidden, "forbidden")
			return
		}
		if sessions == nil {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		token := auth.SessionToken(r)
		ok, mismatch, err := sessions.Validate(token, r.UserAgent(), ClientIP(r), bindingEnabled)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		if mismatch {
			_ = sessions.Destroy(token)
			auth.ClearSessionCookie(w, r)
			securitylog.Event(securitylog.EventSessionBindingMismatch, ClientIP(r))
			response.Error(w, http.StatusUnauthorized, "reauth_required")
			return
		}
		if !ok {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}
