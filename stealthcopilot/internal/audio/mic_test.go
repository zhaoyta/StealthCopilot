package audio

import (
	"context"
	"testing"
	"time"
)

func TestPCMBuffer_AppendDrain(t *testing.T) {
	buf := &PCMBuffer{}
	if buf.Len() != 0 {
		t.Error("new buffer should be empty")
	}

	buf.Append([]byte{1, 2, 3})
	buf.Append([]byte{4, 5})
	if buf.Len() != 5 {
		t.Errorf("expected 5, got %d", buf.Len())
	}

	data := buf.Drain()
	if len(data) != 5 {
		t.Errorf("Drain: expected 5 bytes, got %d", len(data))
	}
	if buf.Len() != 0 {
		t.Error("buffer should be empty after Drain")
	}
}

func TestNullMicProvider_Start(t *testing.T) {
	p := &NullMicProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch, err := p.Start(ctx, "")
	if err != nil {
		t.Fatalf("NullMicProvider.Start: unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("NullMicProvider.Start should return non-nil channel")
	}

	// 等待至少 2 帧（40ms/帧），验证 channel 持续产出静音帧
	received := 0
	timeout := time.After(150 * time.Millisecond)
loop:
	for {
		select {
		case frame := <-ch:
			if len(frame) != FrameBytes {
				t.Errorf("frame size: expected %d, got %d", FrameBytes, len(frame))
			}
			received++
			if received >= 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if received < 2 {
		t.Errorf("expected at least 2 frames, received %d", received)
	}

	if err := p.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}
