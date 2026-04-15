package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type auditListData struct {
	Total int64             `json:"total"`
	Data  []models.AuditLog `json:"data"`
}

func setupAuditTestRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalDb := models.Db

	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	models.Db = db

	r := gin.New()
	r.GET("/api/audit", Index)

	cleanup := func() {
		models.Db = originalDb
	}

	return r, cleanup
}

func TestAuditIndex_Empty(t *testing.T) {
	r, cleanup := setupAuditTestRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	var data auditListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if data.Total != 0 {
		t.Errorf("expected total 0, got %d", data.Total)
	}
	if len(data.Data) != 0 {
		t.Errorf("expected empty list, got %d items", len(data.Data))
	}
}

func TestAuditIndex_WithData(t *testing.T) {
	r, cleanup := setupAuditTestRouter(t)
	defer cleanup()

	// Insert test records
	entries := []models.AuditLog{
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "host", Action: "delete"},
		{Username: "bob", Ip: "10.0.0.1", Module: "user", Action: "update"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	var data auditListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if data.Total != 3 {
		t.Errorf("expected total 3, got %d", data.Total)
	}
	if len(data.Data) != 3 {
		t.Errorf("expected 3 items, got %d", len(data.Data))
	}
}

func TestAuditIndex_FilterByModule(t *testing.T) {
	r, cleanup := setupAuditTestRouter(t)
	defer cleanup()

	entries := []models.AuditLog{
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "host", Action: "delete"},
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "delete"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit?module=task", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var data auditListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if data.Total != 2 {
		t.Errorf("expected total 2, got %d", data.Total)
	}
	for _, item := range data.Data {
		if item.Module != "task" {
			t.Errorf("expected module 'task', got '%s'", item.Module)
		}
	}
}

func TestAuditIndex_FilterByAction(t *testing.T) {
	r, cleanup := setupAuditTestRouter(t)
	defer cleanup()

	entries := []models.AuditLog{
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "host", Action: "delete"},
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit?action=create", nil)
	r.ServeHTTP(w, req)

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var data auditListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if data.Total != 2 {
		t.Errorf("expected total 2 for action=create, got %d", data.Total)
	}
}

func TestAuditIndex_FilterByUsername(t *testing.T) {
	r, cleanup := setupAuditTestRouter(t)
	defer cleanup()

	entries := []models.AuditLog{
		{Username: "alice", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "bob", Ip: "127.0.0.1", Module: "host", Action: "delete"},
		{Username: "alice_admin", Ip: "127.0.0.1", Module: "user", Action: "update"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit?username=alice", nil)
	r.ServeHTTP(w, req)

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var data auditListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if data.Total != 2 {
		t.Errorf("expected total 2 for username LIKE alice, got %d", data.Total)
	}
}

func TestAuditIndex_Pagination(t *testing.T) {
	r, cleanup := setupAuditTestRouter(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		log := &models.AuditLog{
			Username: "admin",
			Ip:       "127.0.0.1",
			Module:   "task",
			Action:   "create",
		}
		if _, err := log.Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit?page=1&page_size=3", nil)
	r.ServeHTTP(w, req)

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var data auditListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if data.Total != 5 {
		t.Errorf("expected total 5, got %d", data.Total)
	}
	if len(data.Data) != 3 {
		t.Errorf("expected 3 items on page 1, got %d", len(data.Data))
	}
}

func TestAuditIndex_MultipleFilters(t *testing.T) {
	r, cleanup := setupAuditTestRouter(t)
	defer cleanup()

	entries := []models.AuditLog{
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "create"},
		{Username: "admin", Ip: "127.0.0.1", Module: "task", Action: "delete"},
		{Username: "admin", Ip: "127.0.0.1", Module: "host", Action: "create"},
		{Username: "bob", Ip: "127.0.0.1", Module: "task", Action: "create"},
	}
	for i := range entries {
		if _, err := entries[i].Create(); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit?module=task&action=create&username=admin", nil)
	r.ServeHTTP(w, req)

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var data auditListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if data.Total != 1 {
		t.Errorf("expected total 1 for combined filters, got %d", data.Total)
	}
}
