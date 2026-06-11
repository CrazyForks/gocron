package manage

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

func setupLLMRouter(t *testing.T) (*gin.Engine, func()) {
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

	r := gin.New()
	r.GET("/api/system/llm", LLM)
	r.POST("/api/system/llm/update", UpdateLLM)
	return r, func() { models.Db = original }
}

func getLLM(r *gin.Engine) map[string]any {
	req := httptest.NewRequest(http.MethodGet, "/api/system/llm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := env["data"].(map[string]any)
	return data
}

func postLLM(r *gin.Engine, body string) {
	req := httptest.NewRequest(http.MethodPost, "/api/system/llm/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

func TestLLMConfig_GetNeverLeaksKey(t *testing.T) {
	r, cleanup := setupLLMRouter(t)
	defer cleanup()

	postLLM(r, `{"enable":true,"base_url":"https://api.example.com/v1","api_key":"sk-secret","model":"gpt-x"}`)

	data := getLLM(r)
	if data["enable"] != true || data["base_url"] != "https://api.example.com/v1" || data["model"] != "gpt-x" {
		t.Fatalf("unexpected config: %+v", data)
	}
	// 绝不回传明文 key，只暴露是否已配置
	if _, hasKey := data["api_key"]; hasKey {
		t.Fatal("response must not contain api_key field")
	}
	if data["api_key_set"] != true {
		t.Fatalf("expected api_key_set=true, got %v", data["api_key_set"])
	}
}

func TestLLMConfig_EmptyKeyKeepsExisting(t *testing.T) {
	r, cleanup := setupLLMRouter(t)
	defer cleanup()

	postLLM(r, `{"enable":true,"base_url":"https://a.com/v1","api_key":"sk-original","model":"m1"}`)
	// 二次更新不带 key（只改 model），应保留原 key
	postLLM(r, `{"enable":true,"base_url":"https://a.com/v1","api_key":"","model":"m2"}`)

	cfg, err := new(models.Setting).LLM()
	if err != nil {
		t.Fatalf("LLM: %v", err)
	}
	if cfg.ApiKey != "sk-original" {
		t.Fatalf("empty key should keep existing, got %q", cfg.ApiKey)
	}
	if cfg.Model != "m2" {
		t.Fatalf("model should update to m2, got %q", cfg.Model)
	}
}
