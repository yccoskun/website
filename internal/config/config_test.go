package config

import "testing"

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
	cfg := Load()
	if cfg.AllowStaticDir {
		t.Fatal("AllowStaticDir = true, want false by default")
	}
}

func TestLoadSessionBindingDefaultsFalse(t *testing.T) {
	t.Setenv("SESSION_BINDING", "")
	cfg := Load()
	if cfg.SessionBinding {
		t.Fatal("SessionBinding = true, want false by default")
	}
}

func TestLoadSessionBindingEnabled(t *testing.T) {
	t.Setenv("SESSION_BINDING", "1")
	cfg := Load()
	if !cfg.SessionBinding {
		t.Fatal("SessionBinding = false, want true when SESSION_BINDING=1")
	}
}
