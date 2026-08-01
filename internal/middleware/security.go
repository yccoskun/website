package middleware

import "net/http"

// SecurityHeaders adds baseline browser security headers, including a
// restrictive Content-Security-Policy (default-src/script-src/style-src
// 'self'; no unsafe-inline/unsafe-eval). The theme boot inline script is
// allowed only via script-src 'sha256-...'. HSTS is left to Caddy.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'sha256-d09nNKWklfcIveyWn6g0V92ntPlklT3aFfWoUppLn4Q='; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
		)
		next.ServeHTTP(w, r)
	})
}
