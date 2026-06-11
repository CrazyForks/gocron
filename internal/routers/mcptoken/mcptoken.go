package mcptoken

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/i18n"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/gocronx-team/gocron/internal/routers/base"
	"github.com/gocronx-team/gocron/internal/routers/user"
)

// tokenPrefix 用于让 MCP 令牌在配置/日志中一眼可辨识。
const tokenPrefix = "gcx_"

// Index 列出当前用户的 MCP 令牌（不含明文与哈希）。
func Index(c *gin.Context) {
	tokenModel := new(models.ApiToken)
	list, err := tokenModel.ListByUser(user.Uid(c))
	if err != nil {
		base.RespondErrorWithDefaultMsg(c, err)
		return
	}
	base.RespondSuccess(c, utils.SuccessContent, list)
}

// Store 创建一个新的 MCP 令牌，明文仅在此处返回一次。
func Store(c *gin.Context) {
	var form struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&form)
	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = "MCP Token"
	}
	if len([]rune(name)) > 64 {
		base.RespondError(c, i18n.T(c, "param_error"))
		return
	}

	plain := generateToken()
	token := &models.ApiToken{
		UserId:    user.Uid(c),
		Name:      name,
		TokenHash: models.HashToken(plain),
	}
	if err := token.Create(); err != nil {
		base.RespondError(c, i18n.T(c, "operation_failed"), err)
		return
	}

	base.RespondSuccess(c, i18n.T(c, "operation_success"), map[string]interface{}{
		"id":    token.Id,
		"name":  token.Name,
		"token": plain,
	})
}

// Remove 吊销（删除）一个属于当前用户的 MCP 令牌。
func Remove(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		base.RespondError(c, i18n.T(c, "param_error"))
		return
	}
	tokenModel := new(models.ApiToken)
	if _, err := tokenModel.Delete(id, user.Uid(c)); err != nil {
		base.RespondErrorWithDefaultMsg(c, err)
		return
	}
	base.RespondSuccess(c, i18n.T(c, "operation_success"), nil)
}

// generateToken 生成 "gcx_" + 32 字节随机十六进制 的高熵令牌明文。
func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return tokenPrefix + hex.EncodeToString(b)
}
