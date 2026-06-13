// Package stt 定义语音转文字（STT）Provider 接口。
// 具体实现（如讯飞 ASR）在同包的 xunfei.go 中提供。
package stt

import "context"

// Result 表示一次 STT 识别结果。
// IsFinal 为 true 时表示该句识别完成，中间结果 IsFinal 为 false。
type Result struct {
	Text    string // 识别出的文本
	IsFinal bool   // 是否为最终结果（句子完整）
}

// Provider 是 STT 服务的统一抽象接口。
// 所有 STT 实现（讯飞、Whisper 等）必须实现此接口。
type Provider interface {
	// Transcribe 开始实时转写，从 audioStream 读取 PCM 数据（16kHz 16bit mono）。
	// 返回结果 channel，调用方通过 cancel ctx 停止转写。
	Transcribe(ctx context.Context, audioStream <-chan []byte) (<-chan Result, error)

	// Close 释放连接资源。
	Close() error
}
