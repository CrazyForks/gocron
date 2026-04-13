package task

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

func setupTestRouter(t *testing.T) (*gin.Engine, func()) {
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

	err = db.AutoMigrate(&models.Task{})
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	models.Db = db

	r := gin.New()
	r.GET("/api/task/tags", GetAllTags)

	cleanup := func() {
		models.Db = originalDb
	}

	return r, cleanup
}

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestGetAllTagsHandler_Empty(t *testing.T) {
	r, cleanup := setupTestRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/task/tags", nil)
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

	var tags []string
	if err := json.Unmarshal(resp.Data, &tags); err != nil {
		t.Fatalf("failed to parse tags data: %v", err)
	}

	if len(tags) != 0 {
		t.Errorf("expected empty tags, got %v", tags)
	}
}

func TestGetAllTagsHandler_WithTags(t *testing.T) {
	r, cleanup := setupTestRouter(t)
	defer cleanup()

	// Insert test data
	tasks := []map[string]interface{}{
		{"name": "task1", "tag": "alpha,beta", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 1", "status": 1},
		{"name": "task2", "tag": "beta,gamma", "level": 1, "spec": "* * * * *", "protocol": 1, "command": "echo 2", "status": 1},
	}
	for _, data := range tasks {
		if err := models.Db.Model(&models.Task{}).Create(data).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/task/tags", nil)
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

	var tags []string
	if err := json.Unmarshal(resp.Data, &tags); err != nil {
		t.Fatalf("failed to parse tags data: %v", err)
	}

	expected := []string{"alpha", "beta", "gamma"}
	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d: %v", len(expected), len(tags), tags)
	}
	for i, tag := range tags {
		if tag != expected[i] {
			t.Errorf("expected tag[%d] = %q, got %q", i, expected[i], tag)
		}
	}
}
