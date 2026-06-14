package llm

import "strings"

const DefaultOpenAICompatibleBaseURL = "https://api.deepseek.com/v1"

// Config describes an OpenAI-compatible chat completion provider.
type Config struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
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
