// Package config 定义各 Provider 的配置结构体和类型常量。
// 所有枚举状态通过具名常量表示，禁止硬编码字符串。
package config

// TranslationProviderType 表示翻译服务提供商类型。
type TranslationProviderType string

const (
	// TranslationProviderXunfei 使用讯飞实时语音翻译 API。
	TranslationProviderXunfei TranslationProviderType = "xunfei"
)

// TTSProviderType 表示 TTS 服务提供商类型。
type TTSProviderType string

const (
	// TTSProviderElevenLabs 使用 ElevenLabs 流式 TTS。
	TTSProviderElevenLabs TTSProviderType = "elevenlabs"
)

// LipSyncProviderType 表示口型同步服务提供商类型。
type LipSyncProviderType string

const (
	// LipSyncProviderSimli 使用 Simli AI SaaS API。
	LipSyncProviderSimli LipSyncProviderType = "simli"
	// LipSyncProviderStealth 使用后续自营 StealthCloud 服务（Phase 3）。
	LipSyncProviderStealth LipSyncProviderType = "stealth_cloud"
)

// ProviderConfig 持有所有 Provider 类型选择配置。
// 运行时根据此配置实例化对应的实现。
type ProviderConfig struct {
	Translation TranslationProviderType // 翻译服务类型
	TTS         TTSProviderType         // TTS 服务类型
	LipSync     LipSyncProviderType     // 口型同步服务类型
}

// DefaultProviderConfig 返回生产环境默认配置。
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		Translation: TranslationProviderXunfei,
		TTS:         TTSProviderElevenLabs,
		LipSync:     LipSyncProviderSimli,
	}
}
