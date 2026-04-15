package models

import (
	"testing"
	"time"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupAuditLogTestDB(t *testing.T) func() {
	t.Helper()
	originalDb := Db

	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&AuditLog{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	Db = db

	return func() {
		Db = originalDb
	}
}

func TestAuditLog_Create(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	log := &AuditLog{
		Username:   "admin",
		Ip:         "127.0.0.1",
		Module:     "task",
		Action:     "create",
		TargetId:   1,
		TargetName: "my-task",
		Detail:     "created task my-task",
	}

	insertId, err := log.Create()
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if insertId <= 0 {
		t.Errorf("expected insertId > 0, got %d", insertId)
	}
}

func TestAuditLog_List_Empty(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	auditLog := new(AuditLog)
	params := CommonMap{"Page": 1, "PageSize": 20}
	list, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestAuditLog_List_Pagination(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	// Insert 5 records
	for i := 0; i < 5; i++ {
		log := &AuditLog{
			Username: "admin",
			Ip:       "127.0.0.1",
			Module:   "task",
			Action:   "create",
		}
		if _, err := log.Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	auditLog := new(AuditLog)
	params := CommonMap{"Page": 1, "PageSize": 3}
	list, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 items (page size), got %d", len(list))
	}

	// Page 2 should have the remaining 2
	params["Page"] = 2
	list2, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List page 2 returned error: %v", err)
	}
	if len(list2) != 2 {
		t.Errorf("expected 2 items on page 2, got %d", len(list2))
	}
}

func TestAuditLog_List_FilterByModule(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	entries := []AuditLog{
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "host", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "delete"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	auditLog := new(AuditLog)
	params := CommonMap{"Page": 1, "PageSize": 20, "Module": "task"}
	list, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 task entries, got %d", len(list))
	}
	for _, item := range list {
		if item.Module != "task" {
			t.Errorf("expected module 'task', got '%s'", item.Module)
		}
	}
}

func TestAuditLog_List_FilterByAction(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	entries := []AuditLog{
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "delete"},
		{Username: "admin", Ip: "127.0.0.1", Module: "host", Action: "create"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	auditLog := new(AuditLog)
	params := CommonMap{"Page": 1, "PageSize": 20, "Action": "delete"}
	list, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 delete entry, got %d", len(list))
	}
}

func TestAuditLog_List_FilterByUsername(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	entries := []AuditLog{
		{Username: "alice", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "bob", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "alice_admin", Ip: "127.0.0.1", Module: "host", Action: "delete"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	auditLog := new(AuditLog)
	// LIKE match: "alice" matches "alice" and "alice_admin"
	params := CommonMap{"Page": 1, "PageSize": 20, "Username": "alice"}
	list, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 alice entries, got %d", len(list))
	}
}

func TestAuditLog_List_FilterByDateRange(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	// Insert records with specific timestamps via raw insert
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02 15:04:05")
	twoDaysAgo := now.AddDate(0, 0, -2).Format("2006-01-02 15:04:05")

	Db.Exec("INSERT INTO audit_log (username, ip, module, action, target_id, target_name, detail, created) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"admin", "127.0.0.1", "task", "create", 0, "", "", yesterday)
	Db.Exec("INSERT INTO audit_log (username, ip, module, action, target_id, target_name, detail, created) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"admin", "127.0.0.1", "task", "delete", 0, "", "", twoDaysAgo)

	auditLog := new(AuditLog)
	// Filter to only yesterday
	startDate := now.AddDate(0, 0, -1).Format("2006-01-02") + " 00:00:00"
	endDate := tomorrow
	params := CommonMap{"Page": 1, "PageSize": 20, "StartDate": startDate, "EndDate": endDate}
	list, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 entry in date range, got %d", len(list))
	}
}

func TestAuditLog_Total(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	// Empty DB
	auditLog := new(AuditLog)
	params := CommonMap{}
	total, err := auditLog.Total(params)
	if err != nil {
		t.Fatalf("Total returned error: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0, got %d", total)
	}

	// Insert 3 records
	for i := 0; i < 3; i++ {
		log := &AuditLog{
			Username: "admin",
			Ip:       "127.0.0.1",
			Module:   "task",
			Action:   "create",
		}
		if _, err := log.Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	total, err = auditLog.Total(params)
	if err != nil {
		t.Fatalf("Total returned error: %v", err)
	}
	if total != 3 {
		t.Errorf("expected 3, got %d", total)
	}
}

func TestAuditLog_Total_WithFilter(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	entries := []AuditLog{
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "host", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "delete"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	auditLog := new(AuditLog)
	params := CommonMap{"Module": "task"}
	total, err := auditLog.Total(params)
	if err != nil {
		t.Fatalf("Total returned error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 task entries, got %d", total)
	}
}

func TestAuditLog_List_OrderByIdDesc(t *testing.T) {
	cleanup := setupAuditLogTestDB(t)
	defer cleanup()

	for i := 0; i < 3; i++ {
		log := &AuditLog{
			Username: "admin",
			Ip:       "127.0.0.1",
			Module:   "task",
			Action:   "create",
		}
		if _, err := log.Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	auditLog := new(AuditLog)
	params := CommonMap{"Page": 1, "PageSize": 10}
	list, err := auditLog.List(params)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(list))
	}
	// Verify descending order
	for i := 1; i < len(list); i++ {
		if list[i].Id > list[i-1].Id {
			t.Errorf("expected descending order by id, but list[%d].Id=%d > list[%d].Id=%d",
				i, list[i].Id, i-1, list[i-1].Id)
		}
	}
}
