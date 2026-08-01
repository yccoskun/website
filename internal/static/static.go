// Package static serves the built frontend SPA. By default it serves the
// build embedded at compile time from dist/. Setting STATIC_DIR serves a
// directory on disk instead (useful in development), but only takes effect
// when ALLOW_STATIC_DIR opts in, and the directory must resolve to a
// subdirectory of the process working directory. When no build is present
// in dist/ (fresh clone), a tracked placeholder page is served.
package static

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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

// ResolveOverride validates a requested static directory override and
// returns its resolved absolute path, or ("", nil) to use the embedded
// build. dir being empty always means "use embed", regardless of allow.
// Otherwise allow must be true (ALLOW_STATIC_DIR), and dir must resolve to
// a strict descendant of the process working directory (not the cwd
// itself), containing an index.html file. Rejecting cwd equality avoids
// accidentally exposing sibling trees such as data/ under WorkingDirectory.
func ResolveOverride(dir string, allow bool) (string, error) {
	if dir == "" {
		return "", nil
	}
	if !allow {
		return "", errors.New("STATIC_DIR is set but ALLOW_STATIC_DIR is not enabled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve STATIC_DIR: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve STATIC_DIR: %w", err)
	}

	rel, err := filepath.Rel(cwd, resolved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("STATIC_DIR %q must be a subdirectory of the working directory %q", dir, cwd)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat STATIC_DIR: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("STATIC_DIR %q is not a directory", dir)
	}

	indexInfo, err := os.Stat(filepath.Join(resolved, "index.html"))
	if err != nil {
		return "", fmt.Errorf("STATIC_DIR %q has no index.html: %w", dir, err)
	}
	if !indexInfo.Mode().IsRegular() {
		return "", fmt.Errorf("STATIC_DIR %q index.html is not a regular file", dir)
	}

	return resolved, nil
}

// Handler returns an http.Handler for the SPA. Existing files are served
// as-is; every other path falls back to index.html so client-side routing
// works on hard refresh and deep links. Missing /assets/* paths return 404
// (never the SPA shell) so hashed asset URLs fail closed.
//
// When dirOverride is non-empty it is opened with os.OpenRoot so nested
// symlinks cannot escape the tree at request time.
func Handler(dirOverride string) (http.Handler, error) {
	if dirOverride != "" {
		root, err := os.OpenRoot(dirOverride)
		if err != nil {
			return nil, fmt.Errorf("open STATIC_DIR: %w", err)
		}
		return spaHandler(root.FS()), nil
	}
	return spaHandler(embeddedFS()), nil
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
