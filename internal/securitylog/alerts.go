package securitylog

import (
	"context"
	"strconv"
	"sync"
	"time"
)

const (
	ReasonRateLimit                 = "rate_limit"
	ReasonLoginFailureBurst         = "login_failure_burst"
	ReasonLoginSuccessAfterFailures = "login_success_after_failures"
	ReasonMediaUploadSpike          = "media_upload_spike"
	ReasonExport                    = "export"
	ReasonImportReplace             = "import_replace"

	loginFailureThreshold    = 5
	loginFailureWindow       = 15 * time.Minute
	loginSuccessPriorMin     = 5 // align with burst; typo-then-success must not page
	mediaUploadThreshold     = 15
	mediaUploadWindow        = 10 * time.Minute
	rateLimitAlertWindow     = 15 * time.Minute
	alertTrackerMaxEntries   = 10_000
	alertTrackerPruneDefault = time.Minute
)

// Overridable in tests via t.Cleanup restore.
var alertTrackerPruneInterval = alertTrackerPruneDefault

type ipWindow struct {
	count   int
	resetAt time.Time
}

// AlertTracker tracks per-IP windows and decides when to attach alert=1 fields.
type AlertTracker struct {
	mu           sync.Mutex
	now          func() time.Time
	loginFails   map[string]ipWindow
	mediaUploads map[string]ipWindow
	alerted      map[string]time.Time // key: ip\x00reason → window end
}

// Default is the process-wide tracker used by handlers and middleware.
var Default = NewAlertTracker()

// NewAlertTracker constructs an empty in-memory alert tracker.
func NewAlertTracker() *AlertTracker {
	return &AlertTracker{
		now:          time.Now,
		loginFails:   make(map[string]ipWindow),
		mediaUploads: make(map[string]ipWindow),
		alerted:      make(map[string]time.Time),
	}
}

// RunPruneLoop periodically removes expired tracker entries until ctx is cancelled.
// Not started by NewAlertTracker; callers must start it when desired.
func (t *AlertTracker) RunPruneLoop(ctx context.Context) {
	ticker := time.NewTicker(alertTrackerPruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := t.clock()
			t.mu.Lock()
			t.pruneExpired(now)
			t.mu.Unlock()
		}
	}
}

// LoginFailure records a failed login and emits login_failure with route=.
// On the 5th failure in 15m for an IP, includes alert=1 reason=login_failure_burst (once per window).
func (t *AlertTracker) LoginFailure(ip, route string) {
	fields := t.loginFailureFields(ip, route)
	Event(EventLoginFailure, ip, fields...)
}

// LoginSuccess emits login_success with route=.
// If ≥5 prior failures in the window, includes alert=1 reason=login_success_after_failures failures=N (once).
func (t *AlertTracker) LoginSuccess(ip, route string) {
	fields := t.loginSuccessFields(ip, route)
	Event(EventLoginSuccess, ip, fields...)
}

// RateLimit emits rate_limit with route=. alert=1 reason=rate_limit once per IP per 15m window.
func (t *AlertTracker) RateLimit(ip, route string) {
	t.mu.Lock()
	now := t.clock()
	alert := t.markAlertLocked(ip, ReasonRateLimit, now.Add(rateLimitAlertWindow), now)
	t.mu.Unlock()

	fields := []string{"route", route}
	if alert {
		fields = append(fields, "alert", "1", "reason", ReasonRateLimit)
	}
	Event(EventRateLimit, ip, fields...)
}

// MediaUpload records an upload and emits media_upload with route=.
// On the 15th upload in 10m for an IP, includes alert=1 reason=media_upload_spike count=N (once per window).
func (t *AlertTracker) MediaUpload(ip, route string) {
	fields := t.mediaUploadFields(ip, route)
	Event(EventMediaUpload, ip, fields...)
}

// ExportAlertFields returns alert fields for a successful export (always alert).
func ExportAlertFields() []string {
	return []string{"alert", "1", "reason", ReasonExport}
}

// ImportAlertFields returns alert fields when any replace flag is true; otherwise nil.
func ImportAlertFields(replaceWork, replaceStudio, replaceResume bool) []string {
	if !replaceWork && !replaceStudio && !replaceResume {
		return nil
	}
	return []string{"alert", "1", "reason", ReasonImportReplace}
}

func (t *AlertTracker) loginFailureFields(ip, route string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.clock()

	w, ok := t.loginFails[ip]
	if !ok || now.After(w.resetAt) {
		if !ok {
			if len(t.loginFails) >= alertTrackerMaxEntries {
				t.pruneExpired(now)
				t.evictMapIfFull(t.loginFails)
			}
		}
		w = ipWindow{resetAt: now.Add(loginFailureWindow)}
	}
	w.count++
	t.loginFails[ip] = w

	fields := []string{"route", route}
	if w.count >= loginFailureThreshold && t.markAlertLocked(ip, ReasonLoginFailureBurst, w.resetAt, now) {
		fields = append(fields, "alert", "1", "reason", ReasonLoginFailureBurst)
	}
	return fields
}

func (t *AlertTracker) loginSuccessFields(ip, route string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.clock()

	failures := 0
	var failWindowEnd time.Time
	if w, ok := t.loginFails[ip]; ok && !now.After(w.resetAt) {
		failures = w.count
		failWindowEnd = w.resetAt
	}
	delete(t.loginFails, ip)

	fields := []string{"route", route}
	if failures >= loginSuccessPriorMin {
		if t.markAlertLocked(ip, ReasonLoginSuccessAfterFailures, failWindowEnd, now) {
			fields = append(fields,
				"alert", "1",
				"reason", ReasonLoginSuccessAfterFailures,
				"failures", strconv.Itoa(failures),
			)
		}
	}
	return fields
}

func (t *AlertTracker) mediaUploadFields(ip, route string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.clock()

	w, ok := t.mediaUploads[ip]
	if !ok || now.After(w.resetAt) {
		if !ok {
			if len(t.mediaUploads) >= alertTrackerMaxEntries {
				t.pruneExpired(now)
				t.evictMapIfFull(t.mediaUploads)
			}
		}
		w = ipWindow{resetAt: now.Add(mediaUploadWindow)}
	}
	w.count++
	t.mediaUploads[ip] = w

	fields := []string{"route", route}
	if w.count >= mediaUploadThreshold && t.markAlertLocked(ip, ReasonMediaUploadSpike, w.resetAt, now) {
		fields = append(fields,
			"alert", "1",
			"reason", ReasonMediaUploadSpike,
			"count", strconv.Itoa(w.count),
		)
	}
	return fields
}

func (t *AlertTracker) markAlertLocked(ip, reason string, windowEnd, now time.Time) bool {
	key := ip + "\x00" + reason
	if until, ok := t.alerted[key]; ok {
		if now.Before(until) {
			return false
		}
		delete(t.alerted, key)
	}
	if len(t.alerted) >= alertTrackerMaxEntries {
		t.pruneExpired(now)
		t.evictAlertedIfFull()
	}
	t.alerted[key] = windowEnd
	return true
}

func (t *AlertTracker) evictAlertedIfFull() {
	if len(t.alerted) < alertTrackerMaxEntries {
		return
	}
	var victim string
	var earliest time.Time
	first := true
	for key, until := range t.alerted {
		if first || until.Before(earliest) {
			victim = key
			earliest = until
			first = false
		}
	}
	if victim != "" {
		delete(t.alerted, victim)
	}
}

func (t *AlertTracker) pruneExpired(now time.Time) {
	for ip, w := range t.loginFails {
		if now.After(w.resetAt) {
			delete(t.loginFails, ip)
		}
	}
	for ip, w := range t.mediaUploads {
		if now.After(w.resetAt) {
			delete(t.mediaUploads, ip)
		}
	}
	for key, until := range t.alerted {
		if !now.Before(until) {
			delete(t.alerted, key)
		}
	}
}

func (t *AlertTracker) evictMapIfFull(m map[string]ipWindow) {
	if len(m) < alertTrackerMaxEntries {
		return
	}
	var victim string
	var earliest time.Time
	first := true
	for ip, w := range m {
		if first || w.resetAt.Before(earliest) {
			victim = ip
			earliest = w.resetAt
			first = false
		}
	}
	if victim != "" {
		delete(m, victim)
	}
}

func (t *AlertTracker) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}
