// Package llm 实现 DeepSeek SSE 流式回答生成，将面试回答 token 逐个推送到提词窗前端。
// 使用 EventEmitter 抽象 Wails runtime.EventsEmit，便于测试时替换为 mock。
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	streamTimeout = 60 * time.Second
	// historyMaxTurns 是内存中保留的最大对话轮数（超出后滚动删除最旧记录）。
	historyMaxTurns = 3
)

const (
	// EventAnswerToken 是逐 token 推送的 Wails 事件名，前端监听后逐字追加显示。
	EventAnswerToken = "answer:token"
	// EventAnswerDone 是回答流结束的 Wails 事件名，前端收到后隐藏打字光标。
	EventAnswerDone = "answer:done"
)

// EventEmitter 是 Wails runtime.EventsEmit 的函数类型抽象，便于注入测试 mock。
type EventEmitter func(eventName string, data ...any)

// QAPair 是一轮对话的问答对，用于多轮对话历史追踪。
type QAPair struct {
	Question string
	Answer   string
}

// AnswerGenerator 调用 DeepSeek SSE 流式接口生成面试回答。
// 每收到 token 立即通过 EventEmitter 推送前端；回答结束发送 EventAnswerDone。
// 多轮对话历史（最近 historyMaxTurns 轮 Q&A）按 session ID 存储在内存中。
type AnswerGenerator struct {
	apiKey  string
	model   string
	baseURL string
	emitter EventEmitter
	client  *http.Client

	historyMu sync.Mutex
	history   map[string][]QAPair // key: session ID
}

// NewAnswerGenerator 创建回答生成器。
// emitter 传入 Wails EventsEmit 的包装函数；model 为 DeepSeek 模型名。
func NewAnswerGenerator(apiKey, model string, emitter EventEmitter) *AnswerGenerator {
	return NewAnswerGeneratorWithConfig(Config{APIKey: apiKey, Model: model}, emitter)
}

func NewAnswerGeneratorWithConfig(cfg Config, emitter EventEmitter) *AnswerGenerator {
	return &AnswerGenerator{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: cfg.BaseURL,
		emitter: emitter,
		client:  &http.Client{Timeout: streamTimeout},
		history: make(map[string][]QAPair),
	}
}

// GenerateConfig 配置单次回答生成请求的参数。
type GenerateConfig struct {
	// SessionID 对话会话 ID，用于维护多轮历史（重启后清空）。
	SessionID string
	// Question 面试官问题（英文原文，来自 src_text）。
	Question string
	// ResumeChunks 是 RAG 检索返回的相关简历片段列表。
	ResumeChunks []string
	// PromptTpl 是用户自定义的 RAG 回答 Prompt 模板（含 {resume_context}/{question} 占位符）。
	PromptTpl string
	// WithHistory 为 true 时（followup 意图）附带最近 historyMaxTurns 轮对话历史。
	WithHistory bool
}

// Generate 异步启动 DeepSeek SSE 流式回答，通过 emitter 逐 token 推送 EventAnswerToken，
// 完成后推送 EventAnswerDone，并将本轮 Q&A 追加到 session 历史。
// 调用方应在独立 goroutine 中调用，Generate 内部阻塞直到流结束或 ctx 取消。
func (g *AnswerGenerator) Generate(ctx context.Context, cfg GenerateConfig) {
	history := g.getHistory(cfg.SessionID, cfg.WithHistory)
	systemPrompt := buildSystemPrompt(cfg.ResumeChunks, cfg.Question, cfg.PromptTpl, history)

	messages := []llmMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: cfg.Question},
	}

	answer, _ := g.streamGenerate(ctx, messages)
	if answer != "" {
		g.appendHistory(cfg.SessionID, cfg.Question, answer)
	}
	g.emitter(EventAnswerDone)
}

// streamGenerate 向 DeepSeek 发起 SSE 流式请求，逐 chunk 解析并推送 EventAnswerToken。
// 返回完整回答文本（用于追加历史）。
func (g *AnswerGenerator) streamGenerate(ctx context.Context, messages []llmMessage) (string, error) {
	reqBody := llmStreamRequest{
		Model:    g.model,
		Messages: messages,
		Stream:   true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		Config{BaseURL: g.baseURL}.chatCompletionsURL(),
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepseek stream: status %d", resp.StatusCode)
	}

	var fullAnswer strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		token := extractSSEToken(payload)
		if token == "" {
			continue
		}
		fullAnswer.WriteString(token)
		g.emitter(EventAnswerToken, token)
	}
	return fullAnswer.String(), scanner.Err()
}

// extractSSEToken 从 SSE data payload 中提取 DeepSeek token 内容。
func extractSSEToken(payload string) string {
	var chunk sseChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil || len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

// buildSystemPrompt 将 RAG 检索结果、问题和对话历史整合为 DeepSeek System Prompt。
// {resume_context} 和 {question} 是模板占位符；对话历史追加在末尾。
func buildSystemPrompt(chunks []string, question, promptTpl string, history []QAPair) string {
	if promptTpl == "" {
		promptTpl = defaultRAGPrompt
	}
	resumeCtx := strings.Join(chunks, "\n\n")
	prompt := strings.ReplaceAll(promptTpl, "{resume_context}", resumeCtx)
	prompt = strings.ReplaceAll(prompt, "{question}", question)

	if len(history) > 0 {
		var buf strings.Builder
		buf.WriteString("\n\n对话历史（最近若干轮）：\n")
		for _, qa := range history {
			fmt.Fprintf(&buf, "Q: %s\nA: %s\n", qa.Question, qa.Answer)
		}
		prompt += buf.String()
	}
	return prompt
}

// defaultRAGPrompt 是当用户未自定义 Prompt 时使用的内置模板，与 config.DefaultRAGPrompt 保持一致。
const defaultRAGPrompt = `你是一位专业面试助手。根据简历内容和面试官问题，生成简洁专业的英文回答建议（不超过3句话）。

简历内容：
{resume_context}

面试官问题：{question}

回答建议：`

// getHistory 返回 session 的最近 historyMaxTurns 轮对话（副本）。
// withHistory=false 时返回 nil（question 类型不需要历史）。
func (g *AnswerGenerator) getHistory(sessionID string, withHistory bool) []QAPair {
	if !withHistory || sessionID == "" {
		return nil
	}
	g.historyMu.Lock()
	defer g.historyMu.Unlock()
	pairs := g.history[sessionID]
	if len(pairs) > historyMaxTurns {
		pairs = pairs[len(pairs)-historyMaxTurns:]
	}
	result := make([]QAPair, len(pairs))
	copy(result, pairs)
	return result
}

// appendHistory 向 session 追加一轮 Q&A，超出 historyMaxTurns 时滚动删除最旧记录。
func (g *AnswerGenerator) appendHistory(sessionID, question, answer string) {
	if sessionID == "" {
		return
	}
	g.historyMu.Lock()
	defer g.historyMu.Unlock()
	g.history[sessionID] = append(g.history[sessionID], QAPair{Question: question, Answer: answer})
	if len(g.history[sessionID]) > historyMaxTurns {
		g.history[sessionID] = g.history[sessionID][len(g.history[sessionID])-historyMaxTurns:]
	}
}

// --- DeepSeek API 数据结构 ---

type llmStreamRequest struct {
	Model    string       `json:"model"`
	Messages []llmMessage `json:"messages"`
	Stream   bool         `json:"stream"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}
