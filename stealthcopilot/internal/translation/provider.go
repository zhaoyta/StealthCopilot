// Package translation 定义实时 ASR 与文本翻译 Provider 接口。
// 成本优先路径是 RTASR 只转写，最终文本再进入机器翻译。
package translation

import (
	"context"
	"fmt"
	"strings"
)

// DualResult 包含一次翻译结果的两路输出：
//   - SrcText：面试官原始语言文本（用于 RAG 检索）
//   - DstText：用户目标语言翻译文本（用于幽灵提词窗字幕）
type DualResult struct {
	SrcText string // 原文（面试官语言，如英文）
	DstText string // 译文（用户语言，如中文）
	IsFinal bool   // 是否为句子最终结果
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

// SpeakProvider translates a completed speech segment into target-language text.
type SpeakProvider interface {
	Translate(ctx context.Context, pcmData []byte) (string, error)
}

// TextTranslator translates source text to target text.
type TextTranslator interface {
	TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error)
}

// ASRThenTextProvider wraps an ASR provider and translates only final segments.
// Interim segments keep DstText equal to SrcText to avoid charging text translation for partials.
type ASRThenTextProvider struct {
	asr        Provider
	translator TextTranslator
	sourceLang string
	targetLang string
}

func NewASRThenTextProvider(asr Provider, translator TextTranslator, sourceLang, targetLang string) *ASRThenTextProvider {
	return &ASRThenTextProvider{
		asr:        asr,
		translator: translator,
		sourceLang: sourceLang,
		targetLang: targetLang,
	}
}

func (p *ASRThenTextProvider) Translate(ctx context.Context, audioStream <-chan []byte) (<-chan DualResult, error) {
	if p.asr == nil {
		return nil, fmt.Errorf("translation: missing ASR provider")
	}
	in, err := p.asr.Translate(ctx, audioStream)
	if err != nil {
		return nil, err
	}
	out := make(chan DualResult, 32)
	go func() {
		defer close(out)
		for result := range in {
			result.DstText = result.SrcText
			if result.IsFinal && result.SrcText != "" && p.translator != nil && p.sourceLang != p.targetLang {
				if translated, err := p.translator.TranslateText(ctx, result.SrcText, p.sourceLang, p.targetLang); err == nil && translated != "" {
					result.DstText = translated
				}
			}
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (p *ASRThenTextProvider) Close() error {
	if p.asr != nil {
		return p.asr.Close()
	}
	return nil
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

type NullTextTranslator struct{}

func (NullTextTranslator) TranslateText(_ context.Context, text, _, _ string) (string, error) {
	return text, nil
}

func XunfeiConfigReady(cfg XunfeiConfig) bool {
	return strings.TrimSpace(cfg.AppID) != "" &&
		strings.TrimSpace(cfg.APIKey) != "" &&
		strings.TrimSpace(cfg.SourceLang) != "" &&
		strings.TrimSpace(cfg.TargetLang) != ""
}

func XunfeiMachineTranslationConfigReady(cfg XunfeiMachineTranslationConfig) bool {
	return strings.TrimSpace(cfg.AppID) != "" &&
		strings.TrimSpace(cfg.APIKey) != "" &&
		strings.TrimSpace(cfg.APISecret) != ""
}
