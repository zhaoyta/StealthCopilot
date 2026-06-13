// Package tts 定义文字转语音（TTS）Provider 接口。
// ElevenLabs 流式 TTS 实现在同包的 elevenlabs.go 中提供。
package tts

import "context"

// Provider 是 TTS 服务的统一抽象接口。
// 实现必须支持流式输出，首个音频 chunk 应尽快返回以降低延迟。
type Provider interface {
	// Synthesize 将文本转换为音频流（PCM 或 MP3 chunk）。
	// 返回 channel，每次接收到 chunk 即可写入虚拟麦克风，不需等待全部完成。
	// text 为目标语言文本（如英文），通过 cancel ctx 中止合成。
	Synthesize(ctx context.Context, text string) (<-chan []byte, error)

	// VoiceID 返回当前配置的声音克隆 ID（用于日志和校验）。
	VoiceID() string

	// Close 释放资源。
	Close() error
}
