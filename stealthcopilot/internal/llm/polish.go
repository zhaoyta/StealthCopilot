// Package llm — polish.go 实现说话链 DeepSeek 文本润色：单次同步 HTTP 调用，
// 将讯飞翻译的中文译文润色为流利、专业的英文。
// 与流式回答生成（answer.go）分离，保持接口简洁：输入文本 → 输出润色文本，出错时降级返回原文。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// polishTimeout 单次润色请求超时，考虑到说话链延迟预算（≤1.2s 含翻译）设置为 5s。
// 如 DeepSeek 超时，handleSegment 降级使用原始翻译文本（不中断 TTS 流程）。
const polishTimeout = 5 * time.Second

// Polish 调用 DeepSeek 非流式接口对 input 文本进行润色，返回润色后的英文文本。
//
// 参数：
//   - ctx：父 context，通常来自说话链 handleSegment；
//   - apiKey/model：DeepSeek 凭据，来自应用配置；
//   - promptTpl：包含 {input} 占位符的润色 Prompt 模板（config.DefaultSpeakPolishPrompt）；
//   - input：讯飞翻译返回的英文译文（偶尔语法不流畅，需润色）。
//
// 返回值：润色后文本；失败时返回 (input, err)，调用方应降级使用原始文本。
func Polish(ctx context.Context, apiKey, model, promptTpl, input string) (string, error) {
	prompt := strings.ReplaceAll(promptTpl, "{input}", input)

	reqBody := polishRequest{
		Model: model,
		Messages: []llmMessage{
			{Role: "system", Content: prompt},
		},
		Stream: false,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	timeoutCtx, cancel := context.WithTimeout(ctx, polishTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(
		timeoutCtx, http.MethodPost, deepSeekChatURL, bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return input, fmt.Errorf("polish: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: polishTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return input, fmt.Errorf("polish: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return input, fmt.Errorf("polish: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return input, fmt.Errorf("polish: read body: %w", err)
	}

	var result polishResponse
	if err := json.Unmarshal(body, &result); err != nil || len(result.Choices) == 0 {
		return input, fmt.Errorf("polish: parse response: %w", err)
	}
	polished := strings.TrimSpace(result.Choices[0].Message.Content)
	if polished == "" {
		return input, fmt.Errorf("polish: empty response content")
	}
	return polished, nil
}

// polishRequest 是 DeepSeek 非流式 chat completions 请求结构（仅润色需要的字段）。
type polishRequest struct {
	Model    string       `json:"model"`
	Messages []llmMessage `json:"messages"`
	Stream   bool         `json:"stream"`
}

// polishResponse 是非流式 DeepSeek 响应结构。
type polishResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
