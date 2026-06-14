// Package translation 定义实时语音翻译 Provider 接口。
// 讯飞实时语音翻译 API 单次 WebSocket 调用同时返回原文和译文，
// 通过 DualResult 的两个 channel 分发给听力链的不同下游。
package translation

import "context"

// DualResult 包含一次翻译结果的两路输出：
//   - SrcText：面试官原始语言文本（用于 RAG 检索）
//   - DstText：用户目标语言翻译文本（用于幽灵提词窗字幕）
type DualResult struct {
	SrcText string // 原文（面试官语言，如英文）
	DstText string // 译文（用户语言，如中文）
	IsFinal bool   // 是否为句子最终结果
}

// Provider 是实时语音翻译服务的统一抽象接口。
// 单次调用同时返回 src_text 和 dst_text，两路并行不串行。
type Provider interface {
	// Translate 开始实时翻译，从 audioStream 读取 PCM 数据。
	// 返回 DualResult channel，每条记录同时包含原文和译文。
	// 通过 cancel ctx 停止翻译并关闭 channel。
	Translate(ctx context.Context, audioStream <-chan []byte) (<-chan DualResult, error)

	// Close 释放 WebSocket 连接等资源。
	Close() error
}

// SpeakProvider translates a completed speech segment into target-language text.
type SpeakProvider interface {
	Translate(ctx context.Context, pcmData []byte) (string, error)
}

type NullProvider struct{}

func (NullProvider) Translate(_ context.Context, _ <-chan []byte) (<-chan DualResult, error) {
	ch := make(chan DualResult)
	close(ch)
	return ch, nil
}

func (NullProvider) Close() error { return nil }

type NullSpeakProvider struct{}

func (NullSpeakProvider) Translate(context.Context, []byte) (string, error) { return "", nil }
