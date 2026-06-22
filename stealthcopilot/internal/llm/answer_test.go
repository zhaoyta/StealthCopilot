// Package llm 单测：覆盖事件常量、SSE token 解析、Prompt 构建、对话历史管理和流式生成逻辑。
package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhaoyta/stealthcopilot/internal/session"
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
	prompt := buildSystemPrompt(chunks, "Tell me about yourself", "{resume_context}\n{question}", nil, "en")
	if !strings.Contains(prompt, "chunk1") || !strings.Contains(prompt, "chunk2") {
		t.Error("prompt should contain resume chunks")
	}
	if !strings.Contains(prompt, "Tell me about yourself") {
		t.Error("prompt should contain the question")
	}
}

// TestBuildSystemPrompt_DefaultTemplate 验证未传模板时使用内置默认模板。
func TestBuildSystemPrompt_DefaultTemplate(t *testing.T) {
	prompt := buildSystemPrompt(nil, "What is your strength?", "", nil, "zh")
	if !strings.Contains(prompt, "What is your strength?") {
		t.Error("default prompt should contain the question")
	}
	if !strings.Contains(prompt, "简历内容") {
		t.Error("default prompt should contain '简历内容' header")
	}
	if !strings.Contains(prompt, "请用中文回答") {
		t.Error("default prompt should require Chinese target language")
	}
}

// TestBuildSystemPrompt_WithHistory 验证对话历史被追加到 prompt 末尾。
func TestBuildSystemPrompt_WithHistory(t *testing.T) {
	history := []QAPair{
		{Question: "Q1", Answer: "A1"},
	}
	prompt := buildSystemPrompt(nil, "follow-up", "{resume_context}\n{question}", history, "en")
	if !strings.Contains(prompt, "Q: Q1") || !strings.Contains(prompt, "A: A1") {
		t.Errorf("prompt should contain dialog history, got: %s", prompt)
	}
}

// TestAnswerGenerator_HistoryFromStore 验证历史从 session store 读取并遵守配置轮数。
func TestAnswerGenerator_HistoryFromStore(t *testing.T) {
	store := &fakeSessionStore{
		turns: map[string][]session.Turn{
			"sess1": {
				{Question: "Q1", Answer: "A1"},
				{Question: "Q2", Answer: "A2"},
				{Question: "Q3", Answer: "A3"},
			},
		},
	}
	g := NewAnswerGeneratorWithSessionStore(Config{HistoryMaxTurns: 2}, func(_ string, _ ...any) {}, store)

	history := g.getHistory("sess1")
	if len(history) != 2 {
		t.Fatalf("history len = %d, want 2", len(history))
	}
	if history[0].Question != "Q2" || history[1].Question != "Q3" {
		t.Fatalf("history order mismatch: %+v", history)
	}
	if store.lastLimit != 2 {
		t.Fatalf("store limit = %d, want 2", store.lastLimit)
	}
}

func TestAnswerGenerator_DefaultHistoryMaxTurns(t *testing.T) {
	g := NewAnswerGeneratorWithConfig(Config{}, func(_ string, _ ...any) {})
	if g.historyMaxTurns != DefaultHistoryMaxTurns {
		t.Fatalf("historyMaxTurns = %d, want %d", g.historyMaxTurns, DefaultHistoryMaxTurns)
	}
}

// TestAnswerGenerator_HistoryIsolation 验证不同 session 历史互不影响。
func TestAnswerGenerator_HistoryIsolation(t *testing.T) {
	store := &fakeSessionStore{
		turns: map[string][]session.Turn{
			"sess-a": {{Question: "Qa", Answer: "Aa"}},
			"sess-b": {{Question: "Qb", Answer: "Ab"}},
		},
	}
	g := NewAnswerGeneratorWithSessionStore(Config{}, func(_ string, _ ...any) {}, store)

	hA := g.getHistory("sess-a")
	hB := g.getHistory("sess-b")
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
		SessionID:       "test",
		Question:        "hello?",
		DisplayQuestion: "你好？",
	})

	if !doneFired {
		t.Error("EventAnswerDone was not emitted")
	}
	joined := strings.Join(tokens, "")
	if joined != "Hi there" {
		t.Errorf("tokens = %q, want %q", joined, "Hi there")
	}
}

func TestAnswerGenerator_GenerateAppendsTurn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Answer\"}}]}\n\ndata: [DONE]\n")
	}))
	defer srv.Close()

	store := &fakeSessionStore{turns: map[string][]session.Turn{}}
	g := NewAnswerGeneratorWithSessionStore(Config{APIKey: "k", Model: "m"}, func(_ string, _ ...any) {}, store)
	g.client = &http.Client{Transport: rewriteHostTransport{target: srv.URL}}

	g.Generate(context.Background(), GenerateConfig{
		SessionID:       "sess",
		Question:        "Q",
		DisplayQuestion: "显示Q",
	})

	if len(store.appended) != 1 {
		t.Fatalf("appended len = %d, want 1", len(store.appended))
	}
	got := store.appended[0]
	if got.SessionID != "sess" || got.Question != "Q" || got.DisplayQuestion != "显示Q" || got.Answer != "Answer" {
		t.Fatalf("appended turn mismatch: %+v", got)
	}
}

func TestAnswerGenerator_GenerateEmptyAnswerDoesNotAppend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n")
	}))
	defer srv.Close()

	store := &fakeSessionStore{turns: map[string][]session.Turn{}}
	g := NewAnswerGeneratorWithSessionStore(Config{APIKey: "k", Model: "m"}, func(_ string, _ ...any) {}, store)
	g.client = &http.Client{Transport: rewriteHostTransport{target: srv.URL}}

	g.Generate(context.Background(), GenerateConfig{SessionID: "sess", Question: "Q"})
	if len(store.appended) != 0 {
		t.Fatalf("appended len = %d, want 0", len(store.appended))
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

type fakeSessionStore struct {
	session.Store
	turns     map[string][]session.Turn
	lastLimit int
	appended  []session.Turn
}

func (f *fakeSessionStore) GetRecentTurns(sessionID string, limit int) ([]session.Turn, error) {
	f.lastLimit = limit
	turns := append([]session.Turn(nil), f.turns[sessionID]...)
	if limit > 0 && len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	return turns, nil
}

func (f *fakeSessionStore) AppendTurn(sessionID, question, displayQuestion, answer string) error {
	f.appended = append(f.appended, session.Turn{
		SessionID:       sessionID,
		Question:        question,
		DisplayQuestion: displayQuestion,
		Answer:          answer,
	})
	return nil
}
