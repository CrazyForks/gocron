package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/app"
	"github.com/gocronx-team/gocron/internal/modules/setting"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

// setupAuthTest 构建挂载了 Auth + Handler 的 /mcp 引擎，并在内存库中放入一个用户与令牌。
// 返回明文令牌与清理函数。
func setupAuthTest(t *testing.T, userStatus models.Status, isAdmin int8) (*gin.Engine, string, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ApiToken{}, &models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	origDb, origInstalled, origSetting := models.Db, app.Installed, app.Setting
	models.Db = db
	app.Installed = true
	app.Setting = &setting.Setting{McpEnable: true}

	user := &models.User{Name: "alice", Status: userStatus, IsAdmin: isAdmin}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	// Status 列带 default:1，GORM 会用默认值覆盖零值（Disabled），这里显式回写确保状态准确。
	if err := db.Model(&models.User{}).Where("id = ?", user.Id).UpdateColumn("status", userStatus).Error; err != nil {
		t.Fatalf("set user status: %v", err)
	}
	plain := "gcx_integration_secret"
	token := &models.ApiToken{UserId: user.Id, Name: "t", TokenHash: models.HashToken(plain)}
	if err := token.Create(); err != nil {
		t.Fatalf("create token: %v", err)
	}

	r := gin.New()
	g := r.Group("/mcp")
	g.Use(Auth)
	g.Any("", gin.WrapH(Handler()))

	cleanup := func() {
		models.Db, app.Installed, app.Setting = origDb, origInstalled, origSetting
	}
	return r, plain, cleanup
}

func doMcp(r *gin.Engine, authHeader string) int {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestAuth_RejectsMissingAndBadToken(t *testing.T) {
	r, plain, cleanup := setupAuthTest(t, models.Enabled, 1)
	defer cleanup()

	if code := doMcp(r, ""); code != http.StatusUnauthorized {
		t.Errorf("no token: expected 401, got %d", code)
	}
	if code := doMcp(r, "Bearer wrong-token"); code != http.StatusUnauthorized {
		t.Errorf("bad token: expected 401, got %d", code)
	}
	if code := doMcp(r, "Basic "+plain); code != http.StatusUnauthorized {
		t.Errorf("wrong scheme: expected 401, got %d", code)
	}
}

func TestAuth_RejectsDisabledUser(t *testing.T) {
	r, plain, cleanup := setupAuthTest(t, models.Disabled, 1)
	defer cleanup()

	if code := doMcp(r, "Bearer "+plain); code != http.StatusUnauthorized {
		t.Errorf("disabled user: expected 401, got %d", code)
	}
}

func TestAuth_NotFoundWhenMcpDisabled(t *testing.T) {
	r, plain, cleanup := setupAuthTest(t, models.Enabled, 1)
	defer cleanup()
	app.Setting.McpEnable = false

	if code := doMcp(r, "Bearer "+plain); code != http.StatusNotFound {
		t.Errorf("mcp disabled: expected 404, got %d", code)
	}
}

func TestAuth_ServiceUnavailableWhenNotInstalled(t *testing.T) {
	r, plain, cleanup := setupAuthTest(t, models.Enabled, 1)
	defer cleanup()
	app.Installed = false

	if code := doMcp(r, "Bearer "+plain); code != http.StatusServiceUnavailable {
		t.Errorf("not installed: expected 503, got %d", code)
	}
}

func TestAuth_PassesValidTokenToHandler(t *testing.T) {
	r, plain, cleanup := setupAuthTest(t, models.Enabled, 1)
	defer cleanup()

	// 有效令牌应通过 Auth 进入 MCP Handler。裸 POST 未走 MCP 握手，
	// Handler 会以协议错误（4xx）回应，但关键是不再是鉴权失败的 401。
	code := doMcp(r, "Bearer "+plain)
	if code == http.StatusUnauthorized {
		t.Errorf("valid token should not be rejected by Auth, got 401")
	}
}
