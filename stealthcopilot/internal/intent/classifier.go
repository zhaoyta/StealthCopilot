// Package intent 实现讯飞翻译 src_text 的三分类意图识别（question/followup/statement）。
// 通过 DeepSeek API 对完整句子（is_end=true）进行分类；
// 分类失败时降级返回 IntentStatement，避免中断字幕显示流程。
package intent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// IntentType 表示意图识别的三分类结果。
type IntentType string

const (
	// IntentQuestion 面试官提出新问题，触发 RAG 检索 + 回答生成。
	IntentQuestion IntentType = "question"
	// IntentFollowup 对上一回答的追问，携带对话历史触发 RAG + 回答生成。
	IntentFollowup IntentType = "followup"
	// IntentStatement 陈述或闲聊，不触发回答生成，仅显示字幕。
	IntentStatement IntentType = "statement"
)

const (
	deepSeekChatURL = "https://api.deepseek.com/v1/chat/completions"
	classifyTimeout = 5 * time.Second

	// classifySystemPrompt 是 Go 后端硬编码的分类指令，不允许前端修改。
	// 约束 DeepSeek 只输出 JSON，防止自由发挥导致解析失败。
	classifySystemPrompt = `你是意图分类助手。判断下面的英文句子属于哪种意图：
question（面试官提出独立新问题）、followup（基于上一个回答的追问）、statement（陈述背景信息或闲聊）。
只输出 JSON，格式：{"intent":"question"}，不要其他文字。`
)

// Classifier 调用 DeepSeek API 对完整句子（is_end=true）进行三分类意图识别。
// 分类结果用于决定是否触发 RAG + 回答生成管道。
type Classifier struct {
	apiKey string
	model  string
	client *http.Client
}

// NewClassifier 创建意图分类器。
// model 为 DeepSeek 模型名（如 "deepseek-chat"）。
func NewClassifier(apiKey, model string) *Classifier {
	return &Classifier{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: classifyTimeout},
	}
}

// Classify 对 srcText 进行意图分类（异步调用，仅对 is_end=true 的完整句子触发）。
// 分类失败时返回 IntentStatement（降级：仅显示字幕，不触发 RAG），不中断流程。
func (c *Classifier) Classify(ctx context.Context, srcText string) IntentType {
	result, err := c.callDeepSeek(ctx, srcText)
	if err != nil {
		return IntentStatement
	}
	return result
}

// callDeepSeek 发起单次 DeepSeek 分类请求并解析 JSON 响应。
func (c *Classifier) callDeepSeek(ctx context.Context, srcText string) (IntentType, error) {
	reqBody := classifyRequest{
		Model: c.model,
		Messages: []classifyMessage{
			{Role: "system", Content: classifySystemPrompt},
			{Role: "user", Content: srcText},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, deepSeekChatURL, bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return IntentStatement, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return IntentStatement, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return IntentStatement, fmt.Errorf("deepseek classify: status %d", resp.StatusCode)
	}

	var dsResp classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&dsResp); err != nil || len(dsResp.Choices) == 0 {
		return IntentStatement, fmt.Errorf("deepseek classify: parse response")
	}

	content := extractJSON(strings.TrimSpace(dsResp.Choices[0].Message.Content))
	var result struct {
		Intent string `json:"intent"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return IntentStatement, fmt.Errorf("deepseek classify: parse intent JSON")
	}

	switch IntentType(result.Intent) {
	case IntentQuestion, IntentFollowup, IntentStatement:
		return IntentType(result.Intent), nil
	default:
		return IntentStatement, nil
	}
}

// extractJSON 从可能含有 markdown 代码块标记的字符串中提取 JSON 内容。
// DeepSeek 有时会在 JSON 外包裹 ```json ... ``` 标记。
func extractJSON(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	var inner []string
	for i, l := range lines {
		if i == 0 {
			continue
		}
		if strings.HasPrefix(l, "```") {
			break
		}
		inner = append(inner, l)
	}
	return strings.Join(inner, "\n")
}

// --- DeepSeek API 数据结构 ---

type classifyRequest struct {
	Model    string            `json:"model"`
	Messages []classifyMessage `json:"messages"`
}

type classifyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type classifyResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
