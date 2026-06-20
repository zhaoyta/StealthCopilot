package vad

import (
	"context"
	"sync"
	"testing"
	"time"
)

// makePCMFrame 生成 16kHz 16bit 单声道的 40ms 帧（1280 字节），能量由振幅决定。
func makePCMFrame(amplitude int16) []byte {
	const samples = 640 // 16000 * 0.04
	buf := make([]byte, samples*2)
	for i := 0; i < samples; i++ {
		buf[i*2] = byte(amplitude)
		buf[i*2+1] = byte(amplitude >> 8)
	}
	return buf
}

// makeSilenceFrame 生成全零静音帧。
func makeSilenceFrame() []byte { return makePCMFrame(0) }

func TestRMSEnergy(t *testing.T) {
	// 振幅 1000 => RMS ≈ 1000
	frame := makePCMFrame(1000)
	energy := rmsEnergy(frame)
	if energy < 990 || energy > 1010 {
		t.Errorf("rmsEnergy: expected ~1000, got %.2f", energy)
	}

	// 全零 => RMS = 0
	silence := makeSilenceFrame()
	if rmsEnergy(silence) != 0 {
		t.Error("rmsEnergy of silence should be 0")
	}
}

func TestEnergyDetector_SetSilenceThreshold(t *testing.T) {
	d := NewEnergyDetector(800, 40)
	d.SetSilenceThreshold(500)
	if d.silenceMs.Load() != 500 {
		t.Errorf("expected 500, got %d", d.silenceMs.Load())
	}
}

func TestEnergyDetector_SetMaxSpeechMs(t *testing.T) {
	const frameDurMs = 40
	d := NewEnergyDetector(DefaultSilenceThresholdMs, frameDurMs)
	d.SetMaxSpeechMs(2400)
	ch := make(chan []byte, 128)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	segments := make(chan SpeechSegment, 1)
	go d.Run(ctx, ch, func(seg SpeechSegment) {
		segments <- seg
		cancel()
	})

	for i := 0; i < 2400/frameDurMs+5; i++ {
		select {
		case ch <- makePCMFrame(1000):
		case <-ctx.Done():
		}
	}

	select {
	case seg := <-segments:
		if seg.DurationMs != 2400 {
			t.Fatalf("DurationMs = %d, want 2400", seg.DurationMs)
		}
	case <-ctx.Done():
		t.Fatal("expected configured max speech segment")
	}
}

func TestEnergyDetector_Run_DetectsSegment(t *testing.T) {
	const frameDurMs = 40
	// 静音阈值 120ms = 3 帧
	d := NewEnergyDetector(3*frameDurMs, frameDurMs)

	ch := make(chan []byte, 32)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var mu sync.Mutex
	var segments []SpeechSegment

	// 先启动 Run goroutine，再发帧
	go d.Run(ctx, ch, func(seg SpeechSegment) {
		mu.Lock()
		segments = append(segments, seg)
		mu.Unlock()
		cancel() // 收到一段就取消
	})

	// 发送 5 帧有声音（振幅 1000，远超 DefaultEnergyThreshold）
	for i := 0; i < 5; i++ {
		select {
		case ch <- makePCMFrame(1000):
		case <-ctx.Done():
		}
	}
	// 再发送 5 帧静音触发检测
	for i := 0; i < 5; i++ {
		select {
		case ch <- makeSilenceFrame():
		case <-ctx.Done():
		}
	}

	<-ctx.Done()

	mu.Lock()
	got := len(segments)
	mu.Unlock()

	if got < 1 {
		t.Errorf("expected at least 1 segment, got %d", got)
	}
}

func TestEnergyDetector_Run_IgnoresShortSpeech(t *testing.T) {
	// minSpeechMs=200, frameDurMs=40 → 需要 5 帧有声才触发
	const frameDurMs = 40
	d := NewEnergyDetector(DefaultSilenceThresholdMs, frameDurMs)

	ch := make(chan []byte, 16)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var called bool
	// 先启动 Run goroutine，再发帧，防止 channel 满后死锁
	go d.Run(ctx, ch, func(_ SpeechSegment) {
		called = true
		cancel()
	})

	// 仅发送 2 帧有声（不足 minSpeechMs）
	for i := 0; i < 2; i++ {
		select {
		case ch <- makePCMFrame(1000):
		case <-ctx.Done():
		}
	}
	// 发送静音触发判断
	for i := 0; i < 30; i++ {
		select {
		case ch <- makeSilenceFrame():
		case <-ctx.Done():
		}
	}

	<-ctx.Done()

	if called {
		t.Error("should not trigger segment for speech shorter than minSpeechMs")
	}
}

func TestEnergyDetector_Run_CutsLongSpeech(t *testing.T) {
	const frameDurMs = 40
	d := NewEnergyDetector(DefaultSilenceThresholdMs, frameDurMs)
	ch := make(chan []byte, 256)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	segments := make(chan SpeechSegment, 1)
	go d.Run(ctx, ch, func(seg SpeechSegment) {
		segments <- seg
		cancel()
	})

	for i := 0; i < DefaultMaxSpeechMs/frameDurMs+5; i++ {
		select {
		case ch <- makePCMFrame(1000):
		case <-ctx.Done():
		}
	}

	select {
	case seg := <-segments:
		if seg.DurationMs != DefaultMaxSpeechMs {
			t.Fatalf("DurationMs = %d, want %d", seg.DurationMs, DefaultMaxSpeechMs)
		}
	case <-ctx.Done():
		t.Fatal("expected max speech segment")
	}
}

func TestEnergyDetector_Run_CountsIntermittentSilenceInDuration(t *testing.T) {
	const frameDurMs = 40
	d := NewEnergyDetector(3*frameDurMs, frameDurMs)
	ch := make(chan []byte, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	segments := make(chan SpeechSegment, 1)
	go d.Run(ctx, ch, func(seg SpeechSegment) {
		segments <- seg
		cancel()
	})

	for i := 0; i < 5; i++ {
		ch <- makePCMFrame(1000)
	}
	for i := 0; i < 2; i++ {
		ch <- makeSilenceFrame()
	}
	for i := 0; i < 5; i++ {
		ch <- makePCMFrame(1000)
	}
	for i := 0; i < 3; i++ {
		ch <- makeSilenceFrame()
	}

	select {
	case seg := <-segments:
		if seg.DurationMs != 15*frameDurMs {
			t.Fatalf("DurationMs = %d, want %d", seg.DurationMs, 15*frameDurMs)
		}
	case <-ctx.Done():
		t.Fatal("expected segment")
	}
}
