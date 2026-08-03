package auth

import "testing"

func TestIPPrefixIPv4(t *testing.T) {
	got := IPPrefix("198.51.100.42")
	if got != "198.51.100.0/24" {
		t.Fatalf("IPPrefix = %q, want 198.51.100.0/24", got)
	}
	if HashIPPrefix("198.51.100.42") != HashIPPrefix("198.51.100.99") {
		t.Fatal("same /24 should hash equal")
	}
	if HashIPPrefix("198.51.100.42") == HashIPPrefix("198.51.101.1") {
		t.Fatal("different /24 should hash different")
	}
}

func TestIPPrefixIPv6(t *testing.T) {
	got := IPPrefix("2001:db8:abcd:1234::1")
	want := "2001:db8:abcd::/48"
	if got != want {
		t.Fatalf("IPPrefix = %q, want %q", got, want)
	}
	if HashIPPrefix("2001:db8:abcd:1234::1") != HashIPPrefix("2001:db8:abcd:ffff::9") {
		t.Fatal("same /48 should hash equal")
	}
	if HashIPPrefix("2001:db8:abcd::1") == HashIPPrefix("2001:db8:abce::1") {
		t.Fatal("different /48 should hash different")
	}
}

func TestHashUAEmpty(t *testing.T) {
	if HashUA("") != "" {
		t.Fatal("empty UA should yield empty hash")
	}
	if HashIPPrefix("") != "" {
		t.Fatal("empty IP should yield empty hash")
	}
	if IPPrefix("not-an-ip") != "" {
		t.Fatal("invalid IP should yield empty prefix")
	}
}

func TestHashUADeterministic(t *testing.T) {
	a := HashUA("Mozilla/5.0")
	b := HashUA("Mozilla/5.0")
	if a == "" || a != b {
		t.Fatalf("HashUA not stable: %q vs %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("hash len = %d, want 64 hex chars", len(a))
	}
}
