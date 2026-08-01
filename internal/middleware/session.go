package middleware

import (
	"net/http"
	"strings"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/response"
	"github.com/yccoskun/website/internal/services"
)

// RequireSession rejects requests without a valid admin session cookie.
// Requests with Sec-Fetch-Site: cross-site are rejected (403) to block
// cross-site cookie use on authenticated admin APIs. Missing Sec-Fetch-Site
// is allowed (curl, tests, older clients).
func RequireSession(sessions *services.SessionService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if site := r.Header.Get("Sec-Fetch-Site"); strings.EqualFold(site, "cross-site") {
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
