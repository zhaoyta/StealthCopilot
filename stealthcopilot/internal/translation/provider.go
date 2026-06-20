// Package translation 定义实时语音同传 Provider 接口。
package translation

import (
	"context"
	"encoding/binary"
	"errors"
)

// DualResult 包含一次翻译结果的两路输出：
//   - SrcText：面试官原始语言文本（用于 RAG 检索）
//   - DstText：用户目标语言翻译文本（用于幽灵提词窗字幕）
type DualResult struct {
	SrcText  string // 原文（面试官语言，如英文）
	DstText  string // 译文（用户语言，如中文）
	IsFinal  bool   // 是否为句子最终结果
	AudioPCM []byte // 可选：同传服务返回的译文音频，16k/16bit/mono PCM
}

// Provider 是实时 ASR/翻译服务的统一抽象接口。
// 单次调用同时返回 src 和 dst，两路并行不串行。
type Provider interface {
	// Translate 开始实时 ASR，从 audioStream 读取 PCM 数据。
	// 返回 DualResult channel，每条记录包含原文，译文可由包装器补齐。
	// 通过 cancel ctx 停止翻译并关闭 channel。
	Translate(ctx context.Context, audioStream <-chan []byte) (<-chan DualResult, error)

	// Close 释放 WebSocket 连接等资源。
	Close() error
}

// SpeakProvider translates a completed speech segment into source and target text.
type SpeakProvider interface {
	Translate(ctx context.Context, pcmData []byte) (DualResult, error)
}

// ErrNoSpeechRecognized 表示语音服务正常返回但没有可用文本。
var ErrNoSpeechRecognized = errors.New("translation: no speech text recognized")

// ErrNoTranslationReturned 表示 ASR 有文本，但跨语言翻译没有返回目标文本。
var ErrNoTranslationReturned = errors.New("translation: no translated text returned")

type NullProvider struct{}

func (NullProvider) Translate(_ context.Context, _ <-chan []byte) (<-chan DualResult, error) {
	ch := make(chan DualResult)
	close(ch)
	return ch, nil
}

func (NullProvider) Close() error { return nil }

type NullSpeakProvider struct{}

func (NullSpeakProvider) Translate(context.Context, []byte) (DualResult, error) {
	return DualResult{}, nil
}

func pcmDurationMs(pcm []byte) int {
	const sampleRate = 16000
	return len(pcm) / 2 * 1000 / sampleRate
}

func pcmPeak(pcm []byte) int {
	peak := 0
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int(int16(binary.LittleEndian.Uint16(pcm[i:])))
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	return peak
}

func previewResponse(data []byte) string {
	const max = 240
	text := string(data)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
