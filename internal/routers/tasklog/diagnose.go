package tasklog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/i18n"
	"github.com/gocronx-team/gocron/internal/modules/llm"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"github.com/gocronx-team/gocron/internal/routers/base"
)

const (
	diagnoseTimeout   = 60 * time.Second
	maxResultRunes    = 4000 // 截断过长输出，控制 token 成本
	diagnoseSysPrompt = `你是一位资深运维工程师。根据给出的定时任务配置及其失败的执行输出，用简体中文给出诊断：
1. 根本原因：一句话点明。
2. 修复建议：分点列出，具体、可操作。
不要复述输入内容，保持简洁。`
)

// Diagnose 调用 LLM 对某条任务执行日志做失败归因与修复建议。
func Diagnose(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		base.RespondError(c, i18n.T(c, "param_error"))
		return
	}

	logModel := new(models.TaskLog)
	if err := logModel.Find(id); err != nil {
		base.RespondError(c, i18n.T(c, "log_not_found"))
		return
	}
	if strings.TrimSpace(logModel.Result) == "" {
		base.RespondError(c, i18n.T(c, "log_no_result"))
		return
	}

	client, err := llm.FromSettings()
	if err != nil {
		base.RespondError(c, i18n.T(c, "llm_not_configured"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), diagnoseTimeout)
	defer cancel()

	diagnosis, err := client.Chat(ctx, diagnoseSysPrompt, buildDiagnosePrompt(logModel))
	if err != nil {
		logger.Errorf("日志诊断#调用LLM失败#日志ID-%d#%s", id, err)
		base.RespondError(c, i18n.T(c, "llm_call_failed"))
		return
	}

	base.RespondSuccess(c, i18n.T(c, "operation_success"), gin.H{"diagnosis": diagnosis})
}

func buildDiagnosePrompt(log *models.TaskLog) string {
	protocol := "HTTP"
	if log.Protocol == models.TaskRPC {
		protocol = "RPC(Shell)"
	}
	result := strings.TrimSpace(log.Result)
	if r := []rune(result); len(r) > maxResultRunes {
		result = string(r[:maxResultRunes]) + "\n...(输出过长已截断)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "任务名称：%s\n", log.Name)
	fmt.Fprintf(&b, "执行协议：%s\n", protocol)
	fmt.Fprintf(&b, "执行命令：%s\n", log.Command)
	if log.Hostname != "" {
		fmt.Fprintf(&b, "执行节点：%s\n", log.Hostname)
	}
	fmt.Fprintf(&b, "执行输出：\n%s", result)
	return b.String()
}
