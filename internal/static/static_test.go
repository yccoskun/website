package static

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingAssetReturns404(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>ok")
	h := mustHandler(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/assets/missing-hash.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("body = %q, want non-SPA 404", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestAssetCacheControlImmutable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>ok")
	assetDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(assetDir, "app.js"), "console.log(1)")
	h := mustHandler(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	got := rec.Header().Get("Cache-Control")
	want := "public, max-age=31536000, immutable"
	if got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}

func TestSPAFallbackNoCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>ok")
	h := mustHandler(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/blog/some-post", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("body = %q, want index.html SPA fallback", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestIndexNoCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>ok")
	h := mustHandler(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestResolveOverrideEmptyDirUsesEmbed(t *testing.T) {
	for _, allow := range []bool{false, true} {
		got, err := ResolveOverride("", allow)
		if err != nil {
			t.Fatalf("allow=%v: err = %v, want nil", allow, err)
		}
		if got != "" {
			t.Fatalf("allow=%v: dir = %q, want empty", allow, got)
		}
	}
}

func TestResolveOverrideRequiresAllowFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>ok")

	_, err := ResolveOverride(dir, false)
	if err == nil {
		t.Fatal("err = nil, want error when ALLOW_STATIC_DIR is not set")
	}
	if !strings.Contains(err.Error(), "ALLOW_STATIC_DIR") {
		t.Fatalf("err = %v, want mention of ALLOW_STATIC_DIR", err)
	}
}

func TestResolveOverrideRejectsPathOutsideCwd(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)

	if _, err := ResolveOverride("/", true); err == nil {
		t.Fatal("err = nil, want error for path outside cwd")
	}
}

func TestResolveOverrideRejectsCwdItself(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "index.html"), "<!doctype html>ok")
	t.Chdir(cwd)

	_, err := ResolveOverride(".", true)
	if err == nil {
		t.Fatal("err = nil, want error when STATIC_DIR resolves to cwd")
	}
	if !strings.Contains(err.Error(), "subdirectory") {
		t.Fatalf("err = %v, want subdirectory confinement message", err)
	}
}

func TestResolveOverrideAcceptsNestedDirUnderCwd(t *testing.T) {
	parent := t.TempDir()
	nested := filepath.Join(parent, "web", "dist")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nested, "index.html"), "<!doctype html>ok")
	t.Chdir(parent)

	got, err := ResolveOverride("web/dist", true)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	resolvedNested, err := filepath.EvalSymlinks(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != resolvedNested {
		t.Fatalf("dir = %q, want %q", got, resolvedNested)
	}
}

func TestResolveOverrideRequiresIndexHTML(t *testing.T) {
	parent := t.TempDir()
	nested := filepath.Join(parent, "web", "dist")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(parent)

	if _, err := ResolveOverride("web/dist", true); err == nil {
		t.Fatal("err = nil, want error for missing index.html")
	}
}

func TestResolveOverrideRejectsSymlinkEscapingCwd(t *testing.T) {
	parent := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "index.html"), "<!doctype html>ok")

	link := filepath.Join(parent, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	t.Chdir(parent)

	if _, err := ResolveOverride("escape", true); err == nil {
		t.Fatal("err = nil, want error for symlink escaping cwd")
	}
}

func TestHandlerRejectsNestedSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>ok")

	secretDir := t.TempDir()
	writeFile(t, filepath.Join(secretDir, "secret.txt"), "top-secret")

	link := filepath.Join(dir, "leak")
	if err := os.Symlink(secretDir, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	h := mustHandler(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/leak/secret.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "top-secret") {
		t.Fatalf("nested symlink escaped rooted FS: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func mustHandler(t *testing.T, dir string) http.Handler {
	t.Helper()
	h, err := Handler(dir)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
