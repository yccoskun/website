package services_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yccoskun/website/internal/services"
)

// Minimal magic-byte fixtures for http.DetectContentType.
var (
	magicPNG  = []byte("\x89PNG\r\n\x1a\n")
	magicJPEG = []byte("\xff\xd8\xff\xe0\x00\x10JFIF")
	magicGIF  = []byte("GIF89a")
	magicWebP = []byte("RIFF\x08\x00\x00\x00WEBPVP")
	magicPDF  = []byte("%PDF-1.4")
)

func TestMediaCreateSniffsAllowedTypes(t *testing.T) {
	db := openCMSDB(t)
	uploads := filepath.Join(t.TempDir(), "up")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}

	cases := []struct {
		name     string
		filename string
		clientCT string
		data     []byte
		wantMIME string
	}{
		{"jpeg", "photo.jpg", "application/octet-stream", magicJPEG, "image/jpeg"},
		{"png", "still.png", "text/plain", magicPNG, "image/png"},
		{"gif", "anim.gif", "image/jpeg", magicGIF, "image/gif"},
		{"webp", "shot.webp", "image/png", magicWebP, "image/webp"},
		{"pdf", "cv.pdf", "image/png", magicPDF, "application/pdf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asset, err := media.Create(tc.filename, tc.clientCT, bytes.NewReader(tc.data), int64(len(tc.data)))
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if asset.Mime != tc.wantMIME {
				t.Fatalf("Mime = %q, want %q", asset.Mime, tc.wantMIME)
			}
		})
	}
}

func TestMediaCreateRejectsMislabeled(t *testing.T) {
	db := openCMSDB(t)
	uploads := filepath.Join(t.TempDir(), "up")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}

	cases := []struct {
		name string
		data []byte
	}{
		{"html", []byte("<!DOCTYPE html><html>")},
		{"exe-mz", []byte("MZ\x90\x00")},
		{"plain", []byte("not-an-image")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := media.Create("evil.png", "image/png", bytes.NewReader(tc.data), int64(len(tc.data)))
			if err == nil || !errors.Is(err, services.ErrValidation) {
				t.Fatalf("want ErrValidation, got %v", err)
			}
		})
	}
}

func TestMediaCreateIgnoresClientContentType(t *testing.T) {
	db := openCMSDB(t)
	uploads := filepath.Join(t.TempDir(), "up")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}
	asset, err := media.Create("renamed.bin", "application/x-msdownload", bytes.NewReader(magicPNG), int64(len(magicPNG)))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if asset.Mime != "image/png" {
		t.Fatalf("Mime = %q, want image/png", asset.Mime)
	}
}

func TestMediaRejectsBadType(t *testing.T) {
	db := openCMSDB(t)
	uploads := filepath.Join(t.TempDir(), "up")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}
	_, err = media.Create("x.exe", "application/x-msdownload", bytes.NewReader([]byte("MZ")), 2)
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "upload"},
		{".", "upload"},
		{"..", "upload"},
		{"...", "upload"},
		{"photo.png", "photo.png"},
		{"my photo.png", "my-photo.png"},
		{`evil"name.pdf`, "evilname.pdf"},
		{"evil\r\ninject.pdf", "evilinject.pdf"},
		{"../../evil.pdf", "evil.pdf"},
		{"/tmp/path/seg.pdf", "seg.pdf"},
		{`path\seg.pdf`, "pathseg.pdf"},
		{"name;inject.pdf", "nameinject.pdf"},
		{"nul\x00byte.pdf", "nulbyte.pdf"},
		{"café.png", "caf.png"},
		{"!!!", "upload"},
	}
	for _, tc := range cases {
		got := services.SanitizeFilename(tc.in)
		if got != tc.want {
			t.Fatalf("SanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if strings.ContainsAny(got, "\"\r\n/\\\x00;") {
			t.Fatalf("SanitizeFilename(%q) = %q still unsafe", tc.in, got)
		}
		for _, r := range got {
			if r > 127 {
				t.Fatalf("SanitizeFilename(%q) = %q has non-ASCII", tc.in, got)
			}
		}
	}
}

func TestContentDisposition(t *testing.T) {
	cases := []struct {
		mime, name, want string
	}{
		{"image/jpeg", "photo.jpg", `inline; filename="photo.jpg"`},
		{"image/png", "still.png", `inline; filename="still.png"`},
		{"image/gif", "anim.gif", `inline; filename="anim.gif"`},
		{"image/webp", "shot.webp", `inline; filename="shot.webp"`},
		{"IMAGE/PNG", "still.png", `inline; filename="still.png"`},
		{"image/png; charset=binary", "still.png", `inline; filename="still.png"`},
		{"  Application/PDF  ", "cv.pdf", `attachment; filename="cv.pdf"`},
		{"application/pdf", "cv.pdf", `attachment; filename="cv.pdf"`},
		{"application/pdf", "evil\"\r\nname.pdf", `attachment; filename="evilname.pdf"`},
		{"application/pdf", "", `attachment; filename="upload"`},
		{"image/png", "../../evil.png", `inline; filename="evil.png"`},
		{"application/pdf", "../../evil.pdf", `attachment; filename="evil.pdf"`},
		{"text/plain", "notes.txt", ""},
		{"application/octet-stream", "bin", ""},
	}
	for _, tc := range cases {
		got := services.ContentDisposition(tc.mime, tc.name)
		if got != tc.want {
			t.Fatalf("ContentDisposition(%q, %q) = %q, want %q", tc.mime, tc.name, got, tc.want)
		}
		if got == "" {
			continue
		}
		if strings.ContainsAny(got, "\r\n") {
			t.Fatalf("ContentDisposition(%q, %q) contains CRLF: %q", tc.mime, tc.name, got)
		}
		// Quoted filename must not reintroduce quotes or path segments.
		start := strings.Index(got, `filename="`)
		if start < 0 {
			t.Fatalf("ContentDisposition(%q, %q) missing filename=: %q", tc.mime, tc.name, got)
		}
		fname := got[start+len(`filename="`):]
		fname = strings.TrimSuffix(fname, `"`)
		if strings.ContainsAny(fname, "\"\r\n/\\\x00;") {
			t.Fatalf("filename in %q is unsafe: %q", got, fname)
		}
	}
}
