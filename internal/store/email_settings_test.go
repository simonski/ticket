package store

import (
	"context"
	"database/sql"
	"testing"
)

func openEmailTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := createSchema(context.Background(), db); err != nil {
		t.Fatalf("createSchema: %v", err)
	}
	return db
}

func TestEmailConfigDefaultsAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openEmailTestDB(t)

	// Defaults on an unconfigured database.
	cfg, err := GetEmailConfig(ctx, db)
	if err != nil {
		t.Fatalf("get defaults: %v", err)
	}
	if cfg.Enabled || cfg.Port != 587 || cfg.Security != EmailSecuritySTARTTLS {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	in := EmailConfig{
		Enabled: true, Host: "smtp.example.com", Port: 465, Username: "mailer",
		Password: "secret", FromAddress: "noreply@example.com", FromName: "Ticket", Security: EmailSecurityTLS,
	}
	if err := SetEmailConfig(ctx, db, in, true); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := GetEmailConfig(ctx, db)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.Enabled || got.Host != "smtp.example.com" || got.Port != 465 || got.Username != "mailer" ||
		got.Password != "secret" || got.FromAddress != "noreply@example.com" || got.Security != EmailSecurityTLS {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	// Saving without updatePassword preserves the stored secret.
	in.Password = ""
	in.FromName = "Ticket Bot"
	if err := SetEmailConfig(ctx, db, in, false); err != nil {
		t.Fatalf("set no-pw: %v", err)
	}
	got, _ = GetEmailConfig(ctx, db)
	if got.Password != "secret" {
		t.Fatalf("password should be preserved, got %q", got.Password)
	}
	if got.FromName != "Ticket Bot" {
		t.Fatalf("from name not updated: %q", got.FromName)
	}
}

func TestEmailConfigValidation(t *testing.T) {
	ctx := context.Background()
	db := openEmailTestDB(t)

	if err := SetEmailConfig(ctx, db, EmailConfig{Port: 0, Security: EmailSecuritySTARTTLS}, true); err == nil {
		t.Fatal("port 0 should fail")
	}
	if err := SetEmailConfig(ctx, db, EmailConfig{Port: 25, Security: "bogus"}, true); err == nil {
		t.Fatal("bad security should fail")
	}
	if err := SetEmailConfig(ctx, db, EmailConfig{Enabled: true, Host: "", Port: 25, Security: EmailSecurityNone}, true); err == nil {
		t.Fatal("enabling without a host should fail")
	}
	// Enabling via SetEmailEnabled requires a configured host.
	if err := SetEmailEnabled(ctx, db, true); err == nil {
		t.Fatal("enable without host should fail")
	}
	if err := SetEmailConfig(ctx, db, EmailConfig{Host: "smtp.example.com", Port: 587, Security: EmailSecuritySTARTTLS}, true); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	if err := SetEmailEnabled(ctx, db, true); err != nil {
		t.Fatalf("enable with host: %v", err)
	}
	got, _ := GetEmailConfig(ctx, db)
	if !got.Enabled {
		t.Fatal("email should be enabled")
	}
}
