package tasklog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	logger.InitLogger()
	os.Exit(m.Run())
}

func mockLLMServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": content}},
			},
		})
	}))
}

func setupDiagnoseRouter(t *testing.T, llmBaseURL string) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Setting{}, &models.TaskLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	original := models.Db
	models.Db = db
	if llmBaseURL != "" {
		if err := new(models.Setting).UpdateLLM(true, llmBaseURL, "sk-test", "gpt-test"); err != nil {
			t.Fatalf("seed llm: %v", err)
		}
	}

	r := gin.New()
	r.POST("/api/task/log/diagnose/:id", Diagnose)
	return r, func() { models.Db = original }
}

func seedLog(t *testing.T, result string) int64 {
	t.Helper()
	// 显式指定 Id：sqlite 对 bigint 主键不会自增（生产环境另有修复），测试里直接给定。
	log := &models.TaskLog{Id: 1, Name: "backup", Protocol: models.TaskRPC, Command: "python3 backup.py", Result: result}
	if _, err := log.Create(); err != nil {
		t.Fatalf("seed log: %v", err)
	}
	return log.Id
}

func callDiagnose(r *gin.Engine, id int64) map[string]any {
	req := httptest.NewRequest(http.MethodPost, "/api/task/log/diagnose/"+strconv.FormatInt(id, 10), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return env
}

func TestDiagnose_Success(t *testing.T) {
	srv := mockLLMServer(t, "根本原因：节点未安装 python3。建议：安装 python3。")
	defer srv.Close()
	r, cleanup := setupDiagnoseRouter(t, srv.URL)
	defer cleanup()

	id := seedLog(t, "/bin/sh: python3: not found\nexit status 127")
	env := callDiagnose(r, id)
	if code, _ := env["code"].(float64); code != 0 {
		t.Fatalf("expected success, got code=%v msg=%v", env["code"], env["message"])
	}
	data, _ := env["data"].(map[string]any)
	if s, _ := data["diagnosis"].(string); s == "" {
		t.Fatalf("expected diagnosis text, got %v", data)
	}
}

func TestDiagnose_NotFound(t *testing.T) {
	srv := mockLLMServer(t, "x")
	defer srv.Close()
	r, cleanup := setupDiagnoseRouter(t, srv.URL)
	defer cleanup()

	env := callDiagnose(r, 9999)
	if code, _ := env["code"].(float64); code == 0 {
		t.Fatal("expected failure for missing log")
	}
}

func TestDiagnose_NoResult(t *testing.T) {
	srv := mockLLMServer(t, "x")
	defer srv.Close()
	r, cleanup := setupDiagnoseRouter(t, srv.URL)
	defer cleanup()

	id := seedLog(t, "   ") // 空白输出
	env := callDiagnose(r, id)
	if code, _ := env["code"].(float64); code == 0 {
		t.Fatal("expected failure when log has no result")
	}
}

func TestDiagnose_NotConfigured(t *testing.T) {
	r, cleanup := setupDiagnoseRouter(t, "")
	defer cleanup()

	id := seedLog(t, "some error output")
	env := callDiagnose(r, id)
	if code, _ := env["code"].(float64); code == 0 {
		t.Fatal("expected failure when LLM not configured")
	}
}
