// Package vad 实现语音活动检测（Voice Activity Detection）。
// 当前使用能量阈值 VAD（EnergyVAD），检测 PCM 帧 RMS 能量。
// 接口设计与 WebRTC VAD 兼容，后续可无缝替换为 go-webrtcvad 等精度更高的实现。
package vad

import (
	"context"
	"encoding/binary"
	"math"
	"sync/atomic"
	"time"
)

// 默认配置常量
const (
	// DefaultSilenceThresholdMs 连续静音超过此时长（毫秒）触发 VAD 回调
	DefaultSilenceThresholdMs = 800
	// DefaultEnergyThreshold RMS 能量低于此值判定为静音帧（16bit PCM 满量程 32768）
	DefaultEnergyThreshold = 200.0
	// DefaultMaxSpeechMs 单段语音最长时长，避免长回答一次性送入下游导致超时。
	DefaultMaxSpeechMs = 8000
	// minSpeechMs 至少检测到此时长的语音才触发 VAD（避免瞬时噪声误触发）
	minSpeechMs = 200
)

// SpeechSegment 是 VAD 回调携带的一段完整语音 PCM 数据。
type SpeechSegment struct {
	// PCM 是整段语音的原始字节（16kHz 16bit mono）
	PCM []byte
	// DurationMs 是语音段时长（毫秒）
	DurationMs int
}

// Detector 是 VAD 的主接口，对外提供检测能力。
// EnergyDetector 是当前默认实现；后续可替换为 WebRTCDetector。
type Detector interface {
	// SetSilenceThreshold 运行时更新静音触发阈值（毫秒），即时生效。
	SetSilenceThreshold(ms int)
	// Run 启动 VAD 检测循环，从 audioStream 读取 PCM 帧，
	// 检测到完整语音段时调用 onSegment 回调。ctx 取消时退出。
	Run(ctx context.Context, audioStream <-chan []byte, onSegment func(seg SpeechSegment))
}

// EnergyDetector 基于 RMS 能量阈值实现 VAD。
// 算法：逐帧计算 RMS，能量高于 energyThreshold 判定为语音帧，否则为静音帧；
// 连续静音帧超过 silenceThresholdMs 且已积累足够语音（minSpeechMs）时触发回调。
type EnergyDetector struct {
	silenceMs    atomic.Int64 // 静音阈值（毫秒），支持运行时原子更新
	maxSpeechMs  atomic.Int64 // 单段最长语音时长（毫秒），支持不同链路按延迟目标调整
	energyThresh float64      // RMS 能量阈值
	frameDurMs   int          // 每帧时长（ms），用于计算帧数阈值
}

// NewEnergyDetector 创建能量阈值 VAD 检测器。
// silenceThresholdMs：静音触发时长（毫秒），0 时使用默认值；
// frameDurMs：每帧时长（默认 40ms，与 audio.FrameDur 一致）。
func NewEnergyDetector(silenceThresholdMs, frameDurMs int) *EnergyDetector {
	if silenceThresholdMs <= 0 {
		silenceThresholdMs = DefaultSilenceThresholdMs
	}
	if frameDurMs <= 0 {
		frameDurMs = 40
	}
	d := &EnergyDetector{
		energyThresh: DefaultEnergyThreshold,
		frameDurMs:   frameDurMs,
	}
	d.silenceMs.Store(int64(silenceThresholdMs))
	d.maxSpeechMs.Store(DefaultMaxSpeechMs)
	return d
}

// SetSilenceThreshold 运行时更新静音阈值（毫秒），原子写，即时生效。
func (d *EnergyDetector) SetSilenceThreshold(ms int) {
	d.silenceMs.Store(int64(ms))
}

// SetMaxSpeechMs 运行时更新单段最长语音时长（毫秒），用于控制下游延迟。
func (d *EnergyDetector) SetMaxSpeechMs(ms int) {
	if ms <= 0 {
		ms = DefaultMaxSpeechMs
	}
	d.maxSpeechMs.Store(int64(ms))
}

// Run 启动 VAD 检测主循环。
// 状态机：idle（无语音）→ speaking（有语音，累积帧）→ idle（静音触发，回调）
func (d *EnergyDetector) Run(
	ctx context.Context,
	audioStream <-chan []byte,
	onSegment func(seg SpeechSegment),
) {
	var (
		speechBuf     []byte // 累积语音 PCM
		silenceCount  int    // 连续静音帧数
		speechFrames  int    // 已累积语音帧数（用于判断 minSpeechMs）
		segmentFrames int    // 当前语音段累计帧数，包含短静音
		inSpeech      bool   // 是否处于语音段内
	)
	minSpeechFrames := minSpeechMs / d.frameDurMs

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-audioStream:
			if !ok {
				return
			}
			isSpeech := rmsEnergy(frame) >= d.energyThresh

			if isSpeech {
				inSpeech = true
				silenceCount = 0
				speechFrames++
				segmentFrames++
				speechBuf = append(speechBuf, frame...)
				maxSpeechFrames := int(d.maxSpeechMs.Load()) / d.frameDurMs
				if segmentFrames >= maxSpeechFrames && speechFrames >= minSpeechFrames {
					onSegment(SpeechSegment{
						PCM:        speechBuf,
						DurationMs: segmentFrames * d.frameDurMs,
					})
					speechBuf = nil
					silenceCount = 0
					speechFrames = 0
					segmentFrames = 0
					inSpeech = false
				}
			} else if inSpeech {
				silenceCount++
				segmentFrames++
				speechBuf = append(speechBuf, frame...) // 静音帧也纳入以保持自然尾音
				threshFrames := int(d.silenceMs.Load()) / d.frameDurMs
				if silenceCount >= threshFrames && speechFrames >= minSpeechFrames {
					// 触发回调
					seg := SpeechSegment{
						PCM:        speechBuf,
						DurationMs: segmentFrames * d.frameDurMs,
					}
					onSegment(seg)
					// 重置状态
					speechBuf = nil
					silenceCount = 0
					speechFrames = 0
					segmentFrames = 0
					inSpeech = false
				}
			}
		}
	}
}

// rmsEnergy 计算 PCM 帧（16bit little-endian）的 RMS 能量值。
func rmsEnergy(frame []byte) float64 {
	if len(frame) < 2 {
		return 0
	}
	sampleCount := len(frame) / 2
	var sumSq float64
	for i := 0; i < sampleCount; i++ {
		sample := int16(binary.LittleEndian.Uint16(frame[i*2:]))
		sumSq += float64(sample) * float64(sample)
	}
	return math.Sqrt(sumSq / float64(sampleCount))
}

// SilenceTimeout 是 VAD 等待静音超时的辅助函数，用于测试和调试。
func SilenceTimeout(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
