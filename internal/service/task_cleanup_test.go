package service

import (
	"testing"
	"time"

	"github.com/gocronx-team/gocron/internal/models"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupCleanupTestDB(t *testing.T) func() {
	t.Helper()
	originalDb := models.Db
	originalPrefix := models.TablePrefix

	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	models.TablePrefix = ""
	models.Db = db

	err = db.AutoMigrate(&models.Task{}, &models.TaskLog{})
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return func() {
		models.Db = originalDb
		models.TablePrefix = originalPrefix
	}
}

// TestTaskLevelRetentionBeforeGlobal verifies that tasks with custom retention
// days have their logs cleaned according to their own policy, and that the
// global policy applies to remaining logs.
func TestTaskLevelRetentionBeforeGlobal(t *testing.T) {
	cleanup := setupCleanupTestDB(t)
	defer cleanup()

	now := time.Now()

	// Task 1: custom retention of 3 days
	models.Db.Create(&models.Task{
		Name: "task-custom", Level: 1, Spec: "* * * * *",
		Protocol: 1, Command: "echo 1", LogRetentionDays: 3,
		Status: models.Enabled,
	})
	// Task 2: no custom retention (uses global)
	models.Db.Create(&models.Task{
		Name: "task-global", Level: 1, Spec: "* * * * *",
		Protocol: 1, Command: "echo 2", LogRetentionDays: 0,
		Status: models.Enabled,
	})

	oldTime5Days := models.LocalTime(now.AddDate(0, 0, -5))
	oldTime2Days := models.LocalTime(now.AddDate(0, 0, -2))

	// Task 1 logs: one 5-day old, one 2-day old
	models.Db.Create(&models.TaskLog{TaskId: 1, Name: "task-custom", Spec: "* * * * *", Protocol: 1, Command: "echo 1", Result: "ok", StartTime: oldTime5Days, Status: models.Finish})
	models.Db.Create(&models.TaskLog{TaskId: 1, Name: "task-custom", Spec: "* * * * *", Protocol: 1, Command: "echo 1", Result: "ok", StartTime: oldTime2Days, Status: models.Finish})

	// Task 2 logs: one 5-day old, one 2-day old
	models.Db.Create(&models.TaskLog{TaskId: 2, Name: "task-global", Spec: "* * * * *", Protocol: 1, Command: "echo 2", Result: "ok", StartTime: oldTime5Days, Status: models.Finish})
	models.Db.Create(&models.TaskLog{TaskId: 2, Name: "task-global", Spec: "* * * * *", Protocol: 1, Command: "echo 2", Result: "ok", StartTime: oldTime2Days, Status: models.Finish})

	// Step 1: Simulate task-level cleanup for tasks with custom retention
	var tasks []models.Task
	err := models.Db.Where("log_retention_days > 0").Find(&tasks).Error
	if err != nil {
		t.Fatalf("failed to query tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task with custom retention, got %d", len(tasks))
	}
	if tasks[0].Name != "task-custom" {
		t.Errorf("expected task-custom, got %s", tasks[0].Name)
	}

	taskLogModel := new(models.TaskLog)
	for _, task := range tasks {
		count, err := taskLogModel.RemoveByTaskIdAndDays(task.Id, task.LogRetentionDays)
		if err != nil {
			t.Fatalf("failed to cleanup task %d: %v", task.Id, err)
		}
		// Task 1 with 3-day retention should delete the 5-day old log
		if count != 1 {
			t.Errorf("expected 1 deleted for task %d, got %d", task.Id, count)
		}
	}

	// Verify task 1 has 1 remaining log (the 2-day old one)
	var task1Count int64
	models.Db.Model(&models.TaskLog{}).Where("task_id = ?", 1).Count(&task1Count)
	if task1Count != 1 {
		t.Errorf("expected 1 remaining log for task 1, got %d", task1Count)
	}

	// Task 2 should still have both logs (no custom cleanup was applied)
	var task2Count int64
	models.Db.Model(&models.TaskLog{}).Where("task_id = ?", 2).Count(&task2Count)
	if task2Count != 2 {
		t.Errorf("expected 2 remaining logs for task 2, got %d", task2Count)
	}

	// Step 2: Simulate global cleanup with 4-day retention
	globalDays := 4
	globalCount, err := taskLogModel.RemoveByDays(globalDays)
	if err != nil {
		t.Fatalf("global cleanup failed: %v", err)
	}
	// Only task 2's 5-day old log should be deleted (task 1's 5-day old log was already removed)
	if globalCount != 1 {
		t.Errorf("expected 1 deleted by global cleanup, got %d", globalCount)
	}

	// Final state: task 1 has 1 log, task 2 has 1 log
	var finalTask1 int64
	models.Db.Model(&models.TaskLog{}).Where("task_id = ?", 1).Count(&finalTask1)
	if finalTask1 != 1 {
		t.Errorf("expected 1 final log for task 1, got %d", finalTask1)
	}

	var finalTask2 int64
	models.Db.Model(&models.TaskLog{}).Where("task_id = ?", 2).Count(&finalTask2)
	if finalTask2 != 1 {
		t.Errorf("expected 1 final log for task 2, got %d", finalTask2)
	}
}

// TestTaskWithoutCustomRetentionUsesGlobal verifies that tasks without
// custom retention (log_retention_days=0) are not affected by task-level
// cleanup and only cleaned by global policy.
func TestTaskWithoutCustomRetentionUsesGlobal(t *testing.T) {
	cleanup := setupCleanupTestDB(t)
	defer cleanup()

	now := time.Now()
	oldTime := models.LocalTime(now.AddDate(0, 0, -10))

	// Task with no custom retention
	models.Db.Create(&models.Task{
		Name: "task-no-custom", Level: 1, Spec: "* * * * *",
		Protocol: 1, Command: "echo 1", LogRetentionDays: 0,
		Status: models.Enabled,
	})

	// Create old logs
	models.Db.Create(&models.TaskLog{TaskId: 1, Name: "task-no-custom", Spec: "* * * * *", Protocol: 1, Command: "echo 1", Result: "ok", StartTime: oldTime, Status: models.Finish})

	// Query tasks with custom retention - should find none
	var tasks []models.Task
	models.Db.Where("log_retention_days > 0").Find(&tasks)
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks with custom retention, got %d", len(tasks))
	}

	// Logs should still exist
	var count int64
	models.Db.Model(&models.TaskLog{}).Where("task_id = ?", 1).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 log before global cleanup, got %d", count)
	}

	// Global cleanup with 5-day retention should remove it
	taskLogModel := new(models.TaskLog)
	deleted, err := taskLogModel.RemoveByDays(5)
	if err != nil {
		t.Fatalf("global cleanup failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted by global cleanup, got %d", deleted)
	}
}
