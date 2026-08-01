package services

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var navPathCharset = regexp.MustCompile(`^[A-Za-z0-9/_#?&=%.~+-]+$`)

// ValidateHTTPSURL checks that raw is empty (when allowed) or an https URL.
// Rejects javascript:, data:, vbscript:, and scheme-relative //… forms.
func ValidateHTTPSURL(raw string, allowEmpty bool) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("%w: url is required", ErrValidation)
	}
	if strings.HasPrefix(s, "//") {
		return fmt.Errorf("%w: scheme-relative urls are not allowed", ErrValidation)
	}
	lower := strings.ToLower(s)
	for _, bad := range []string{"javascript:", "data:", "vbscript:"} {
		if strings.HasPrefix(lower, bad) {
			return fmt.Errorf("%w: disallowed url scheme", ErrValidation)
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("%w: invalid url", ErrValidation)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("%w: url must use https", ErrValidation)
	}
	return nil
}

// ValidateNavPath checks that raw is a non-empty relative path starting with a
// single /, with no scheme and an allowlisted character set.
func ValidateNavPath(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fmt.Errorf("%w: path is required", ErrValidation)
	}
	if strings.HasPrefix(s, "//") {
		return fmt.Errorf("%w: path must not be scheme-relative", ErrValidation)
	}
	if !strings.HasPrefix(s, "/") {
		return fmt.Errorf("%w: path must start with /", ErrValidation)
	}
	if strings.Contains(s, "://") {
		return fmt.Errorf("%w: path must be relative", ErrValidation)
	}
	if !navPathCharset.MatchString(s) {
		return fmt.Errorf("%w: path contains invalid characters", ErrValidation)
	}
	return nil
}

// ValidateEmail allows empty values. Non-empty values must contain @ and must
// not include whitespace, colon, or control characters.
func ValidateEmail(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "//") {
		return fmt.Errorf("%w: invalid email", ErrValidation)
	}
	if !strings.Contains(s, "@") {
		return fmt.Errorf("%w: invalid email", ErrValidation)
	}
	for _, r := range s {
		if r == ':' || unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("%w: invalid email", ErrValidation)
		}
	}
	return nil
}
