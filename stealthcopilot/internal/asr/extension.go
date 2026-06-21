// Package asr defines speech recognition extensions for streaming and segmented audio.
package asr

import (
	"context"
	"encoding/binary"
	"errors"
)

// Result 包含一次语音识别/同传结果的两路输出：
//   - SrcText：面试官原始语言文本（用于 RAG 检索）
//   - DstText：用户目标语言翻译文本（用于幽灵提词窗字幕）
type Result struct {
	SrcText  string // 原文（面试官语言，如英文）
	DstText  string // 译文（用户语言，如中文）
	IsFinal  bool   // 是否为句子最终结果
	AudioPCM []byte // 可选：同传服务返回的译文音频，16k/16bit/mono PCM
}

// StreamingExtension 是实时流式 ASR 扩展点。
// 单次调用同时返回 src 和 dst，两路并行不串行。
type StreamingExtension interface {
	// Translate 开始实时 ASR，从 audioStream 读取 PCM 数据。
	// 返回 Result channel，每条记录包含原文，译文可由 Trans 扩展补齐。
	// 通过 cancel ctx 停止翻译并关闭 channel。
	Translate(ctx context.Context, audioStream <-chan []byte) (<-chan Result, error)

	// Close 释放 WebSocket 连接等资源。
	Close() error
}

// SegmentExtension translates a completed speech segment into source and target text.
type SegmentExtension interface {
	Translate(ctx context.Context, pcmData []byte) (Result, error)
}

// ErrNoSpeechRecognized 表示语音服务正常返回但没有可用文本。
var ErrNoSpeechRecognized = errors.New("asr: no speech text recognized")

// ErrNoTranslationReturned 表示 ASR 有文本，但跨语言翻译没有返回目标文本。
var ErrNoTranslationReturned = errors.New("asr: no translated text returned")

type NullStreamingExtension struct{}

func (NullStreamingExtension) Translate(_ context.Context, _ <-chan []byte) (<-chan Result, error) {
	ch := make(chan Result)
	close(ch)
	return ch, nil
}

func (NullStreamingExtension) Close() error { return nil }

type NullSegmentExtension struct{}

func (NullSegmentExtension) Translate(context.Context, []byte) (Result, error) {
	return Result{}, nil
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
