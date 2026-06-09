package main

import (
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
	u := models.User{Name: name, Password: hashed, Status: models.Enabled}
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
