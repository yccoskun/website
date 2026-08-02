package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/yccoskun/website/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestReadPasswordInput_rejectsArgs(t *testing.T) {
	t.Parallel()
	_, err := readPasswordInput([]string{"secret"}, strings.NewReader(""), 0, io.Discard, alwaysFalse, nil)
	if err == nil {
		t.Fatal("expected error when argv is present")
	}
	if !strings.Contains(err.Error(), "do not pass the password on the command line") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadPasswordInput_pipedPassword(t *testing.T) {
	t.Parallel()
	got, err := readPasswordInput(nil, strings.NewReader("correct horse\n"), 0, io.Discard, alwaysFalse, nil)
	if err != nil {
		t.Fatalf("readPasswordInput: %v", err)
	}
	if got != "correct horse" {
		t.Fatalf("got %q, want %q", got, "correct horse")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(got), 12)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	if !strings.HasPrefix(string(hash), "$2a$12$") {
		t.Fatalf("hash %q missing cost-12 prefix $2a$12$", hash)
	}
	if !auth.CheckPassword(string(hash), got) {
		t.Fatal("expected hashed password to verify with auth.CheckPassword")
	}
}

func TestReadPasswordInput_emptyStdin(t *testing.T) {
	t.Parallel()
	got, err := readPasswordInput(nil, strings.NewReader(""), 0, io.Discard, alwaysFalse, nil)
	if err != nil {
		t.Fatalf("readPasswordInput: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestReadPasswordInput_trimsTrailingNewlines(t *testing.T) {
	t.Parallel()
	got, err := readPasswordInput(nil, strings.NewReader("secret\r\n"), 0, io.Discard, alwaysFalse, nil)
	if err != nil {
		t.Fatalf("readPasswordInput: %v", err)
	}
	if got != "secret" {
		t.Fatalf("got %q, want %q", got, "secret")
	}
}

func TestReadPasswordInput_interactive(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	got, err := readPasswordInput(nil, nil, 7, &stderr, alwaysTrue, func(fd int) ([]byte, error) {
		if fd != 7 {
			t.Fatalf("fd = %d, want 7", fd)
		}
		return []byte("typed-secret"), nil
	})
	if err != nil {
		t.Fatalf("readPasswordInput: %v", err)
	}
	if got != "typed-secret" {
		t.Fatalf("got %q, want %q", got, "typed-secret")
	}
	if !strings.Contains(stderr.String(), "Password: ") {
		t.Fatalf("stderr missing prompt: %q", stderr.String())
	}
}

func TestReadPasswordInput_interactiveError(t *testing.T) {
	t.Parallel()
	want := errors.New("read failed")
	_, err := readPasswordInput(nil, nil, 0, io.Discard, alwaysTrue, func(int) ([]byte, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func alwaysFalse(int) bool { return false }
func alwaysTrue(int) bool  { return true }
