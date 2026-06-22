// Package video — ring_buffer.go 实现音频/视频双通道环形缓冲区。
// 功能：
//  1. 分别缓存带 PTS 时间戳的音频帧和视频帧（各最多 maxFrames 帧）
//  2. 持续检测是否存在 PTS delta ≤ ptsTolerance 的帧对并输出
//  3. 缓冲区超限时丢弃最旧帧（溢出保护）
//  4. 视频 PTS 落后音频 PTS 超过 lagThresholdMs 时触发熔断回调
package video

import (
	"sync"
	"time"
)

const (
	// maxFrames 每个通道最大缓冲帧数（≈ 2s × 60fps）
	maxFrames = 120
	// ptsTolerance 帧对 PTS 最大允许差值（毫秒）
	ptsTolerance = 40
	// lagThresholdMs 视频 PTS 落后音频 PTS 超过此值时触发熔断（毫秒）
	lagThresholdMs = 300
	// alignTickDur 对齐检测间隔
	alignTickDur = 5 * time.Millisecond
)

// AudioFrame 带 PTS 时间戳的音频帧（用于环形缓冲区对齐）。
type AudioFrame struct {
	Data []byte
	PTS  int64 // 毫秒
}

// AlignedPair 一对已对齐的音频+视频帧（PTS delta ≤ ptsTolerance）。
type AlignedPair struct {
	Audio AudioFrame
	Video Frame
}

// OnLagFunc 视频延迟超出阈值时的熔断回调函数类型。
type OnLagFunc func(lagMs int64)

// RingBuffer 音视频双通道环形缓冲区，负责 PTS 对齐和溢出保护。
type RingBuffer struct {
	mu         sync.Mutex
	audioQueue []AudioFrame
	videoQueue []Frame
	output     chan AlignedPair
	onLag      OnLagFunc
	once       sync.Once
}

// NewRingBuffer 创建环形缓冲区，output channel 容量为 outputCap。
// onLag 在视频 PTS 落后超过 lagThresholdMs 时被调用（可为 nil）。
func NewRingBuffer(outputCap int, onLag OnLagFunc) *RingBuffer {
	if outputCap <= 0 {
		outputCap = 32
	}
	return &RingBuffer{
		output: make(chan AlignedPair, outputCap),
		onLag:  onLag,
	}
}

// PushAudio 将音频帧入队（溢出时丢弃最旧帧）。
func (r *RingBuffer) PushAudio(frame AudioFrame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audioQueue = append(r.audioQueue, frame)
	if len(r.audioQueue) > maxFrames {
		r.audioQueue = r.audioQueue[len(r.audioQueue)-maxFrames:] // 丢弃最旧帧
	}
}

// PushVideo 将视频帧入队（溢出时丢弃最旧帧）。
func (r *RingBuffer) PushVideo(frame Frame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.videoQueue = append(r.videoQueue, frame)
	if len(r.videoQueue) > maxFrames {
		r.videoQueue = r.videoQueue[len(r.videoQueue)-maxFrames:] // 丢弃最旧帧
	}
}

// Output 返回已对齐的帧对 channel（消费方将音频写虚拟麦、视频写虚拟摄像头）。
func (r *RingBuffer) Output() <-chan AlignedPair { return r.output }

// RunAligner 启动 PTS 对齐检测循环（阻塞，在独立 goroutine 中调用）。
// ctx 取消时退出，关闭 output channel。
func (r *RingBuffer) RunAligner(stopCh <-chan struct{}) {
	ticker := time.NewTicker(alignTickDur)
	defer ticker.Stop()
	defer r.once.Do(func() { close(r.output) })

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			r.align()
		}
	}
}

// align 在两个队列中查找 PTS delta ≤ ptsTolerance 的帧对并弹出。
// 同时检测视频落后是否超过 lagThresholdMs 触发熔断回调。
func (r *RingBuffer) align() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.audioQueue) == 0 || len(r.videoQueue) == 0 {
		return
	}

	// 检查视频 PTS 落后于音频最新帧的延迟
	latestAudioPTS := r.audioQueue[len(r.audioQueue)-1].PTS
	oldestVideoPTS := r.videoQueue[0].PTS
	lag := latestAudioPTS - oldestVideoPTS
	if lag > lagThresholdMs && r.onLag != nil {
		r.onLag(lag)
		return
	}

	// 找最优帧对（delta 最小）
	bestA, bestV, bestDelta := -1, -1, int64(ptsTolerance+1)
	for ai, a := range r.audioQueue {
		for vi, v := range r.videoQueue {
			d := a.PTS - v.PTS
			if d < 0 {
				d = -d
			}
			if d < bestDelta {
				bestDelta = d
				bestA, bestV = ai, vi
			}
		}
	}
	if bestA < 0 || bestDelta > ptsTolerance {
		return // 没有满足条件的帧对
	}

	pair := AlignedPair{
		Audio: r.audioQueue[bestA],
		Video: r.videoQueue[bestV],
	}

	// 弹出已匹配的帧（及其之前更旧的帧）
	r.audioQueue = removeUpTo(r.audioQueue, bestA)
	r.videoQueue = removeVideoUpTo(r.videoQueue, bestV)

	select {
	case r.output <- pair:
	default:
		// output 满时丢帧（下游消费不及时）
	}
}

// removeUpTo 移除 slice 中索引 0..idx（含）的元素，返回剩余部分。
func removeUpTo(q []AudioFrame, idx int) []AudioFrame {
	if idx+1 >= len(q) {
		return nil
	}
	return q[idx+1:]
}

func removeVideoUpTo(q []Frame, idx int) []Frame {
	if idx+1 >= len(q) {
		return nil
	}
	return q[idx+1:]
}

// Drain 清空两个队列（熔断时调用）。
func (r *RingBuffer) Drain() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audioQueue = nil
	r.videoQueue = nil
}
