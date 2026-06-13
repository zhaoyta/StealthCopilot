package video

import (
	"testing"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
)

func TestRingBuffer_PushAndAlign(t *testing.T) {
	rb := NewRingBuffer(16, nil)
	stopCh := make(chan struct{})
	go rb.RunAligner(stopCh)
	defer close(stopCh)

	// 推入匹配的音视频帧（PTS 差值 = 0，满足 ≤40ms 条件）
	rb.PushAudio(AudioFrame{Data: []byte{1}, PTS: 100})
	rb.PushVideo(lipsync.VideoFrame{Data: []byte{2}, PTS: 100})

	select {
	case pair := <-rb.Output():
		if pair.Audio.PTS != 100 || pair.Video.PTS != 100 {
			t.Errorf("unexpected pair PTS: audio=%d video=%d", pair.Audio.PTS, pair.Video.PTS)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("expected aligned pair within 200ms")
	}
}

func TestRingBuffer_NoAlignWhenDeltaTooLarge(t *testing.T) {
	rb := NewRingBuffer(16, nil)
	stopCh := make(chan struct{})
	go rb.RunAligner(stopCh)
	defer close(stopCh)

	// PTS 差值 100ms > 40ms，不应产生对齐帧对
	rb.PushAudio(AudioFrame{Data: []byte{1}, PTS: 0})
	rb.PushVideo(lipsync.VideoFrame{Data: []byte{2}, PTS: 100})

	select {
	case <-rb.Output():
		t.Error("should not produce pair when delta > ptsTolerance")
	case <-time.After(100 * time.Millisecond):
		// 正确：超时无输出
	}
}

func TestRingBuffer_OverflowProtection(t *testing.T) {
	rb := NewRingBuffer(16, nil)
	// 推入超过 maxFrames 的音频帧，不应 panic 且 Len ≤ maxFrames
	for i := 0; i < maxFrames+10; i++ {
		rb.PushAudio(AudioFrame{PTS: int64(i)})
	}
	rb.mu.Lock()
	l := len(rb.audioQueue)
	rb.mu.Unlock()
	if l > maxFrames {
		t.Errorf("audioQueue len %d exceeds maxFrames %d", l, maxFrames)
	}
}

func TestRingBuffer_LagTriggersCallback(t *testing.T) {
	triggered := false
	rb := NewRingBuffer(16, func(lagMs int64) {
		triggered = true
	})
	stopCh := make(chan struct{})
	go rb.RunAligner(stopCh)
	defer close(stopCh)

	// 音频 PTS = 500，视频 PTS = 0 → lag = 500ms > 300ms → 触发回调
	rb.PushAudio(AudioFrame{PTS: 500})
	rb.PushVideo(lipsync.VideoFrame{PTS: 0})

	time.Sleep(50 * time.Millisecond)
	if !triggered {
		t.Error("expected onLag callback to be triggered")
	}
}

func TestRingBuffer_Drain(t *testing.T) {
	rb := NewRingBuffer(16, nil)
	rb.PushAudio(AudioFrame{PTS: 1})
	rb.PushVideo(lipsync.VideoFrame{PTS: 1})
	rb.Drain()

	rb.mu.Lock()
	al, vl := len(rb.audioQueue), len(rb.videoQueue)
	rb.mu.Unlock()

	if al != 0 || vl != 0 {
		t.Errorf("after Drain: audio=%d video=%d, expected 0", al, vl)
	}
}
