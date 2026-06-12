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
	srv := mockLLMServer(t, `{"root_cause":"节点未安装 python3","suggestions":["安装 python3","使用绝对路径"]}`)
	defer srv.Close()
	r, cleanup := setupDiagnoseRouter(t, srv.URL)
	defer cleanup()

	id := seedLog(t, "/bin/sh: python3: not found\nexit status 127")
	env := callDiagnose(r, id)
	if code, _ := env["code"].(float64); code != 0 {
		t.Fatalf("expected success, got code=%v msg=%v", env["code"], env["message"])
	}
	data, _ := env["data"].(map[string]any)
	if rc, _ := data["root_cause"].(string); rc != "节点未安装 python3" {
		t.Fatalf("expected parsed root_cause, got %v", data["root_cause"])
	}
	sug, _ := data["suggestions"].([]any)
	if len(sug) != 2 {
		t.Fatalf("expected 2 suggestions, got %v", data["suggestions"])
	}
}

func TestDiagnose_FallbackOnNonJSON(t *testing.T) {
	// 模型没按 JSON 返回时，降级把原文放进 root_cause，前端仍能展示
	srv := mockLLMServer(t, "就是连不上而已")
	defer srv.Close()
	r, cleanup := setupDiagnoseRouter(t, srv.URL)
	defer cleanup()

	id := seedLog(t, "connection refused")
	env := callDiagnose(r, id)
	data, _ := env["data"].(map[string]any)
	if rc, _ := data["root_cause"].(string); rc != "就是连不上而已" {
		t.Fatalf("expected raw text fallback in root_cause, got %v", data["root_cause"])
	}
}

func TestDiagnose_ParsesJSONWithCodeFence(t *testing.T) {
	// 模型把 JSON 包在 ```json 代码块里也应能解析
	srv := mockLLMServer(t, "```json\n{\"root_cause\":\"端口未监听\",\"suggestions\":[\"启动服务\"]}\n```")
	defer srv.Close()
	r, cleanup := setupDiagnoseRouter(t, srv.URL)
	defer cleanup()

	id := seedLog(t, "dial tcp: connection refused")
	env := callDiagnose(r, id)
	data, _ := env["data"].(map[string]any)
	if rc, _ := data["root_cause"].(string); rc != "端口未监听" {
		t.Fatalf("expected fenced JSON to parse, got %v", data["root_cause"])
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
