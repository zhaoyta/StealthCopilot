package tts

import "context"

// NullTTSProvider 是 TTS 不可用时的空实现，不输出任何音频。
type NullTTSProvider struct{}

// Synthesize 返回立即关闭的空 channel（无音频输出）。
func (n *NullTTSProvider) Synthesize(_ context.Context, _ string) (<-chan []byte, error) {
	ch := make(chan []byte)
	close(ch)
	return ch, nil
}

// VoiceID 返回空字符串（Null 实现无声音 ID）。
func (n *NullTTSProvider) VoiceID() string { return "" }

// Close 无需操作。
func (n *NullTTSProvider) Close() error { return nil }
