package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupTagTestDB(t *testing.T) func() {
	t.Helper()
	originalDb := Db

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&Task{})
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	Db = db

	return func() {
		Db = originalDb
	}
}

func TestGetAllTags_MultipleTags(t *testing.T) {
	cleanup := setupTagTestDB(t)
	defer cleanup()

	// Create tasks with various tag combinations
	tasks := []map[string]interface{}{
		{"name": "task1", "tag": "tag1", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 1", "status": 1},
		{"name": "task2", "tag": "tag1,tag2", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 2", "status": 1},
		{"name": "task3", "tag": "tag2,tag3", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 3", "status": 1},
	}
	for _, data := range tasks {
		if err := Db.Model(&Task{}).Create(data).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	taskModel := new(Task)
	tags, err := taskModel.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags returned error: %v", err)
	}

	expected := []string{"tag1", "tag2", "tag3"}
	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d: %v", len(expected), len(tags), tags)
	}
	for i, tag := range tags {
		if tag != expected[i] {
			t.Errorf("expected tag[%d] = %q, got %q", i, expected[i], tag)
		}
	}
}

func TestGetAllTags_EmptyTagsExcluded(t *testing.T) {
	cleanup := setupTagTestDB(t)
	defer cleanup()

	// Create tasks with empty and non-empty tags
	tasks := []map[string]interface{}{
		{"name": "task1", "tag": "", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 1", "status": 1},
		{"name": "task2", "tag": "mytag", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 2", "status": 1},
	}
	for _, data := range tasks {
		if err := Db.Model(&Task{}).Create(data).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	taskModel := new(Task)
	tags, err := taskModel.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags returned error: %v", err)
	}

	if len(tags) != 1 || tags[0] != "mytag" {
		t.Errorf("expected [\"mytag\"], got %v", tags)
	}
}

func TestGetAllTags_NoTasks(t *testing.T) {
	cleanup := setupTagTestDB(t)
	defer cleanup()

	taskModel := new(Task)
	tags, err := taskModel.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags returned error: %v", err)
	}

	if len(tags) != 0 {
		t.Errorf("expected empty list, got %v", tags)
	}
}

func TestGetAllTags_SingleTagBackwardCompatibility(t *testing.T) {
	cleanup := setupTagTestDB(t)
	defer cleanup()

	// Single tag (no comma) should still work
	data := map[string]interface{}{
		"name": "task1", "tag": "single", "level": 1, "spec": "* * * * *",
		"protocol": 1, "command": "echo 1", "status": 1,
	}
	if err := Db.Model(&Task{}).Create(data).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	taskModel := new(Task)
	tags, err := taskModel.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags returned error: %v", err)
	}

	if len(tags) != 1 || tags[0] != "single" {
		t.Errorf("expected [\"single\"], got %v", tags)
	}
}

func TestGetAllTags_Deduplication(t *testing.T) {
	cleanup := setupTagTestDB(t)
	defer cleanup()

	// Same tag appears in multiple tasks
	tasks := []map[string]interface{}{
		{"name": "task1", "tag": "common,unique1", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 1", "status": 1},
		{"name": "task2", "tag": "common,unique2", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 2", "status": 1},
	}
	for _, data := range tasks {
		if err := Db.Model(&Task{}).Create(data).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	taskModel := new(Task)
	tags, err := taskModel.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags returned error: %v", err)
	}

	expected := []string{"common", "unique1", "unique2"}
	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d: %v", len(expected), len(tags), tags)
	}
	for i, tag := range tags {
		if tag != expected[i] {
			t.Errorf("expected tag[%d] = %q, got %q", i, expected[i], tag)
		}
	}
}

func TestLikeQueryWithCommaSeparatedTags(t *testing.T) {
	cleanup := setupTagTestDB(t)
	defer cleanup()

	// Create tasks with comma-separated tags
	tasks := []map[string]interface{}{
		{"name": "task1", "tag": "backend,api", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 1", "status": 1},
		{"name": "task2", "tag": "frontend", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 2", "status": 1},
		{"name": "task3", "tag": "backend,cron", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 3", "status": 1},
	}
	for _, data := range tasks {
		if err := Db.Model(&Task{}).Create(data).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	// LIKE query for "backend" should match task1 and task3
	var results []Task
	err := Db.Where("tag LIKE ?", "%backend%").Find(&results).Error
	if err != nil {
		t.Fatalf("LIKE query returned error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for LIKE '%%backend%%', got %d", len(results))
	}

	// LIKE query for "api" should match only task1
	results = nil
	err = Db.Where("tag LIKE ?", "%api%").Find(&results).Error
	if err != nil {
		t.Fatalf("LIKE query returned error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for LIKE '%%api%%', got %d", len(results))
	}
}
