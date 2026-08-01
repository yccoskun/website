package services_test

import (
	"bytes"
	"errors"
	"path/filepath"
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
