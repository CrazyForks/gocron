package tasklog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestDb(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	err = db.AutoMigrate(&models.TaskLog{})
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	models.Db = db
}

func TestClearByTaskId_InvalidId(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"non-numeric", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)
			r.POST("/api/task/log/clear/:id", ClearByTaskId)

			c.Request, _ = http.NewRequest("POST", "/api/task/log/clear/"+tt.id, nil)
			r.ServeHTTP(w, c.Request)

			body := w.Body.String()
			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
			// The response should indicate failure (code != 0)
			if !strings.Contains(body, `"code"`) {
				t.Errorf("expected JSON response with code field, got: %s", body)
			}
			// Should not contain success indicators for invalid input
			if strings.Contains(body, `"code":0`) {
				t.Errorf("expected error response for invalid id %q, got success: %s", tt.id, body)
			}
		})
	}
}

func TestClearByTaskId_ValidId(t *testing.T) {
	setupTestDb(t)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.POST("/api/task/log/clear/:id", ClearByTaskId)

	req, _ := http.NewRequest("POST", "/api/task/log/clear/1", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	// Should contain a successful JSON response
	if !strings.Contains(body, `"code":0`) {
		t.Errorf("expected success response for valid id, got: %s", body)
	}
}
