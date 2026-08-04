package config

import (
	"net"
	"testing"
)

func TestEnvBool(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"no", false},
		{"garbage", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"YES", true},
	}

	for _, c := range cases {
		t.Setenv("ALLOW_STATIC_DIR", c.value)
		if got := envBool("ALLOW_STATIC_DIR"); got != c.want {
			t.Errorf("envBool(%q) = %v, want %v", c.value, got, c.want)
		}
	}
}

func TestLoadAllowStaticDirDefaultsFalse(t *testing.T) {
	t.Setenv("ALLOW_STATIC_DIR", "")
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AllowStaticDir {
		t.Fatal("AllowStaticDir = true, want false by default")
	}
}

func TestLoadSessionBindingDefaultsFalse(t *testing.T) {
	t.Setenv("SESSION_BINDING", "")
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SessionBinding {
		t.Fatal("SessionBinding = true, want false by default")
	}
}

func TestLoadSessionBindingEnabled(t *testing.T) {
	t.Setenv("SESSION_BINDING", "1")
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.SessionBinding {
		t.Fatal("SessionBinding = false, want true when SESSION_BINDING=1")
	}
}

func TestParseTrustedProxies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantN    int
		wantUnix bool
		wantErr  bool
		contains string // IP that should match first net, if wantN > 0
	}{
		{name: "empty", input: "", wantN: 0},
		{name: "whitespace", input: "  ", wantN: 0},
		{name: "cidr v4", input: "127.0.0.0/8", wantN: 1, contains: "127.0.0.1"},
		{name: "bare ipv4", input: "127.0.0.1", wantN: 1, contains: "127.0.0.1"},
		{name: "bare ipv6", input: "::1", wantN: 1, contains: "::1"},
		{name: "ipv6 cidr", input: "::1/128", wantN: 1, contains: "::1"},
		{name: "unix only", input: "unix", wantUnix: true},
		{name: "mixed", input: "127.0.0.0/8,::1/128,unix", wantN: 2, wantUnix: true, contains: "127.1.2.3"},
		{name: "spaces around commas", input: " 127.0.0.1/32 , unix ", wantN: 1, wantUnix: true, contains: "127.0.0.1"},
		{name: "invalid ip", input: "not-an-ip", wantErr: true},
		{name: "invalid cidr", input: "127.0.0.1/99", wantErr: true},
		{name: "empty token", input: "127.0.0.1,,unix", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTrustedProxies(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTrustedProxies: %v", err)
			}
			if len(got.Nets) != tt.wantN {
				t.Fatalf("Nets = %d, want %d", len(got.Nets), tt.wantN)
			}
			if got.Unix != tt.wantUnix {
				t.Fatalf("Unix = %v, want %v", got.Unix, tt.wantUnix)
			}
			if tt.contains != "" {
				ip := net.ParseIP(tt.contains)
				if ip == nil || !got.Nets[0].Contains(ip) {
					t.Fatalf("Nets[0] does not contain %s", tt.contains)
				}
			}
		})
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.0/8,unix")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TrustedProxies.Nets) != 1 {
		t.Fatalf("Nets = %d, want 1", len(cfg.TrustedProxies.Nets))
	}
	if !cfg.TrustedProxies.Unix {
		t.Fatal("Unix = false, want true")
	}
}

func TestLoadTrustedProxiesInvalid(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "bogus")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid TRUSTED_PROXIES")
	}
}

func TestLoadTrustedProxiesDefaultEmpty(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TrustedProxies.Nets) != 0 || cfg.TrustedProxies.Unix {
		t.Fatalf("default TrustedProxies = %+v, want empty", cfg.TrustedProxies)
	}
}
