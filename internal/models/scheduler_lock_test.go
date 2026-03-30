package models

import "testing"

func TestSchedulerLock_TableName(t *testing.T) {
	lock := SchedulerLock{}

	// Default: no prefix
	original := TablePrefix
	TablePrefix = ""
	defer func() { TablePrefix = original }()

	if got := lock.TableName(); got != "scheduler_lock" {
		t.Errorf("expected %q, got %q", "scheduler_lock", got)
	}

	// With prefix
	TablePrefix = "gocron_"
	if got := lock.TableName(); got != "gocron_scheduler_lock" {
		t.Errorf("expected %q, got %q", "gocron_scheduler_lock", got)
	}
}
