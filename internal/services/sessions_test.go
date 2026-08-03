package services_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/yccoskun/website/internal/auth"
	"github.com/yccoskun/website/internal/services"
)

var hexTokenPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestSessionCreateValidate(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, expires, err := svc.Create("", "", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	wantMin := time.Now().UTC().Add(23 * time.Hour)
	wantMax := time.Now().UTC().Add(25 * time.Hour)
	if expires.Before(wantMin) || expires.After(wantMax) {
		t.Fatalf("expires = %v, want within ~24h of now", expires)
	}

	ok, mismatch, err := svc.Validate(token, "", "", false)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if mismatch {
		t.Fatal("unexpected binding mismatch")
	}
	if !ok {
		t.Fatal("expected valid session")
	}
}

func TestSessionCreateTokenIsHexFormat(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, _, err := svc.Create("", "", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !hexTokenPattern.MatchString(token) {
		t.Fatalf("token = %q, want 64 lowercase hex chars (32 bytes)", token)
	}
}

func TestSessionCreateStoresOnlyTokenHash(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, _, err := svc.Create("", "", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var storedHash string
	if err := db.QueryRow(`SELECT token_hash FROM sessions`).Scan(&storedHash); err != nil {
		t.Fatalf("select token_hash: %v", err)
	}

	sum := sha256.Sum256([]byte(token))
	wantHash := hex.EncodeToString(sum[:])
	if storedHash != wantHash {
		t.Fatalf("stored token_hash = %q, want sha256(raw) = %q", storedHash, wantHash)
	}

	ok, mismatch, err := svc.Validate(token, "", "", false)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if mismatch || !ok {
		t.Fatal("expected Validate to succeed with the raw token")
	}
}

func TestSessionCreateStoresBindingHashes(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	ua := "TestAgent/1.0"
	ip := "198.51.100.42"
	if _, _, err := svc.Create(ua, ip, true); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var uaHash, ipHash sql.NullString
	if err := db.QueryRow(`SELECT ua_hash, ip_prefix_hash FROM sessions`).Scan(&uaHash, &ipHash); err != nil {
		t.Fatalf("select binding: %v", err)
	}
	if !uaHash.Valid || uaHash.String != auth.HashUA(ua) {
		t.Fatalf("ua_hash = %v, want %q", uaHash, auth.HashUA(ua))
	}
	if !ipHash.Valid || ipHash.String != auth.HashIPPrefix(ip) {
		t.Fatalf("ip_prefix_hash = %v, want %q", ipHash, auth.HashIPPrefix(ip))
	}
}

func TestSessionCreateFlagOffStoresNULL(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	if _, _, err := svc.Create("TestAgent/1.0", "198.51.100.1", false); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var uaHash, ipHash sql.NullString
	if err := db.QueryRow(`SELECT ua_hash, ip_prefix_hash FROM sessions`).Scan(&uaHash, &ipHash); err != nil {
		t.Fatalf("select binding: %v", err)
	}
	if uaHash.Valid {
		t.Fatalf("ua_hash = %v, want NULL when binding off", uaHash)
	}
	if ipHash.Valid {
		t.Fatalf("ip_prefix_hash = %v, want NULL when binding off", ipHash)
	}
}

func TestSessionCreateBindingRequiresHints(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	_, _, err := svc.Create("", "198.51.100.1", true)
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("empty UA: want ErrValidation, got %v", err)
	}
	_, _, err = svc.Create("TestAgent/1.0", "", true)
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("empty IP: want ErrValidation, got %v", err)
	}
	_, _, err = svc.Create("TestAgent/1.0", "not-an-ip", true)
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("bad IP: want ErrValidation, got %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("sessions = %d, want 0 after failed Creates", n)
	}
}

func TestSessionValidateBindingMatch(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	ua := "Mozilla/5.0"
	ip := "203.0.113.9"
	token, _, err := svc.Create(ua, ip, true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, ua, ip, true)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok || mismatch {
		t.Fatalf("ok=%v mismatch=%v, want match", ok, mismatch)
	}

	// Same /24, different host → still match.
	ok, mismatch, err = svc.Validate(token, ua, "203.0.113.200", true)
	if err != nil {
		t.Fatalf("Validate same /24: %v", err)
	}
	if !ok || mismatch {
		t.Fatalf("same /24: ok=%v mismatch=%v, want match", ok, mismatch)
	}
}

func TestSessionValidateBindingMismatch(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	ua := "Mozilla/5.0"
	ip := "203.0.113.9"
	token, _, err := svc.Create(ua, ip, true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, "OtherAgent", ip, true)
	if err != nil {
		t.Fatalf("Validate UA mismatch: %v", err)
	}
	if ok || !mismatch {
		t.Fatalf("UA mismatch: ok=%v mismatch=%v, want mismatch", ok, mismatch)
	}

	ok, mismatch, err = svc.Validate(token, ua, "198.51.100.1", true)
	if err != nil {
		t.Fatalf("Validate IP mismatch: %v", err)
	}
	if ok || !mismatch {
		t.Fatalf("IP mismatch: ok=%v mismatch=%v, want mismatch", ok, mismatch)
	}
}

func TestSessionValidateIPv6Prefix(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	ua := "Mozilla/5.0"
	ip := "2001:db8:abcd:1111::1"
	token, _, err := svc.Create(ua, ip, true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, ua, "2001:db8:abcd:2222::9", true)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok || mismatch {
		t.Fatalf("same /48: ok=%v mismatch=%v, want match", ok, mismatch)
	}

	ok, mismatch, err = svc.Validate(token, ua, "2001:db8:ffff::1", true)
	if err != nil {
		t.Fatalf("Validate other /48: %v", err)
	}
	if ok || !mismatch {
		t.Fatalf("other /48: ok=%v mismatch=%v, want mismatch", ok, mismatch)
	}
}

func TestSessionValidateBindingFlagOffIgnoresMismatch(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, _, err := svc.Create("Mozilla/5.0", "203.0.113.9", true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, "OtherAgent", "198.51.100.1", false)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok || mismatch {
		t.Fatalf("flag off: ok=%v mismatch=%v, want ok", ok, mismatch)
	}
}

func TestSessionValidateLegacyNULLSkipsBinding(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, _, err := svc.Create("Mozilla/5.0", "203.0.113.9", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, "OtherAgent", "198.51.100.1", true)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok || mismatch {
		t.Fatalf("legacy NULL: ok=%v mismatch=%v, want ok", ok, mismatch)
	}
}

func TestSessionValidatePartialBindingSkips(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, _, err := svc.Create("Mozilla/5.0", "203.0.113.9", true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := db.Exec(`UPDATE sessions SET ip_prefix_hash = NULL`); err != nil {
		t.Fatalf("clear ip hash: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, "OtherAgent", "203.0.113.1", true)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok || mismatch {
		t.Fatalf("partial binding: ok=%v mismatch=%v, want ok", ok, mismatch)
	}
}

func TestSessionExpiredValidateFalse(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionServiceWithTTL(db, -time.Hour)

	token, _, err := svc.Create("", "", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, "", "", false)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok || mismatch {
		t.Fatal("expected expired session to fail Validate")
	}
}

func TestSessionDestroyValidateFalse(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, _, err := svc.Create("", "", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Destroy(token); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	ok, mismatch, err := svc.Validate(token, "", "", false)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok || mismatch {
		t.Fatal("expected destroyed session to fail Validate")
	}
}

func TestSessionDestroyExpired(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionServiceWithTTL(db, -time.Hour)

	if _, _, err := svc.Create("", "", false); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&before); err != nil {
		t.Fatalf("count: %v", err)
	}
	if before != 1 {
		t.Fatalf("sessions before = %d, want 1", before)
	}

	if err := svc.DestroyExpired(); err != nil {
		t.Fatalf("DestroyExpired: %v", err)
	}

	var after int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&after); err != nil {
		t.Fatalf("count: %v", err)
	}
	if after != 0 {
		t.Fatalf("sessions after DestroyExpired = %d, want 0", after)
	}
}
