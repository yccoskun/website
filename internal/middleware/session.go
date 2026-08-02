package middleware

import (
	"net/http"
	"strings"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/response"
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
// Sec-Fetch-Site must be missing or one of: same-origin, same-site, none
// (case-insensitive). Any other value (including cross-site) yields 403
// before session validation. Export and other requireAuth routes share this
// check; there is no second Sec-Fetch wrapper on export. Login and logout
// are not wrapped by RequireSession.
func RequireSession(sessions *services.SessionService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedSecFetchSite(r.Header.Get("Sec-Fetch-Site")) {
			response.Error(w, http.StatusForbidden, "forbidden")
			return
		}
		if sessions == nil {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ok, err := sessions.Validate(auth.SessionToken(r))
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !ok {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}
