package llm

import "strings"

const DefaultOpenAICompatibleBaseURL = "https://api.deepseek.com/v1"
const DefaultHistoryMaxTurns = 5

// Config describes an OpenAI-compatible chat completion provider.
type Config struct {
	Provider        string
	APIKey          string
	Model           string
	BaseURL         string
	HistoryMaxTurns int
}

func (c Config) chatCompletionsURL() string {
	return c.ChatCompletionsURL()
}

func (c Config) ChatCompletionsURL() string {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultOpenAICompatibleBaseURL
	}
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	return baseURL + "/chat/completions"
}

func (c Config) EffectiveHistoryMaxTurns() int {
	if c.HistoryMaxTurns <= 0 {
		return DefaultHistoryMaxTurns
	}
	return c.HistoryMaxTurns
}
