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

// 注意：gocron 使用秒级 cron，字段顺序为「秒 分 时 日 月 周」，第一个字段永远是秒。
// 这与标准 5 字段 Unix cron（分 时 日 月 周）不同，必须让模型按本格式输出，否则会在错误时间执行。
const nlToCronSystemPrompt = `你是一个 cron 表达式生成器，目标系统使用「秒级」cron。字段顺序固定为 6 个：秒 分 时 日 月 周（秒在最前）。
严格遵守：
1. 只输出 cron 表达式本身，不要任何解释、引号、反引号或代码块。
2. 必须输出 6 个字段：秒 分 时 日 月 周。不要输出标准的 5 字段 Unix cron。
3. 不需要精确到秒时，秒字段填 0。
4. 周字段中 0 表示周日，1-5 表示周一到周五，6 表示周六。
5. 如果无法理解描述，只输出 ERROR。

示例：
每分钟 -> 0 * * * * *
每 5 分钟 -> 0 */5 * * * *
每 20 秒 -> */20 * * * * *
每天早上 9 点半 -> 0 30 9 * * *
工作日早上 9 点 -> 0 0 9 * * 1-5
每周一凌晨 3 点 -> 0 0 3 * * 1
每月 1 号零点 -> 0 0 0 1 * *`

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

	// 强制 6 字段（秒 分 时 日 月 周）或 @ 描述符。
	// 5 字段标准 cron 虽语法合法，但会被引擎当作「秒 分 时 日 月」误读、在错误时间执行，必须拦截。
	if !isSixFieldOrDescriptor(spec) {
		logger.Errorf("NL转cron#字段数不符合秒级6字段#spec-%q", spec)
		base.RespondError(c, i18n.T(c, "nl_to_cron_invalid"))
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

// isSixFieldOrDescriptor 校验是否为 6 字段秒级表达式，或 @ 开头的描述符（如 @daily）。
func isSixFieldOrDescriptor(spec string) bool {
	if strings.HasPrefix(spec, "@") {
		return true
	}
	return len(strings.Fields(spec)) == 6
}
