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
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/session"
)

const (
	streamTimeout = 60 * time.Second
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
// 多轮对话历史通过 session.Store 按 session ID 持久化存取。
type AnswerGenerator struct {
	apiKey          string
	model           string
	baseURL         string
	historyMaxTurns int
	emitter         EventEmitter
	client          *http.Client
	sessionStore    session.Store
}

// NewAnswerGenerator 创建回答生成器。
// emitter 传入 Wails EventsEmit 的包装函数；model 为 DeepSeek 模型名。
func NewAnswerGenerator(apiKey, model string, emitter EventEmitter) *AnswerGenerator {
	return NewAnswerGeneratorWithConfig(Config{APIKey: apiKey, Model: model}, emitter)
}

func NewAnswerGeneratorWithConfig(cfg Config, emitter EventEmitter) *AnswerGenerator {
	return &AnswerGenerator{
		apiKey:          cfg.APIKey,
		model:           cfg.Model,
		baseURL:         cfg.BaseURL,
		historyMaxTurns: cfg.EffectiveHistoryMaxTurns(),
		emitter:         emitter,
		client:          &http.Client{Timeout: streamTimeout},
	}
}

func NewAnswerGeneratorWithSessionStore(cfg Config, emitter EventEmitter, store session.Store) *AnswerGenerator {
	g := NewAnswerGeneratorWithConfig(cfg, emitter)
	g.sessionStore = store
	return g
}

// GenerateConfig 配置单次回答生成请求的参数。
type GenerateConfig struct {
	// SessionID 对话会话 ID，用于维护多轮历史（重启后清空）。
	SessionID string
	// Question 面试官问题（英文原文，来自 src_text）。
	Question string
	// DisplayQuestion 历史 Tab 展示文本；为空时回退 Question。
	DisplayQuestion string
	// TargetLanguage is the user-visible answer language, e.g. zh or en.
	TargetLanguage string
	// ResumeChunks 是 RAG 检索返回的相关简历片段列表。
	ResumeChunks []string
	// PromptTpl 是用户自定义的 RAG 回答 Prompt 模板（含 {resume_context}/{question} 占位符）。
	PromptTpl string
	// Deprecated: history is now loaded whenever the current session has saved turns.
	WithHistory bool
}

// Generate 异步启动 DeepSeek SSE 流式回答，通过 emitter 逐 token 推送 EventAnswerToken，
// 完成后推送 EventAnswerDone，并将本轮 Q&A 追加到 session 历史。
// 调用方应在独立 goroutine 中调用，Generate 内部阻塞直到流结束或 ctx 取消。
func (g *AnswerGenerator) Generate(ctx context.Context, cfg GenerateConfig) {
	history := g.getHistory(cfg.SessionID)
	systemPrompt := buildSystemPrompt(cfg.ResumeChunks, cfg.Question, cfg.PromptTpl, history, cfg.TargetLanguage)
	diag.Infof("answer generation begin session=%s question_chars=%d chunks=%d history_turns=%d", cfg.SessionID, len(cfg.Question), len(cfg.ResumeChunks), len(history))

	messages := []llmMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: cfg.Question},
	}

	answer, err := g.streamGenerate(ctx, messages)
	if err != nil {
		diag.Warnf("answer generation failed session=%s err=%v", cfg.SessionID, err)
	}
	if answer != "" {
		g.appendHistory(cfg.SessionID, cfg.Question, cfg.DisplayQuestion, answer)
	}
	diag.Infof("answer generation done session=%s answer_chars=%d", cfg.SessionID, len(answer))
	g.emitter(EventAnswerDone)
}

// RecentHistory returns the saved recent turns for context selection before
// answer generation, such as resolving "this project" follow-up questions.
func (g *AnswerGenerator) RecentHistory(sessionID string) []QAPair {
	return g.getHistory(sessionID)
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

// buildSystemPrompt 将 RAG 检索结果、对话历史和当前问题整合为 DeepSeek System Prompt。
// {resume_context}、{history}、{question} 是模板占位符。
// 若模板不含 {history}，历史块追加在末尾（向下兼容旧自定义模板）。
func buildSystemPrompt(chunks []string, question, promptTpl string, history []QAPair, targetLanguage string) string {
	if promptTpl == "" {
		promptTpl = defaultRAGPrompt
	}
	resumeCtx := strings.Join(chunks, "\n\n")

	historyBlock := formatHistory(history)
	languageInstruction := answerLanguageInstruction(targetLanguage)

	prompt := strings.ReplaceAll(promptTpl, "{resume_context}", resumeCtx)
	prompt = strings.ReplaceAll(prompt, "{question}", question)
	prompt = strings.ReplaceAll(prompt, "{target_language}", languageInstruction)

	if strings.Contains(prompt, "{history}") {
		prompt = strings.ReplaceAll(prompt, "{history}", historyBlock)
	} else if historyBlock != "" {
		prompt += "\n\n" + historyBlock
	}
	if !strings.Contains(promptTpl, "{target_language}") {
		prompt += "\n\n" + languageInstruction
	}
	return prompt
}

func answerLanguageInstruction(targetLanguage string) string {
	switch strings.ToLower(strings.TrimSpace(targetLanguage)) {
	case "zh", "zh-cn", "zh_cn", "cn", "chinese":
		return "输出语言：请用中文回答。"
	case "en", "en-us", "en_us", "english":
		return "输出语言：请用英文回答。"
	default:
		if strings.TrimSpace(targetLanguage) == "" {
			return "输出语言：请使用用户的目标语言回答。"
		}
		return "输出语言：请使用 " + strings.TrimSpace(targetLanguage) + " 回答。"
	}
}

// formatHistory 将 QAPair 列表格式化为 Prompt 中可直接嵌入的历史区块文本。
func formatHistory(history []QAPair) string {
	if len(history) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("本场面试对话历史（按时间顺序）：\n")
	for i, qa := range history {
		fmt.Fprintf(&buf, "第%d轮\nQ: %s\nA: %s\n", i+1, qa.Question, qa.Answer)
	}
	return buf.String()
}

// defaultRAGPrompt 是当用户未自定义 Prompt 时使用的内置模板。
// 占位符说明：{resume_context} 简历片段、{history} 本场历史对话、{question} 当前问题。
const defaultRAGPrompt = `你是一位专业面试助手，帮助候选人回答面试官的问题。

简历内容：
{resume_context}

{history}
面试官当前问题：{question}

回答要求：
- {target_language}
- 用简洁专业的表达作答
- 若有对话历史，回答须与前几轮保持一致，不要自相矛盾
- 首次回答新问题：不超过 3 句话
- 追问或补充细节：1~2 句即可，无需重复已说过的内容

回答建议：`

// getHistory returns the persisted recent turns for a session.
func (g *AnswerGenerator) getHistory(sessionID string) []QAPair {
	if sessionID == "" || g.sessionStore == nil {
		return nil
	}
	turns, err := g.sessionStore.GetRecentTurns(sessionID, g.historyMaxTurns)
	if err != nil {
		return nil
	}
	result := make([]QAPair, 0, len(turns))
	for _, turn := range turns {
		result = append(result, QAPair{Question: turn.Question, Answer: turn.Answer})
	}
	return result
}

// appendHistory persists one generated answer. Store errors are intentionally non-fatal
// because the user-facing answer has already been streamed.
func (g *AnswerGenerator) appendHistory(sessionID, question, displayQuestion, answer string) {
	if sessionID == "" || g.sessionStore == nil {
		return
	}
	_ = g.sessionStore.AppendTurn(sessionID, question, displayQuestion, answer)
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
