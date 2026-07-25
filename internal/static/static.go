// Package static serves the built frontend SPA. By default it serves the
// build embedded at compile time from dist/; setting STATIC_DIR serves a
// directory on disk instead (useful in development). When no build is
// present in dist/ (fresh clone), a tracked placeholder page is served.
package static

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

// dist/ is fully gitignored except .gitkeep, which guarantees the embed
// pattern always matches at least one file even on a fresh clone.
//
//go:embed all:dist
var distFS embed.FS

// placeholder/ is tracked in git and never touched by the frontend build,
// so deploy copies of web/dist into dist/ cannot dirty the work tree.
//
//go:embed placeholder
var placeholderFS embed.FS

// Handler returns an http.Handler for the SPA. Existing files are served
// as-is; every other path falls back to index.html so client-side routing
// works on hard refresh and deep links. Missing /assets/* paths return 404
// (never the SPA shell) so hashed asset URLs fail closed.
func Handler(dirOverride string) http.Handler {
	if dirOverride != "" {
		return spaHandler(os.DirFS(dirOverride))
	}
	return spaHandler(embeddedFS())
}

func embeddedFS() fs.FS {
	dist := mustSub(distFS, "dist")
	if fileExists(dist, "index.html") {
		return dist
	}
	return mustSub(placeholderFS, "placeholder")
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		// Unreachable: both directories are embedded at compile time.
		panic(err)
	}
	return sub
}

func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(name, "assets/") {
			if !fileExists(fsys, name) {
				w.Header().Set("Cache-Control", "no-store")
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			fileServer.ServeHTTP(w, r)
			return
		}
		if name != "" && !fileExists(fsys, name) {
			r.URL.Path = "/"
		}
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
