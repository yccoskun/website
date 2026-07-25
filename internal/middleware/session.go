package middleware

import (
	"net/http"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/response"
	"github.com/yccoskun/website/internal/services"
)

// RequireSession rejects requests without a valid admin session cookie.
func RequireSession(sessions *services.SessionService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
