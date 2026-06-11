package mcptoken

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func setupTestRouter(t *testing.T, uid int) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	original := models.Db
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ApiToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	models.Db = db

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uid", uid)
		c.Next()
	})
	r.GET("/api/mcp-token", Index)
	r.POST("/api/mcp-token/store", Store)
	r.POST("/api/mcp-token/remove/:id", Remove)

	return r, func() { models.Db = original }
}

type envelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func TestStoreReturnsPlaintextOnce(t *testing.T) {
	r, cleanup := setupTestRouter(t, 1)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/mcp-token/store", strings.NewReader(`{"name":"my-laptop"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("expected code 0, got %d", env.Code)
	}
	var data struct {
		Id    int    `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data.Name != "my-laptop" {
		t.Errorf("name = %q", data.Name)
	}
	if !strings.HasPrefix(data.Token, "gcx_") {
		t.Errorf("token should have gcx_ prefix, got %q", data.Token)
	}

	// 数据库只存哈希，不存明文
	stored := &models.ApiToken{}
	if err := stored.FindByHash(models.HashToken(data.Token)); err != nil {
		t.Fatalf("token not found by hash: %v", err)
	}
	if stored.TokenHash == data.Token {
		t.Error("database must store the hash, not the plaintext token")
	}
}

func TestStoreDefaultsName(t *testing.T) {
	r, cleanup := setupTestRouter(t, 1)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/mcp-token/store", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	var data struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(env.Data, &data)
	if data.Name != "MCP Token" {
		t.Errorf("expected default name, got %q", data.Name)
	}
}

func TestIndexScopedToUser(t *testing.T) {
	r, cleanup := setupTestRouter(t, 1)
	defer cleanup()

	// user 1 的 token
	_ = (&models.ApiToken{UserId: 1, Name: "a", TokenHash: models.HashToken("a")}).Create()
	// user 2 的 token，不应出现
	_ = (&models.ApiToken{UserId: 2, Name: "b", TokenHash: models.HashToken("b")}).Create()

	req := httptest.NewRequest(http.MethodGet, "/api/mcp-token", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var list []models.ApiToken
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "a" {
		t.Fatalf("expected only user 1's token, got %+v", list)
	}
}

func TestRemoveOnlyOwnToken(t *testing.T) {
	r, cleanup := setupTestRouter(t, 1)
	defer cleanup()

	mine := &models.ApiToken{UserId: 1, Name: "mine", TokenHash: models.HashToken("mine")}
	_ = mine.Create()
	others := &models.ApiToken{UserId: 2, Name: "others", TokenHash: models.HashToken("others")}
	_ = others.Create()

	// 删除他人的 token：请求成功但不应删掉（Delete 限定 user_id）
	reqOther := httptest.NewRequest(http.MethodPost, "/api/mcp-token/remove/"+strconv.Itoa(others.Id), nil)
	wOther := httptest.NewRecorder()
	r.ServeHTTP(wOther, reqOther)

	remaining := &models.ApiToken{}
	if err := remaining.FindByHash(models.HashToken("others")); err != nil {
		t.Fatal("other user's token must not be deletable")
	}

	// 删除自己的 token：成功
	reqMine := httptest.NewRequest(http.MethodPost, "/api/mcp-token/remove/"+strconv.Itoa(mine.Id), nil)
	wMine := httptest.NewRecorder()
	r.ServeHTTP(wMine, reqMine)

	gone := &models.ApiToken{}
	if err := gone.FindByHash(models.HashToken("mine")); err == nil {
		t.Fatal("own token should be deleted")
	}
}
