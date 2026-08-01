package auth

import (
	"strings"
	"testing"
)

func TestConstantTimeUsernameEqual_match(t *testing.T) {
	if !ConstantTimeUsernameEqual("admin", "admin") {
		t.Fatal("expected equal usernames to match")
	}
}

func TestConstantTimeUsernameEqual_mismatch(t *testing.T) {
	if ConstantTimeUsernameEqual("admin", "other") {
		t.Fatal("expected different usernames to not match")
	}
}

func TestConstantTimeUsernameEqual_empty(t *testing.T) {
	if !ConstantTimeUsernameEqual("", "") {
		t.Fatal("expected two empty strings to match")
	}
	if ConstantTimeUsernameEqual("", "admin") {
		t.Fatal("expected empty vs non-empty to not match")
	}
	if ConstantTimeUsernameEqual("admin", "") {
		t.Fatal("expected non-empty vs empty to not match")
	}
}

func TestConstantTimeUsernameEqual_unequalLengths(t *testing.T) {
	if ConstantTimeUsernameEqual("admin", "adm") {
		t.Fatal("expected prefix mismatch to not match")
	}
	if ConstantTimeUsernameEqual("a", "ab") {
		t.Fatal("expected unequal lengths to not match")
	}
}

func TestConstantTimeUsernameEqual_overMaxLength(t *testing.T) {
	long := strings.Repeat("a", maxUsernameLength+1)
	ok := strings.Repeat("a", maxUsernameLength)

	if ConstantTimeUsernameEqual(long, long) {
		t.Fatal("expected over-max username to not match")
	}
	if ConstantTimeUsernameEqual(long, ok) {
		t.Fatal("expected over-max vs valid length to not match")
	}
	if ConstantTimeUsernameEqual(ok, long) {
		t.Fatal("expected valid length vs over-max to not match")
	}
}
