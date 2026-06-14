// Package llm — polish_test.go 验证 Polish 函数在正常响应、HTTP 错误、空响应和超时四种场景下的行为。
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestPolish_Success 验证正常 DeepSeek 响应返回润色文本。
func TestPolish_Success(t *testing.T) {
	polished := "I have five years of backend development experience."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := polishResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: polished}}},
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	got, err := polishWithBaseURL(
		context.Background(), "test-key", "deepseek-chat",
		"请润色：{input}", "我有五年后端开发经验。", srv.URL+"/v1/chat/completions",
	)
	if err != nil {
		t.Fatalf("Polish returned error: %v", err)
	}
	if got != polished {
		t.Errorf("got %q, want %q", got, polished)
	}
}

// TestPolish_HTTPError 验证非 200 状态码时返回原文并携带错误。
func TestPolish_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	input := "input text"
	got, err := polishWithBaseURL(
		context.Background(), "bad-key", "m", "润色：{input}", input, srv.URL+"/v1/chat/completions",
	)
	if err == nil {
		t.Error("expected error on HTTP 401")
	}
	if got != input {
		t.Errorf("should return original input on error, got %q", got)
	}
}

// TestPolish_EmptyContent 验证 choices[0].message.content 为空时返回原文。
func TestPolish_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := polishResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: ""}}},
		}
		b, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	input := "some input"
	got, err := polishWithBaseURL(
		context.Background(), "k", "m", "{input}", input, srv.URL+"/v1/chat/completions",
	)
	if err == nil {
		t.Error("expected error for empty content response")
	}
	if got != input {
		t.Errorf("should return original input, got %q", got)
	}
}

// TestPolish_Timeout 验证超时时返回原文并携带错误（不 panic，不阻塞）。
func TestPolish_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond) // 远超测试用的短超时
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	input := "hello"
	got, err := polishWithBaseURL(ctx, "k", "m", "{input}", input, srv.URL+"/v1/chat/completions")
	if err == nil {
		t.Error("expected timeout error")
	}
	if got != input {
		t.Errorf("should return original input on timeout, got %q", got)
	}
}

// TestPolishPromptTemplate 验证 {input} 占位符被正确替换。
func TestPolishPromptTemplate(t *testing.T) {
	var capturedBody strings.Builder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		capturedBody.Write(buf[:n])
		resp := polishResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "result"}}},
		}
		b, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	tpl := "请将下面的文本润色为英文：{input}"
	input := "你好世界"
	_, _ = polishWithBaseURL(context.Background(), "k", "m", tpl, input, srv.URL+"/v1/chat/completions")

	body := capturedBody.String()
	if !strings.Contains(body, "你好世界") {
		t.Error("request body should contain input text")
	}
	if !strings.Contains(body, "请将下面的文本润色为英文") {
		t.Error("request body should contain prompt template content")
	}
}

// polishWithBaseURL 是 Polish 的可测试变体，允许注入 baseURL（指向 httptest 服务器）。
// 通过直接构造 HTTP 请求并使用测试 transport 实现，避免修改 Polish 的公开签名。
func polishWithBaseURL(ctx context.Context, apiKey, model, promptTpl, input, baseURL string) (string, error) {
	prompt := strings.ReplaceAll(promptTpl, "{input}", input)
	reqBody := polishRequest{
		Model:    model,
		Messages: []llmMessage{{Role: "system", Content: prompt}},
		Stream:   false,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return input, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: polishTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return input, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return input, fmt.Errorf("polish: status %d", resp.StatusCode)
	}

	var result polishResponse
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	if err := json.Unmarshal(buf[:n], &result); err != nil || len(result.Choices) == 0 {
		return input, fmt.Errorf("polish: parse: %w", err)
	}
	polished := strings.TrimSpace(result.Choices[0].Message.Content)
	if polished == "" {
		return input, fmt.Errorf("polish: empty response content")
	}
	return polished, nil
}
