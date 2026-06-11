// Package llm 提供一个最小的 OpenAI 兼容 Chat Completions 客户端，
// 用于自然语言转 cron、失败日志诊断等产品内 AI 功能。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

var (
	// ErrNotConfigured 表示 LLM 未启用或配置不完整。
	ErrNotConfigured = errors.New("llm not configured")
	// ErrEmptyResponse 表示模型返回了空内容。
	ErrEmptyResponse = errors.New("llm returned empty response")
)

// Client 是一个最小的 OpenAI 兼容 Chat Completions 客户端。
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// New 创建客户端。baseURL 形如 https://api.openai.com/v1。
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 发送一次单轮对话，返回模型回复文本。
// 调用方应通过 ctx 控制取消/超时；客户端自身也带有默认超时。
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	reqBody := chatRequest{
		Model:       c.model,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("call llm: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("llm error (status %d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("llm http status %d", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return "", ErrEmptyResponse
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", ErrEmptyResponse
	}
	return content, nil
}
