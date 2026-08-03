package securitylog

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func TestLoginFailureBurstAlert(t *testing.T) {
	tr := NewAlertTracker()
	buf := captureLog(t)
	const ip = "203.0.113.10"
	const route = "/api/admin/login"

	for i := 1; i <= 4; i++ {
		tr.LoginFailure(ip, route)
		line := lastLine(buf)
		if strings.Contains(line, "alert=1") {
			t.Fatalf("failure %d should not alert: %q", i, line)
		}
		if !strings.Contains(line, "route="+route) {
			t.Fatalf("failure %d missing route: %q", i, line)
		}
	}

	tr.LoginFailure(ip, route)
	line := lastLine(buf)
	if !strings.Contains(line, "alert=1") || !strings.Contains(line, "reason="+ReasonLoginFailureBurst) {
		t.Fatalf("5th failure should alert: %q", line)
	}

	buf.Reset()
	tr.LoginFailure(ip, route)
	line = lastLine(buf)
	if strings.Contains(line, "alert=1") {
		t.Fatalf("6th failure should not re-alert in window: %q", line)
	}
	if !strings.Contains(line, "event="+EventLoginFailure) {
		t.Fatalf("6th failure should still log base event: %q", line)
	}
}

func TestLoginSuccessAfterFailuresAlert(t *testing.T) {
	tr := NewAlertTracker()
	buf := captureLog(t)
	const ip = "203.0.113.11"
	const route = "/api/admin/login"

	tr.LoginSuccess(ip, route)
	line := lastLine(buf)
	if !strings.Contains(line, "event="+EventLoginSuccess) {
		t.Fatalf("want login_success: %q", line)
	}
	if strings.Contains(line, "alert=1") {
		t.Fatalf("success with 0 prior failures must not alert: %q", line)
	}

	buf.Reset()
	for i := 0; i < 2; i++ {
		tr.LoginFailure(ip, route)
	}
	tr.LoginSuccess(ip, route)
	line = lastLine(buf)
	if strings.Contains(line, "alert=1") {
		t.Fatalf("success with 2 prior failures must not alert: %q", line)
	}

	buf.Reset()
	for i := 0; i < loginSuccessPriorMin; i++ {
		tr.LoginFailure(ip, route)
	}
	tr.LoginSuccess(ip, route)
	line = lastLine(buf)
	if !strings.Contains(line, "alert=1") ||
		!strings.Contains(line, "reason="+ReasonLoginSuccessAfterFailures) ||
		!strings.Contains(line, "failures=5") {
		t.Fatalf("success after 5 failures should alert: %q", line)
	}

	buf.Reset()
	for i := 0; i < loginSuccessPriorMin; i++ {
		tr.LoginFailure(ip, route)
	}
	tr.LoginSuccess(ip, route)
	line = lastLine(buf)
	if strings.Contains(line, "alert=1") {
		t.Fatalf("second success-after-failures in same window must not re-alert: %q", line)
	}
}

func TestLoginSuccessNoSecrets(t *testing.T) {
	tr := NewAlertTracker()
	buf := captureLog(t)
	tr.LoginFailure("1.2.3.4", "/api/admin/login")
	tr.LoginSuccess("1.2.3.4", "/api/admin/login")
	out := buf.String()
	for _, bad := range []string{"password=", "username=", "token=", "Bearer ", "leaked"} {
		if strings.Contains(out, bad) {
			t.Fatalf("log must not contain %q: %q", bad, out)
		}
	}
}

func TestRateLimitAlertsOncePerWindow(t *testing.T) {
	tr := NewAlertTracker()
	buf := captureLog(t)
	const ip = "203.0.113.12"

	tr.RateLimit(ip, "/api/admin/login")
	line := lastLine(buf)
	if !strings.Contains(line, "alert=1") || !strings.Contains(line, "reason="+ReasonRateLimit) {
		t.Fatalf("first 429 must alert: %q", line)
	}
	if !strings.Contains(line, "route=/api/admin/login") {
		t.Fatalf("missing route: %q", line)
	}

	buf.Reset()
	tr.RateLimit(ip, "/api/admin/login")
	line = lastLine(buf)
	if !strings.Contains(line, "event="+EventRateLimit) {
		t.Fatalf("second 429 must still log base event: %q", line)
	}
	if strings.Contains(line, "alert=1") {
		t.Fatalf("second 429 in window must not re-alert: %q", line)
	}
}

func TestMediaUploadSpikeAlert(t *testing.T) {
	tr := NewAlertTracker()
	buf := captureLog(t)
	const ip = "203.0.113.13"
	const route = "/api/admin/media"

	for i := 1; i < mediaUploadThreshold; i++ {
		tr.MediaUpload(ip, route)
		if strings.Contains(lastLine(buf), "alert=1") {
			t.Fatalf("upload %d should not alert", i)
		}
	}

	tr.MediaUpload(ip, route)
	line := lastLine(buf)
	if !strings.Contains(line, "alert=1") ||
		!strings.Contains(line, "reason="+ReasonMediaUploadSpike) ||
		!strings.Contains(line, "count=15") {
		t.Fatalf("15th upload should alert: %q", line)
	}

	buf.Reset()
	tr.MediaUpload(ip, route)
	line = lastLine(buf)
	if strings.Contains(line, "alert=1") {
		t.Fatalf("16th upload should not re-alert: %q", line)
	}
}

func TestImportExportAlertFields(t *testing.T) {
	if got := ExportAlertFields(); len(got) < 4 || got[1] != "1" || got[3] != ReasonExport {
		t.Fatalf("ExportAlertFields = %v", got)
	}
	if got := ImportAlertFields(false, false, false); got != nil {
		t.Fatalf("no replace should return nil, got %v", got)
	}
	got := ImportAlertFields(true, false, false)
	if len(got) < 4 || got[3] != ReasonImportReplace {
		t.Fatalf("ImportAlertFields = %v", got)
	}
}

func TestAlertWindowsReset(t *testing.T) {
	tr := NewAlertTracker()
	buf := captureLog(t)
	now := time.Now()
	tr.now = func() time.Time { return now }

	const ip = "203.0.113.14"
	for i := 0; i < loginFailureThreshold; i++ {
		tr.LoginFailure(ip, "/api/admin/login")
	}
	if !strings.Contains(lastLine(buf), "alert=1") {
		t.Fatal("expected burst alert")
	}

	now = now.Add(loginFailureWindow + time.Second)
	buf.Reset()
	for i := 0; i < loginFailureThreshold-1; i++ {
		tr.LoginFailure(ip, "/api/admin/login")
	}
	if strings.Contains(buf.String(), "alert=1") {
		t.Fatalf("new window should not alert before threshold: %q", buf.String())
	}
	tr.LoginFailure(ip, "/api/admin/login")
	if !strings.Contains(lastLine(buf), "alert=1") {
		t.Fatalf("new window 5th failure should alert: %q", lastLine(buf))
	}
}

func TestMediaUploadWindowReset(t *testing.T) {
	tr := NewAlertTracker()
	buf := captureLog(t)
	now := time.Now()
	tr.now = func() time.Time { return now }

	const ip = "203.0.113.15"
	const route = "/api/admin/media"
	for i := 0; i < mediaUploadThreshold; i++ {
		tr.MediaUpload(ip, route)
	}
	if !strings.Contains(lastLine(buf), "alert=1") {
		t.Fatal("expected media spike alert")
	}

	now = now.Add(mediaUploadWindow + time.Second)
	buf.Reset()
	for i := 0; i < mediaUploadThreshold-1; i++ {
		tr.MediaUpload(ip, route)
	}
	if strings.Contains(buf.String(), "alert=1") {
		t.Fatalf("new media window should not alert before threshold: %q", buf.String())
	}
	tr.MediaUpload(ip, route)
	if !strings.Contains(lastLine(buf), "alert=1") {
		t.Fatalf("new window 15th upload should alert: %q", lastLine(buf))
	}
}

func lastLine(buf *bytes.Buffer) string {
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}
