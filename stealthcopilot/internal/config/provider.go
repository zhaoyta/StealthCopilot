// Package config 定义各 Provider 的配置结构体和类型常量。
// 所有枚举状态通过具名常量表示，禁止硬编码字符串。
package config

// TranslationProviderType 表示翻译服务提供商类型。
type TranslationProviderType string

const (
	// TranslationProviderXunfeiSimult 复用讯飞语音/翻译凭证；具体链路按业务选择 RTASR、文本翻译或同声传译。
	TranslationProviderXunfeiSimult TranslationProviderType = "xunfei_simult"
	TranslationProviderNull         TranslationProviderType = "null"
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

type EmbeddingProviderType string

const (
	EmbeddingProviderPythonBridge EmbeddingProviderType = "python_bridge"
	EmbeddingProviderNull         EmbeddingProviderType = "null"
)

// DigitalHumanProviderType 表示数字人驱动类型。
type DigitalHumanProviderType string

const (
	// DigitalHumanProviderSimli Simli AI 数字人（推荐，视频同步，TTS 音频直接输出）。
	DigitalHumanProviderSimli DigitalHumanProviderType = "simli"
	// DigitalHumanProviderZego 即构 ZEGO 数字人（企业级，音视频均由云端生成）。
	DigitalHumanProviderZego DigitalHumanProviderType = "zego"
)

// ProviderConfig 持有所有 Provider 类型选择配置。
// 运行时根据此配置实例化对应的实现。
type ProviderConfig struct {
	HearingASR    TranslationProviderType // 听力链 ASR 服务类型
	HearingTrans  TranslationProviderType // 听力链 Trans 服务类型
	HearingTTS    TTSProviderType         // 听力链 TTS 服务类型
	SpeakingASR   TranslationProviderType // 说话链 ASR 服务类型
	SpeakingTrans TranslationProviderType // 说话链 Trans 服务类型
	SpeakingTTS   TTSProviderType         // 说话链 TTS 服务类型
	LLM           LLMProviderType         // LLM / chat completions 服务类型
	Embedding     EmbeddingProviderType   // 简历 embedding 服务类型
}

// DefaultProviderConfig 返回生产环境默认配置。
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		HearingASR:    TranslationProviderXunfeiSimult,
		HearingTrans:  TranslationProviderXunfeiSimult,
		HearingTTS:    TTSProviderSystem,
		SpeakingASR:   TranslationProviderXunfeiSimult,
		SpeakingTrans: TranslationProviderXunfeiSimult,
		SpeakingTTS:   TTSProviderSystem,
		LLM:           LLMProviderDeepSeek,
		Embedding:     EmbeddingProviderPythonBridge,
	}
}
