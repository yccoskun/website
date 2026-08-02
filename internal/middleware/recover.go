package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/yccoskun/website/internal/response"
)

// Recover catches panics from downstream handlers, logs the panic and stack
// server-side only, and returns a generic 500 to the client.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			log.Printf("panic: %v\n%s", rec, debug.Stack())
			if strings.HasPrefix(r.URL.Path, "/api/") {
				response.Error(w, http.StatusInternalServerError, "internal error")
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
