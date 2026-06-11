package task

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// mockLLMServer 返回一个固定 content 的 OpenAI 兼容响应服务器。
func mockLLMServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": content}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func setupNlToCronRouter(t *testing.T, llmBaseURL string) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	original := models.Db
	models.Db = db
	if llmBaseURL != "" {
		if err := new(models.Setting).UpdateLLM(true, llmBaseURL, "sk-test", "gpt-test"); err != nil {
			t.Fatalf("seed llm config: %v", err)
		}
	}

	r := gin.New()
	r.POST("/api/task/nl-to-cron", NlToCron)
	return r, func() { models.Db = original }
}

func postNlToCron(r *gin.Engine, body string) map[string]any {
	req := httptest.NewRequest(http.MethodPost, "/api/task/nl-to-cron", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return env
}

func TestNlToCron_Success(t *testing.T) {
	srv := mockLLMServer(t, "0 9 * * 1-5")
	defer srv.Close()
	r, cleanup := setupNlToCronRouter(t, srv.URL)
	defer cleanup()

	env := postNlToCron(r, `{"text":"工作日早上9点"}`)
	if code, _ := env["code"].(float64); code != 0 {
		t.Fatalf("expected success code 0, got %v (msg=%v)", env["code"], env["message"])
	}
	data, _ := env["data"].(map[string]any)
	if data["spec"] != "0 9 * * 1-5" {
		t.Fatalf("expected spec, got %v", data["spec"])
	}
}

func TestNlToCron_StripsCodeFenceAndExtraLines(t *testing.T) {
	// 模型多嘴：带反引号和解释行，handler 应只取干净的第一行表达式
	srv := mockLLMServer(t, "`0 0 * * *`\n这是每天午夜")
	defer srv.Close()
	r, cleanup := setupNlToCronRouter(t, srv.URL)
	defer cleanup()

	env := postNlToCron(r, `{"text":"每天午夜"}`)
	data, _ := env["data"].(map[string]any)
	if data == nil || data["spec"] != "0 0 * * *" {
		t.Fatalf("expected sanitized spec '0 0 * * *', got %v", env)
	}
}

func TestNlToCron_NotConfigured(t *testing.T) {
	r, cleanup := setupNlToCronRouter(t, "") // 不配置 LLM
	defer cleanup()

	env := postNlToCron(r, `{"text":"每天午夜"}`)
	if code, _ := env["code"].(float64); code == 0 {
		t.Fatalf("expected failure when LLM not configured, got success")
	}
}

func TestNlToCron_InvalidExpression(t *testing.T) {
	srv := mockLLMServer(t, "not a cron at all")
	defer srv.Close()
	r, cleanup := setupNlToCronRouter(t, srv.URL)
	defer cleanup()

	env := postNlToCron(r, `{"text":"乱七八糟"}`)
	if code, _ := env["code"].(float64); code == 0 {
		t.Fatalf("expected failure for invalid cron, got success: %v", env)
	}
}
