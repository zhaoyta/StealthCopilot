// Package config 定义各 Provider 的配置结构体和类型常量。
// 所有枚举状态通过具名常量表示，禁止硬编码字符串。
package config

// TranslationProviderType 表示翻译服务提供商类型。
type TranslationProviderType string

const (
	// TranslationProviderXunfeiSimult 使用讯飞同声传译获取原文和译文。
	TranslationProviderXunfeiSimult TranslationProviderType = "xunfei_simult"
	// TranslationProviderXunfei is kept as a legacy alias for the current iFlytek simultaneous interpretation provider.
	TranslationProviderXunfei TranslationProviderType = "xunfei"
	TranslationProviderNull   TranslationProviderType = "null"
)

type LLMProviderType string

const (
	LLMProviderOpenAICompatible LLMProviderType = "openai_compatible"
	LLMProviderDeepSeek         LLMProviderType = "deepseek"
)

// TTSProviderType 表示 TTS 服务提供商类型。
type TTSProviderType string

const (
	// TTSProviderXunfeiVoiceClone 使用讯飞一句话复刻流式 TTS。
	TTSProviderXunfeiVoiceClone TTSProviderType = "xunfei_voiceclone"
	TTSProviderSystem           TTSProviderType = "system"
	TTSProviderNull             TTSProviderType = "null"
)

// LipSyncProviderType 表示口型同步服务提供商类型。
type LipSyncProviderType string

const (
	// LipSyncProviderSimli 使用 Simli AI SaaS API。
	LipSyncProviderSimli LipSyncProviderType = "simli"
	// LipSyncProviderStealth 使用后续自营 StealthCloud 服务（Phase 3）。
	LipSyncProviderStealth LipSyncProviderType = "stealth_cloud"
	LipSyncProviderNull    LipSyncProviderType = "null"
)

type EmbeddingProviderType string

const (
	EmbeddingProviderPythonBridge EmbeddingProviderType = "python_bridge"
	EmbeddingProviderNull         EmbeddingProviderType = "null"
)

// ProviderConfig 持有所有 Provider 类型选择配置。
// 运行时根据此配置实例化对应的实现。
type ProviderConfig struct {
	Translation TranslationProviderType // ASR/翻译服务类型
	LLM         LLMProviderType         // LLM / chat completions 服务类型
	TTS         TTSProviderType         // TTS 服务类型
	LipSync     LipSyncProviderType     // 口型同步服务类型
	Embedding   EmbeddingProviderType   // 简历 embedding 服务类型
}

// DefaultProviderConfig 返回生产环境默认配置。
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		Translation: TranslationProviderXunfeiSimult,
		LLM:         LLMProviderDeepSeek,
		TTS:         TTSProviderSystem,
		LipSync:     LipSyncProviderSimli,
		Embedding:   EmbeddingProviderPythonBridge,
	}
}
