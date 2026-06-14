// Package llm 单测：覆盖事件常量、SSE token 解析、Prompt 构建、对话历史管理和流式生成逻辑。
package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestEventConstants 验证向前端推送的事件名未被意外修改。
func TestEventConstants(t *testing.T) {
	if EventAnswerToken != "answer:token" {
		t.Errorf("EventAnswerToken = %q, want %q", EventAnswerToken, "answer:token")
	}
	if EventAnswerDone != "answer:done" {
		t.Errorf("EventAnswerDone = %q, want %q", EventAnswerDone, "answer:done")
	}
}

// TestExtractSSEToken_Valid 验证标准 DeepSeek SSE payload 的 token 提取。
func TestExtractSSEToken_Valid(t *testing.T) {
	payload := `{"choices":[{"delta":{"content":"Hello"}}]}`
	got := extractSSEToken(payload)
	if got != "Hello" {
		t.Errorf("extractSSEToken = %q, want %q", got, "Hello")
	}
}

// TestExtractSSEToken_EmptyContent 验证 delta.content 为空字符串时返回空。
func TestExtractSSEToken_EmptyContent(t *testing.T) {
	payload := `{"choices":[{"delta":{"content":""}}]}`
	got := extractSSEToken(payload)
	if got != "" {
		t.Errorf("expected empty token, got %q", got)
	}
}

// TestExtractSSEToken_InvalidJSON 验证非法 JSON 不 panic，返回空字符串。
func TestExtractSSEToken_InvalidJSON(t *testing.T) {
	got := extractSSEToken("not-json")
	if got != "" {
		t.Errorf("expected empty token for invalid JSON, got %q", got)
	}
}

// TestExtractSSEToken_NoChoices 验证 choices 为空数组时返回空。
func TestExtractSSEToken_NoChoices(t *testing.T) {
	got := extractSSEToken(`{"choices":[]}`)
	if got != "" {
		t.Errorf("expected empty token for empty choices, got %q", got)
	}
}

// TestBuildSystemPrompt_WithChunks 验证 RAG 片段和问题被正确插入模板。
func TestBuildSystemPrompt_WithChunks(t *testing.T) {
	chunks := []string{"chunk1", "chunk2"}
	prompt := buildSystemPrompt(chunks, "Tell me about yourself", "{resume_context}\n{question}", nil)
	if !strings.Contains(prompt, "chunk1") || !strings.Contains(prompt, "chunk2") {
		t.Error("prompt should contain resume chunks")
	}
	if !strings.Contains(prompt, "Tell me about yourself") {
		t.Error("prompt should contain the question")
	}
}

// TestBuildSystemPrompt_DefaultTemplate 验证未传模板时使用内置默认模板。
func TestBuildSystemPrompt_DefaultTemplate(t *testing.T) {
	prompt := buildSystemPrompt(nil, "What is your strength?", "", nil)
	if !strings.Contains(prompt, "What is your strength?") {
		t.Error("default prompt should contain the question")
	}
	if !strings.Contains(prompt, "简历内容") {
		t.Error("default prompt should contain '简历内容' header")
	}
}

// TestBuildSystemPrompt_WithHistory 验证对话历史被追加到 prompt 末尾。
func TestBuildSystemPrompt_WithHistory(t *testing.T) {
	history := []QAPair{
		{Question: "Q1", Answer: "A1"},
	}
	prompt := buildSystemPrompt(nil, "follow-up", "{resume_context}\n{question}", history)
	if !strings.Contains(prompt, "Q: Q1") || !strings.Contains(prompt, "A: A1") {
		t.Errorf("prompt should contain dialog history, got: %s", prompt)
	}
}

// TestAnswerGenerator_History 验证多轮对话历史的存储和滚动删除逻辑。
func TestAnswerGenerator_History(t *testing.T) {
	g := NewAnswerGenerator("", "", func(_ string, _ ...any) {})

	// 写入超过 historyMaxTurns 轮
	for i := range historyMaxTurns + 2 {
		g.appendHistory("sess1", fmt.Sprintf("Q%d", i), fmt.Sprintf("A%d", i))
	}

	history := g.getHistory("sess1", true)
	if len(history) != historyMaxTurns {
		t.Errorf("history len = %d, want %d (historyMaxTurns)", len(history), historyMaxTurns)
	}
	// 应保留最新的 historyMaxTurns 轮
	last := history[len(history)-1]
	expectedQ := fmt.Sprintf("Q%d", historyMaxTurns+1)
	if last.Question != expectedQ {
		t.Errorf("last history question = %q, want %q", last.Question, expectedQ)
	}
}

// TestAnswerGenerator_HistoryWithFalse 验证 withHistory=false 时不返回历史。
func TestAnswerGenerator_HistoryWithFalse(t *testing.T) {
	g := NewAnswerGenerator("", "", func(_ string, _ ...any) {})
	g.appendHistory("sess2", "Q1", "A1")

	if h := g.getHistory("sess2", false); len(h) != 0 {
		t.Errorf("expected empty history when withHistory=false, got %v", h)
	}
}

// TestAnswerGenerator_HistoryIsolation 验证不同 session 历史互不影响。
func TestAnswerGenerator_HistoryIsolation(t *testing.T) {
	g := NewAnswerGenerator("", "", func(_ string, _ ...any) {})
	g.appendHistory("sess-a", "Qa", "Aa")
	g.appendHistory("sess-b", "Qb", "Ab")

	hA := g.getHistory("sess-a", true)
	hB := g.getHistory("sess-b", true)
	if len(hA) != 1 || hA[0].Question != "Qa" {
		t.Errorf("session a history incorrect: %v", hA)
	}
	if len(hB) != 1 || hB[0].Question != "Qb" {
		t.Errorf("session b history incorrect: %v", hB)
	}
}

// TestAnswerGenerator_StreamGenerate 使用 httptest.Server 验证 SSE 流式解析：
// token 逐个推送 EventAnswerToken，流结束后 Generate 推送 EventAnswerDone。
func TestAnswerGenerator_StreamGenerate(t *testing.T) {
	// 模拟 DeepSeek SSE 响应
	sseBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hi"}}]}`,
		`data: {"choices":[{"delta":{"content":" there"}}]}`,
		`data: [DONE]`,
		"",
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	var tokens []string
	var doneFired bool
	emitter := EventEmitter(func(name string, data ...any) {
		switch name {
		case EventAnswerToken:
			if len(data) > 0 {
				tokens = append(tokens, data[0].(string))
			}
		case EventAnswerDone:
			doneFired = true
		}
	})

	g := NewAnswerGenerator("test-key", "test-model", emitter)
	// 替换 deepSeekChatURL 为 mock 服务器地址，通过临时覆写 client transport 实现
	g.client = &http.Client{
		Transport: rewriteHostTransport{target: srv.URL},
	}

	g.Generate(context.Background(), GenerateConfig{
		SessionID: "test",
		Question:  "hello?",
	})

	if !doneFired {
		t.Error("EventAnswerDone was not emitted")
	}
	joined := strings.Join(tokens, "")
	if joined != "Hi there" {
		t.Errorf("tokens = %q, want %q", joined, "Hi there")
	}
}

// rewriteHostTransport 将所有请求重定向到 target（httptest 服务器地址），用于测试。
type rewriteHostTransport struct{ target string }

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = strings.TrimPrefix(t.target, "http://")
	return http.DefaultTransport.RoundTrip(req2)
}
