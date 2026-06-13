// Package audio — mic.go 实现物理麦克风 PCM 捕获接口及 PCM 缓冲队列。
// 生产实现需要 portaudio 系统库；未安装时自动使用 NullMicProvider（静音帧）。
package audio

import (
	"context"
	"sync"
)

// MicProvider 从物理麦克风持续读取 PCM 数据，供 VAD 检测使用。
// 采样规格与虚拟声卡捕获一致：16kHz 16bit mono，40ms/帧（FrameBytes 字节）。
type MicProvider interface {
	// Start 开始捕获并返回 PCM 帧 channel，deviceName 为物理麦克风名称。
	// ctx 取消时 goroutine 退出并关闭 channel。
	Start(ctx context.Context, deviceName string) (<-chan []byte, error)
	// Close 停止捕获并释放设备资源。
	Close() error
}

// NullMicProvider 以静音 PCM 帧模拟物理麦克风输入，用于：
//  1. 单元测试（无需真实麦克风）
//  2. portaudio 未安装时的降级运行
//
// 复用 NullCaptureProvider 相同逻辑（MicProvider 与 CaptureProvider 接口相同）。
type NullMicProvider struct {
	inner NullCaptureProvider
}

// Start 以 FrameDur 间隔向 channel 发送静音帧，直到 ctx 取消。
func (n *NullMicProvider) Start(ctx context.Context, deviceName string) (<-chan []byte, error) {
	return n.inner.Start(ctx, deviceName)
}

// Close 无需释放资源。
func (n *NullMicProvider) Close() error { return nil }

// PCMBuffer 是线程安全的 PCM 音频帧累积缓冲区，供 VAD 在静音阈值触发后整段取出。
// VAD 检测时持续 Append，句子结束时 Drain 取出整段音频。
type PCMBuffer struct {
	mu     sync.Mutex
	frames []byte
}

// Append 将一帧 PCM 数据追加到缓冲区。
func (b *PCMBuffer) Append(frame []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.frames = append(b.frames, frame...)
}

// Drain 取出缓冲区中所有积累的 PCM 数据并清空缓冲区。
// 返回自上次 Drain 以来追加的所有数据。
func (b *PCMBuffer) Drain() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.frames
	b.frames = nil
	return out
}

// Len 返回当前缓冲区字节数（线程安全）。
func (b *PCMBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.frames)
}
