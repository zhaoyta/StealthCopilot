package video

import (
	"context"
	"testing"
	"time"
)

func TestNullCaptureProvider_Start(t *testing.T) {
	p := &NullCaptureProvider{}
	defer p.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch, err := p.Start(ctx, "")
	if err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	received := 0
	deadline := time.After(150 * time.Millisecond)
loop:
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				break loop
			}
			if f.PTS <= 0 {
				t.Error("PTS should be positive")
			}
			received++
			if received >= 2 {
				break loop
			}
		case <-deadline:
			break loop
		}
	}
	if received < 2 {
		t.Errorf("expected at least 2 frames, got %d", received)
	}
}

func TestNullCaptureProvider_ListDevices(t *testing.T) {
	p := &NullCaptureProvider{}
	if devs := p.ListDevices(); len(devs) != 0 {
		t.Errorf("NullCaptureProvider.ListDevices should return empty, got %v", devs)
	}
}

func TestNullCaptureProvider_CloseTwice(t *testing.T) {
	p := &NullCaptureProvider{}
	ctx := context.Background()
	p.Start(ctx, "") //nolint
	p.Close()
	p.Close() // 不应 panic
}
