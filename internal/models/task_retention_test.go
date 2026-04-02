package models

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupRetentionTestDB(t *testing.T) func() {
	t.Helper()
	originalDb := Db
	originalPrefix := TablePrefix

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	TablePrefix = ""
	Db = db

	// Create tables
	err = db.AutoMigrate(&Task{}, &TaskLog{})
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return func() {
		Db = originalDb
		TablePrefix = originalPrefix
	}
}

func TestRemoveByTaskIdAndDays_BasicCleanup(t *testing.T) {
	cleanup := setupRetentionTestDB(t)
	defer cleanup()

	now := time.Now()
	oldTime := LocalTime(now.AddDate(0, 0, -10))
	recentTime := LocalTime(now.AddDate(0, 0, -1))

	// Create logs for task 1: 2 old, 1 recent
	for i := 0; i < 2; i++ {
		Db.Create(&TaskLog{TaskId: 1, Name: "task1", Spec: "* * * * *", Protocol: 1, Command: "echo 1", Result: "ok", StartTime: oldTime, Status: Finish})
	}
	Db.Create(&TaskLog{TaskId: 1, Name: "task1", Spec: "* * * * *", Protocol: 1, Command: "echo 1", Result: "ok", StartTime: recentTime, Status: Finish})

	// Create logs for task 2: 2 old
	for i := 0; i < 2; i++ {
		Db.Create(&TaskLog{TaskId: 2, Name: "task2", Spec: "* * * * *", Protocol: 1, Command: "echo 2", Result: "ok", StartTime: oldTime, Status: Finish})
	}

	taskLog := new(TaskLog)

	// Remove logs older than 5 days for task 1 only
	count, err := taskLog.RemoveByTaskIdAndDays(1, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 deleted, got %d", count)
	}

	// Verify task 1 still has the recent log
	var task1Count int64
	Db.Model(&TaskLog{}).Where("task_id = ?", 1).Count(&task1Count)
	if task1Count != 1 {
		t.Errorf("expected 1 remaining log for task 1, got %d", task1Count)
	}

	// Verify task 2 logs are untouched
	var task2Count int64
	Db.Model(&TaskLog{}).Where("task_id = ?", 2).Count(&task2Count)
	if task2Count != 2 {
		t.Errorf("expected 2 remaining logs for task 2, got %d", task2Count)
	}
}

func TestRemoveByTaskIdAndDays_ZeroDays(t *testing.T) {
	cleanup := setupRetentionTestDB(t)
	defer cleanup()

	taskLog := new(TaskLog)
	count, err := taskLog.RemoveByTaskIdAndDays(1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestRemoveByTaskIdAndDays_ZeroTaskId(t *testing.T) {
	cleanup := setupRetentionTestDB(t)
	defer cleanup()

	taskLog := new(TaskLog)
	count, err := taskLog.RemoveByTaskIdAndDays(0, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestRemoveByTaskIdAndDays_NegativeInputs(t *testing.T) {
	cleanup := setupRetentionTestDB(t)
	defer cleanup()

	taskLog := new(TaskLog)

	count, err := taskLog.RemoveByTaskIdAndDays(-1, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for negative taskId, got %d", count)
	}

	count, err = taskLog.RemoveByTaskIdAndDays(1, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for negative days, got %d", count)
	}
}
