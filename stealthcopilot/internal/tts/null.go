package tts

import "context"

// NullExtension 是 TTS 不可用时的空实现，不输出任何音频。
type NullExtension struct{}

// Synthesize 返回立即关闭的空 channel（无音频输出）。
func (n *NullExtension) Synthesize(_ context.Context, _ string) (<-chan []byte, error) {
	ch := make(chan []byte)
	close(ch)
	return ch, nil
}

// VoiceID 返回空字符串（Null 实现无声音 ID）。
func (n *NullExtension) VoiceID() string { return "" }

// Close 无需操作。
func (n *NullExtension) Close() error { return nil }
