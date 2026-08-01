package services_test

import (
	"errors"
	"testing"

	"github.com/yccoskun/website/internal/services"
)

func TestValidateHTTPSURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		raw        string
		allowEmpty bool
		wantErr    bool
	}{
		{name: "good https", raw: "https://example.com/x", allowEmpty: false},
		{name: "good https case", raw: "HTTPS://Example.COM/path", allowEmpty: false},
		{name: "empty allowed", raw: "", allowEmpty: true},
		{name: "empty whitespace allowed", raw: "  ", allowEmpty: true},
		{name: "empty rejected", raw: "", allowEmpty: false, wantErr: true},
		{name: "javascript", raw: "javascript:alert(1)", allowEmpty: true, wantErr: true},
		{name: "javascript mixed case", raw: "JavaScript:alert(1)", allowEmpty: true, wantErr: true},
		{name: "scheme relative", raw: "//evil.com", allowEmpty: true, wantErr: true},
		{name: "data", raw: "data:text/html,hi", allowEmpty: true, wantErr: true},
		{name: "vbscript", raw: "vbscript:msgbox", allowEmpty: true, wantErr: true},
		{name: "http rejected", raw: "http://example.com", allowEmpty: true, wantErr: true},
		{name: "mailto rejected", raw: "mailto:a@b.c", allowEmpty: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := services.ValidateHTTPSURL(tc.raw, tc.allowEmpty)
			if tc.wantErr {
				if err == nil || !errors.Is(err, services.ErrValidation) {
					t.Fatalf("want ErrValidation, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateNavPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "root", raw: "/"},
		{name: "blog", raw: "/blog"},
		{name: "query hash", raw: "/work?x=1#top"},
		{name: "empty", raw: "", wantErr: true},
		{name: "scheme relative", raw: "//evil.com", wantErr: true},
		{name: "no leading slash", raw: "blog", wantErr: true},
		{name: "absolute https", raw: "https://example.com", wantErr: true},
		{name: "javascript", raw: "javascript:alert(1)", wantErr: true},
		{name: "space", raw: "/has space", wantErr: true},
		{name: "backslash", raw: "/a\\b", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := services.ValidateNavPath(tc.raw)
			if tc.wantErr {
				if err == nil || !errors.Is(err, services.ErrValidation) {
					t.Fatalf("want ErrValidation, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "empty", raw: ""},
		{name: "simple", raw: "a@b.c"},
		{name: "no at", raw: "not-an-email", wantErr: true},
		{name: "whitespace", raw: "a @b.c", wantErr: true},
		{name: "colon", raw: "a:b@c.d", wantErr: true},
		{name: "scheme relative", raw: "//evil@x", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := services.ValidateEmail(tc.raw)
			if tc.wantErr {
				if err == nil || !errors.Is(err, services.ErrValidation) {
					t.Fatalf("want ErrValidation, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
