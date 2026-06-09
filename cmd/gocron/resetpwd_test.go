package main

import (
	"strings"
	"testing"

	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

// newTestDB spins up an in-memory SQLite DB with the user table migrated.
// SQLite in-memory is per-connection, so force a single connection.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	return db
}

// seedUser inserts a user with a bcrypt-hashed password and optional 2FA on.
func seedUser(t *testing.T, db *gorm.DB, name, password string, twoFactorOn bool) {
	t.Helper()
	hashed, err := utils.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.User{Name: name, Email: name + "@example.com", Password: hashed, Status: models.Enabled}
	if twoFactorOn {
		u.TwoFactorKey = "SOMEKEY"
		u.TwoFactorOn = 1
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func TestResetUserPassword_UpdatesPassword(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "admin", "oldpass", false)

	user, err := resetUserPassword(db, "admin", "newpass123", false)
	if err != nil {
		t.Fatalf("resetUserPassword returned error: %v", err)
	}
	if user.Name != "admin" {
		t.Fatalf("expected returned user admin, got %q", user.Name)
	}

	var got models.User
	if err := db.Where("name = ?", "admin").First(&got).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !utils.VerifyPassword(got.Password, "newpass123", got.Salt) {
		t.Errorf("new password does not verify")
	}
	if utils.VerifyPassword(got.Password, "oldpass", got.Salt) {
		t.Errorf("old password still verifies after reset")
	}
}

func TestResetUserPassword_UserNotFound(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "admin", "oldpass", false)

	if _, err := resetUserPassword(db, "ghost", "whatever", false); err == nil {
		t.Fatalf("expected error for missing user, got nil")
	}
}

func TestFindUserByName(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "admin", "oldpass", false)

	user, err := findUserByName(db, "admin")
	if err != nil {
		t.Fatalf("findUserByName(admin) returned error: %v", err)
	}
	if user.Name != "admin" {
		t.Errorf("expected admin, got %q", user.Name)
	}

	if _, err := findUserByName(db, "ghost"); err == nil {
		t.Fatalf("expected error for missing user, got nil")
	}
}

func TestResetUserPassword_DisableTwoFactor(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "admin", "oldpass", true)

	if _, err := resetUserPassword(db, "admin", "newpass123", true); err != nil {
		t.Fatalf("resetUserPassword returned error: %v", err)
	}

	var got models.User
	if err := db.Where("name = ?", "admin").First(&got).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if got.TwoFactorOn != 0 {
		t.Errorf("expected two_factor_on=0, got %d", got.TwoFactorOn)
	}
	if got.TwoFactorKey != "" {
		t.Errorf("expected empty two_factor_key, got %q", got.TwoFactorKey)
	}
}

func TestResetUserPassword_KeepsTwoFactorWhenNotRequested(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "admin", "oldpass", true)

	if _, err := resetUserPassword(db, "admin", "newpass123", false); err != nil {
		t.Fatalf("resetUserPassword returned error: %v", err)
	}

	var got models.User
	if err := db.Where("name = ?", "admin").First(&got).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if got.TwoFactorOn != 1 {
		t.Errorf("expected two_factor_on untouched (1), got %d", got.TwoFactorOn)
	}
}

func TestListUsernames(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "admin", "p", false)
	seedUser(t, db, "alice", "p", false)

	names, err := listUsernames(db)
	if err != nil {
		t.Fatalf("listUsernames returned error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 usernames, got %d (%v)", len(names), names)
	}
	got := strings.Join(names, ",")
	if got != "admin,alice" {
		t.Errorf("expected ordered \"admin,alice\", got %q", got)
	}
}

func TestListUsernames_Empty(t *testing.T) {
	db := newTestDB(t)
	names, err := listUsernames(db)
	if err != nil {
		t.Fatalf("listUsernames returned error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected no usernames, got %v", names)
	}
}

func TestResolveSQLitePath(t *testing.T) {
	base := "/opt/gocron"
	cases := []struct {
		name     string
		engine   string
		database string
		want     string
	}{
		{"relative sqlite anchored to base", "sqlite", "data/gocron.db", "/opt/gocron/data/gocron.db"},
		{"engine case-insensitive", "SQLite", "data/gocron.db", "/opt/gocron/data/gocron.db"},
		{"absolute sqlite left as-is", "sqlite", "/var/lib/gocron.db", "/var/lib/gocron.db"},
		{"mysql database name untouched", "mysql", "gocron", "gocron"},
		{"empty left as-is", "sqlite", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveSQLitePath(c.engine, c.database, base); got != c.want {
				t.Errorf("resolveSQLitePath(%q,%q,%q) = %q, want %q", c.engine, c.database, base, got, c.want)
			}
		})
	}
}

func TestGenerateRandomPassword_LengthAndUsable(t *testing.T) {
	pw := generateRandomPassword()
	if len(pw) != randomPasswordLength {
		t.Fatalf("expected length %d, got %d (%q)", randomPasswordLength, len(pw), pw)
	}
	hashed, err := utils.HashPassword(pw)
	if err != nil {
		t.Fatalf("hash generated password: %v", err)
	}
	if !utils.VerifyPassword(hashed, pw, "") {
		t.Errorf("generated password does not verify against its own hash")
	}
}
