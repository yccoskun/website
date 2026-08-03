package securitylog

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestEvent(t *testing.T) {
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	Event(EventLoginFailure, "1.2.3.4")
	got := strings.TrimSpace(buf.String())
	want := "security event=login_failure ip=1.2.3.4"
	if got != want {
		t.Fatalf("Event() = %q, want %q", got, want)
	}

	buf.Reset()
	Event(EventRateLimit, "2001:db8::1", "route", "/api/admin/login")
	got = strings.TrimSpace(buf.String())
	want = "security event=rate_limit ip=2001:db8::1 route=/api/admin/login"
	if got != want {
		t.Fatalf("Event() with fields = %q, want %q", got, want)
	}

	buf.Reset()
	Event(EventExport, "10.0.0.1", "orphan")
	got = strings.TrimSpace(buf.String())
	want = "security event=export ip=10.0.0.1"
	if got != want {
		t.Fatalf("Event() odd fields = %q, want %q", got, want)
	}
}
