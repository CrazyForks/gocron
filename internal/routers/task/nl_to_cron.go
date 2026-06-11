package task

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/modules/i18n"
	"github.com/gocronx-team/gocron/internal/modules/llm"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"github.com/gocronx-team/gocron/internal/routers/base"
	"github.com/gocronx-team/gocron/internal/service"
)

const nlToCronTimeout = 30 * time.Second

const nlToCronSystemPrompt = `你是一个 cron 表达式生成器。把用户的自然语言调度描述转换为一个 cron 表达式。
严格遵守：
1. 只输出 cron 表达式本身，不要任何解释、引号、反引号或代码块。
2. 默认使用标准 5 字段格式：分 时 日 月 周。仅当用户明确要求精确到秒时，才使用 6 字段：秒 分 时 日 月 周。
3. 周字段中 0 或 7 表示周日，1-5 表示周一到周五。
4. 如果无法理解描述，只输出 ERROR。`

type nlToCronRequest struct {
	Text     string `json:"text" binding:"required"`
	Timezone string `json:"timezone"`
}

// NlToCron 把自然语言描述转换为 cron 表达式，并用 PreviewCron 校验后返回。
func NlToCron(c *gin.Context) {
	var req nlToCronRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		base.RespondValidationError(c, err)
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		base.RespondError(c, i18n.T(c, "param_error"))
		return
	}

	client, err := llm.FromSettings()
	if err != nil {
		base.RespondError(c, i18n.T(c, "llm_not_configured"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), nlToCronTimeout)
	defer cancel()

	answer, err := client.Chat(ctx, nlToCronSystemPrompt, text)
	if err != nil {
		logger.Errorf("NL转cron#调用LLM失败#%s", err)
		base.RespondError(c, i18n.T(c, "llm_call_failed"))
		return
	}

	spec := sanitizeCronSpec(answer)
	if spec == "" || strings.Contains(strings.ToUpper(spec), "ERROR") {
		base.RespondError(c, i18n.T(c, "nl_to_cron_failed"))
		return
	}

	preview := service.PreviewCron(spec, req.Timezone, 5)
	if !preview.Valid {
		logger.Errorf("NL转cron#生成非法表达式#spec-%q#err-%s", spec, preview.Error)
		base.RespondError(c, i18n.T(c, "nl_to_cron_invalid"))
		return
	}

	base.RespondSuccess(c, i18n.T(c, "operation_success"), gin.H{
		"spec":    spec,
		"preview": preview,
	})
}

// sanitizeCronSpec 去掉模型可能附带的代码块/反引号/多余空白，只保留单行表达式。
func sanitizeCronSpec(s string) string {
	s = strings.TrimSpace(s)
	// 先只取第一行，防止模型多输出解释
	if idx := strings.IndexAny(s, "\r\n"); idx >= 0 {
		s = s[:idx]
	}
	// 再去掉首尾可能包裹的反引号与空白
	s = strings.Trim(s, "`")
	return strings.TrimSpace(s)
}
