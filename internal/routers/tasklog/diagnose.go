package tasklog

import (
	"context"
	"encoding/json"
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
	diagnoseTimeout = 120 * time.Second
	maxResultRunes  = 4000 // 截断过长输出，控制 token 成本

	diagnoseSysPromptZH = `你是一位资深运维工程师。根据给出的定时任务配置及其失败的执行输出做诊断。
只输出一个 JSON 对象，不要 Markdown、不要代码块、不要任何多余文字。格式严格如下：
{"root_cause": "一句话根本原因", "suggestions": ["具体可操作的建议", "建议2"]}
要求：
- 用简体中文输出 root_cause 和 suggestions；root_cause 一句话；suggestions 每条简洁、可直接执行；不要复述输入。
- 不要假设目标节点的部署方式或初始化系统（systemd / service / docker / supervisor / 手动运行等都有可能），不要臆造服务名。涉及进程或服务排查时用通用表述（如"确认目标节点上的 agent 进程是否在运行、对应端口是否监听"），需要时可简要并列几种部署形态的检查方法，但不要断定只有某一种。`

	diagnoseSysPromptEN = `You are a senior DevOps engineer. Diagnose the failure based on the scheduled task config and its failed execution output.
Output ONLY a JSON object — no Markdown, no code fences, no extra text. Strict format:
{"root_cause": "one-sentence root cause", "suggestions": ["actionable suggestion", "suggestion 2"]}
Requirements:
- Write root_cause and suggestions in English; root_cause one sentence; each suggestion concise and directly actionable; do not restate the input.
- Do not assume the target node's deployment method or init system (systemd / service / docker / supervisor / manual run are all possible) and do not invent service names. For process/service checks use generic wording (e.g. "verify the agent process is running on the target node and the port is listening"); you may briefly list a few deployment forms but do not assert only one.`
)

// diagnoseSysPrompt 按请求语言返回对应的系统提示词。
func diagnoseSysPrompt(c *gin.Context) string {
	if i18n.GetLocale(c) == i18n.EnUS {
		return diagnoseSysPromptEN
	}
	return diagnoseSysPromptZH
}

// diagnosisResult 是结构化诊断结果，便于前端可控渲染（避免模型吐 Markdown 原文）。
type diagnosisResult struct {
	RootCause   string   `json:"root_cause"`
	Suggestions []string `json:"suggestions"`
}

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

	answer, err := client.Chat(ctx, diagnoseSysPrompt(c), buildDiagnosePrompt(logModel))
	if err != nil {
		logger.Errorf("日志诊断#调用LLM失败#日志ID-%d#%s", id, err)
		base.RespondError(c, i18n.T(c, "llm_call_failed"))
		return
	}

	d := parseDiagnosis(answer)
	base.RespondSuccess(c, i18n.T(c, "operation_success"), gin.H{
		"root_cause":  d.RootCause,
		"suggestions": d.Suggestions,
	})
}

// parseDiagnosis 从模型回复中提取 JSON 诊断；解析失败时降级把原文作为 root_cause 返回，前端仍可展示。
func parseDiagnosis(raw string) diagnosisResult {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	// 截取第一个 { 到最后一个 }，容忍模型在 JSON 前后夹带文字
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		s = s[i : j+1]
	}

	var d diagnosisResult
	if err := json.Unmarshal([]byte(s), &d); err == nil && (d.RootCause != "" || len(d.Suggestions) > 0) {
		return d
	}
	return diagnosisResult{RootCause: strings.TrimSpace(raw)}
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
