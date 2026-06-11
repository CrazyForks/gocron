package mcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/app"
)

type ctxKey string

const userCtxKey ctxKey = "mcpUser"

// authUser 携带由令牌解析出的用户身份，供工具处理时做权限判断。
type authUser struct {
	Id      int
	Name    string
	IsAdmin bool
}

// userFromContext 从请求 context 取出已认证用户。
func userFromContext(ctx context.Context) (*authUser, bool) {
	u, ok := ctx.Value(userCtxKey).(*authUser)
	return u, ok
}

// Auth 是 /mcp 端点的 Bearer Token 鉴权中间件。
// 校验 Authorization: Bearer <token>，按哈希定位令牌、加载归属用户，
// 确认用户仍处于启用状态后，将身份写入请求 context 供下游 MCP 工具使用。
func Auth(c *gin.Context) {
	if !app.Installed {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if app.Setting == nil || !app.Setting.McpEnable {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	plain, ok := bearerToken(c.GetHeader("Authorization"))
	if !ok {
		unauthorized(c)
		return
	}

	tokenModel := new(models.ApiToken)
	if err := tokenModel.FindByHash(models.HashToken(plain)); err != nil {
		unauthorized(c)
		return
	}

	userModel := new(models.User)
	if err := userModel.Find(tokenModel.UserId); err != nil || userModel.Id == 0 {
		unauthorized(c)
		return
	}
	if userModel.Status != models.Enabled {
		unauthorized(c)
		return
	}

	tokenModel.TouchLastUsed()

	u := &authUser{Id: userModel.Id, Name: userModel.Name, IsAdmin: userModel.IsAdmin > 0}
	ctx := context.WithValue(c.Request.Context(), userCtxKey, u)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

// bearerToken 从 Authorization 头解析出 Bearer 令牌明文。
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	header = strings.TrimSpace(header)
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

func unauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatus(http.StatusUnauthorized)
}
