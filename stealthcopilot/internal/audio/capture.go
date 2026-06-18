// Package audio 实现音频设备捕获，为听力链提供 PCM 数据流。
// 生产实现需要 portaudio 系统库（macOS: brew install portaudio）；
// 未安装时自动降级使用 NullCaptureProvider（静音帧）。
package audio

import (
	"context"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

// 音频规格常量（PCM 16kHz 16bit mono，40ms/帧）
const (
	// SampleRate 采样率（Hz）
	SampleRate = 16000
	// FrameDur 每帧时长
	FrameDur = 40 * time.Millisecond
	// FrameSamples 每帧样本数 = SampleRate * FrameDur / 1s
	FrameSamples = 640
	// BytesPerSample PCM 16bit = 2 字节
	BytesPerSample = 2
	// FrameBytes 每帧字节数 = 1280
	FrameBytes = FrameSamples * BytesPerSample
)

// CaptureProvider 从命名音频设备持续读取 PCM 数据，为 Xunfei WebSocket 提供音频帧。
// 生产实现见 capture_portaudio.go（需系统 portaudio 库）。
type CaptureProvider interface {
	// Start 开始捕获并返回 PCM 帧 channel（每帧 FrameBytes 字节，16kHz 16bit mono）。
	// deviceName 为虚拟声卡名称（BlackHole/VB-Cable），ctx 取消时关闭 channel。
	Start(ctx context.Context, deviceName string) (<-chan []byte, error)
	// Close 停止捕获并释放设备资源。
	Close() error
}

// NullCaptureProvider 以静音 PCM 帧模拟音频捕获，用于：
//  1. 单元测试（无需真实音频设备）
//  2. portaudio 未安装时的降级运行（允许整条链路在静音输入下完整测试）
type NullCaptureProvider struct{}

// Start 以 FrameDur 为间隔向 channel 发送零值 PCM 帧，直到 ctx 取消。
// 消费者慢时丢弃当前帧（非阻塞写），避免 goroutine 泄漏。
func (n *NullCaptureProvider) Start(ctx context.Context, _ string) (<-chan []byte, error) {
	diag.Warnf("null audio capture started: silent frames only")
	ch := make(chan []byte, 8)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(FrameDur)
		defer ticker.Stop()
		silence := make([]byte, FrameBytes)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				frame := make([]byte, FrameBytes)
				copy(frame, silence)
				select {
				case ch <- frame:
				default: // 消费者慢时丢弃，保持实时性
				}
			}
		}
	}()
	return ch, nil
}

// Close 无需释放资源（生命周期由 ctx 控制）。
func (n *NullCaptureProvider) Close() error { return nil }
