package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverAPIPath(t *testing.T) {
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom secret path /tmp/leak.go:42")
	})
	handler := Recover(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	want := `{"data":null,"error":"internal error"}`
	if body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
	assertSafeErrorBody(t, body)
}

func TestRecoverNonAPIPath(t *testing.T) {
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom secret path /tmp/leak.go:42")
	})
	handler := Recover(inner)

	req := httptest.NewRequest(http.MethodGet, "/blog", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "internal error" {
		t.Fatalf("body = %q, want %q", body, "internal error")
	}
	assertSafeErrorBody(t, body)
}

func assertSafeErrorBody(t *testing.T, body string) {
	t.Helper()
	for _, leak := range []string{"goroutine", "boom", ".go:", "/tmp/"} {
		if strings.Contains(body, leak) {
			t.Fatalf("body must not contain %q; got %q", leak, body)
		}
	}
}
