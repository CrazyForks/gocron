package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTaskLogTestDb(t *testing.T) func() {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	err = db.AutoMigrate(&TaskLog{})
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	originalDb := Db
	Db = db
	return func() {
		Db = originalDb
	}
}

func TestClearByTaskId_Normal(t *testing.T) {
	cleanup := setupTaskLogTestDb(t)
	defer cleanup()

	// Insert logs for task 1 and task 2
	for i := 0; i < 5; i++ {
		log := &TaskLog{TaskId: 1, Name: "task1", Spec: "* * * * *", Command: "echo 1", Result: "ok"}
		if _, err := log.Create(); err != nil {
			t.Fatalf("failed to create log: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		log := &TaskLog{TaskId: 2, Name: "task2", Spec: "* * * * *", Command: "echo 2", Result: "ok"}
		if _, err := log.Create(); err != nil {
			t.Fatalf("failed to create log: %v", err)
		}
	}

	taskLog := new(TaskLog)
	affected, err := taskLog.ClearByTaskId(1)
	if err != nil {
		t.Fatalf("ClearByTaskId returned error: %v", err)
	}
	if affected != 5 {
		t.Errorf("expected 5 affected rows, got %d", affected)
	}

	// Verify task 1 logs are gone
	var count int64
	Db.Model(&TaskLog{}).Where("task_id = ?", 1).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 remaining logs for task 1, got %d", count)
	}

	// Verify task 2 logs are untouched
	Db.Model(&TaskLog{}).Where("task_id = ?", 2).Count(&count)
	if count != 3 {
		t.Errorf("expected 3 remaining logs for task 2, got %d", count)
	}
}

func TestClearByTaskId_NoLogs(t *testing.T) {
	cleanup := setupTaskLogTestDb(t)
	defer cleanup()

	taskLog := new(TaskLog)
	affected, err := taskLog.ClearByTaskId(999)
	if err != nil {
		t.Fatalf("ClearByTaskId returned error: %v", err)
	}
	if affected != 0 {
		t.Errorf("expected 0 affected rows, got %d", affected)
	}
}

func TestClearByTaskId_ZeroId(t *testing.T) {
	cleanup := setupTaskLogTestDb(t)
	defer cleanup()

	taskLog := new(TaskLog)
	affected, err := taskLog.ClearByTaskId(0)
	if err != nil {
		t.Fatalf("ClearByTaskId returned error: %v", err)
	}
	if affected != 0 {
		t.Errorf("expected 0 affected rows, got %d", affected)
	}
}
