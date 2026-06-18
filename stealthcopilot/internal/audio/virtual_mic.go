// Package audio — virtual_mic.go 实现虚拟麦克风 PCM 写入，管理 Zero-PCM/TTS 双状态切换。
// 生产实现需要 portaudio 输出流（设备名绑定 BlackHole/VB-Cable）；
// 未安装时使用 NullVirtualMicWriter（丢弃所有写入）。
package audio

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

// virtualMicState 虚拟麦克风写入状态常量
type virtualMicState int32

// VirtualMicSampleRate is the PCM rate expected by the speaking-chain output.
const VirtualMicSampleRate = 24000

const (
	// micStateIdle 初始状态，无音频写入
	micStateIdle virtualMicState = iota
	// micStateZeroPCM VAD 触发后，TTS 首帧未到达前持续写静音
	micStateZeroPCM
	// micStateTTS TTS 首帧到达后切换为真实音频写入
	micStateTTS
)

// VirtualMicWriter 管理虚拟麦克风的 PCM 数据写入，实现 Zero-PCM/TTS 无缝双状态切换。
// 必须保证：VAD 触发后立即开始写 Zero-PCM，TTS 首帧到达时原子切换，TTS 结束后恢复空闲。
type VirtualMicWriter interface {
	// BeginZeroPCM 开始向虚拟麦克风写静音（VAD 触发时调用，立即切换到 micStateZeroPCM）。
	BeginZeroPCM()
	// WriteChunk 写入一段 TTS 音频 chunk（首次调用时原子切换到 micStateTTS）。
	WriteChunk(chunk []byte)
	// EndTTS TTS 流结束，停止音频写入，回到 micStateIdle 待机状态。
	EndTTS()
	// Close 释放所有资源，停止内部 goroutine。
	Close()
}

// NullVirtualMicWriter 是虚拟麦克风写入的空实现，丢弃所有音频数据。
// 用于：portaudio 未安装时的降级运行，以及单元测试。
type NullVirtualMicWriter struct {
	state atomic.Int32
	done  chan struct{}
	once  sync.Once
}

// NewNullVirtualMicWriter 创建 NullVirtualMicWriter 并启动内部 Zero-PCM 定时 goroutine。
func NewNullVirtualMicWriter() *NullVirtualMicWriter {
	diag.Warnf("null virtual mic writer started: audio output will be discarded")
	w := &NullVirtualMicWriter{done: make(chan struct{})}
	go w.zeroPCMLoop()
	return w
}

// BeginZeroPCM 切换到 Zero-PCM 状态（原子写，立即生效）。
func (w *NullVirtualMicWriter) BeginZeroPCM() {
	w.state.Store(int32(micStateZeroPCM))
}

// WriteChunk 首次调用时原子切换到 TTS 状态，后续写入丢弃（NullWriter 不输出）。
func (w *NullVirtualMicWriter) WriteChunk(_ []byte) {
	// 原子 CAS：仅当处于 micStateZeroPCM 时切换到 micStateTTS（防止并发写竞争）
	w.state.CompareAndSwap(int32(micStateZeroPCM), int32(micStateTTS))
}

// EndTTS TTS 结束，回到 micStateIdle。
func (w *NullVirtualMicWriter) EndTTS() {
	w.state.Store(int32(micStateIdle))
}

// Close 停止内部 goroutine，释放资源。
func (w *NullVirtualMicWriter) Close() {
	w.once.Do(func() { close(w.done) })
}

// zeroPCMLoop 模拟虚拟麦克风的定时帧写入节拍（Zero-PCM 状态下生效）。
// 生产实现中该 goroutine 会向 portaudio 输出流写入全零缓冲区。
func (w *NullVirtualMicWriter) zeroPCMLoop() {
	// 24000Hz，每帧 10ms = 240 样本 × 2 字节 = 480 字节
	const frameDur = 10 * time.Millisecond
	ticker := time.NewTicker(frameDur)
	defer ticker.Stop()
	silence := make([]byte, VirtualMicSampleRate/100*BytesPerSample)
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			switch virtualMicState(w.state.Load()) {
			case micStateZeroPCM:
				_ = silence
				// 生产实现：写 882 字节全零到真实输出流
				// NullWriter：丢弃
			case micStateTTS:
				// TTS 音频由 WriteChunk 外部调用写入，此处不重复写入
			case micStateIdle:
				// 空闲状态不写入
			}
		}
	}
}
