package services_test

import (
	"testing"
	"time"

	"github.com/yccoskun/website/internal/services"
)

func TestSessionCreateValidate(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, expires, err := svc.Create()
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

	ok, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok {
		t.Fatal("expected valid session")
	}
}

func TestSessionExpiredValidateFalse(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionServiceWithTTL(db, -time.Hour)

	token, _, err := svc.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ok, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok {
		t.Fatal("expected expired session to fail Validate")
	}
}

func TestSessionDestroyValidateFalse(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionService(db)

	token, _, err := svc.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Destroy(token); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	ok, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok {
		t.Fatal("expected destroyed session to fail Validate")
	}
}

func TestSessionDestroyExpired(t *testing.T) {
	db := openCMSDB(t)
	svc := services.NewSessionServiceWithTTL(db, -time.Hour)

	if _, _, err := svc.Create(); err != nil {
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
