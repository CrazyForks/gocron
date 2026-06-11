package manage

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/i18n"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/gocronx-team/gocron/internal/routers/base"
)

type updateLLMForm struct {
	Enable  bool   `json:"enable"`
	BaseURL string `json:"base_url"`
	ApiKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// LLM 返回大模型配置。出于安全，绝不回传 api_key 明文，仅返回是否已配置。
func LLM(c *gin.Context) {
	settingModel := new(models.Setting)
	cfg, err := settingModel.LLM()
	if err != nil {
		base.RespondErrorWithDefaultMsg(c, err)
		return
	}
	base.RespondSuccess(c, utils.SuccessContent, gin.H{
		"enable":      cfg.Enable,
		"base_url":    cfg.BaseURL,
		"model":       cfg.Model,
		"api_key_set": cfg.ApiKey != "",
	})
}

// UpdateLLM 更新大模型配置。api_key 留空表示不修改，沿用已保存的值。
func UpdateLLM(c *gin.Context) {
	var form updateLLMForm
	if err := c.ShouldBindJSON(&form); err != nil {
		base.RespondError(c, i18n.T(c, "param_error"))
		return
	}

	settingModel := new(models.Setting)
	apiKey := strings.TrimSpace(form.ApiKey)
	if apiKey == "" {
		existing, err := settingModel.LLM()
		if err != nil {
			base.RespondErrorWithDefaultMsg(c, err)
			return
		}
		apiKey = existing.ApiKey
	}

	if err := settingModel.UpdateLLM(form.Enable, strings.TrimSpace(form.BaseURL), apiKey, strings.TrimSpace(form.Model)); err != nil {
		base.RespondErrorWithDefaultMsg(c, err)
		return
	}
	base.RespondSuccessWithDefaultMsg(c, nil)
}
