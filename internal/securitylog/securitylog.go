package securitylog

import (
	"log"
	"strings"
)

const (
	EventLoginFailure           = "login_failure"
	EventRateLimit              = "rate_limit"
	EventExport                 = "export"
	EventImport                 = "import"
	EventMediaDelete            = "media_delete"
	EventSessionBindingMismatch = "session_binding_mismatch"
)

// Event logs a one-line structured security event to the default logger.
// fields are alternating key, value pairs (e.g. "path", "/api/admin/export").
// A trailing key without a value is ignored.
func Event(event string, ip string, fields ...string) {
	var b strings.Builder
	b.WriteString("security event=")
	b.WriteString(event)
	b.WriteString(" ip=")
	b.WriteString(ip)
	for i := 0; i+1 < len(fields); i += 2 {
		b.WriteByte(' ')
		b.WriteString(fields[i])
		b.WriteByte('=')
		b.WriteString(fields[i+1])
	}
	log.Print(b.String())
}
