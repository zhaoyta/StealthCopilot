package speaking

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

func startVirtualMicLoopbackCheck(ctx context.Context, deviceName string, segmentID int64) func() {
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		return func() {}
	}

	checkCtx, cancel := context.WithCancel(ctx)
	provider, msg := audio.NewSystemCaptureProviderChecked()
	if msg != "" {
		diag.Warnf("speaking loopback check unavailable segment=%d device=%q err=%q", segmentID, deviceName, msg)
		return cancel
	}

	stream, err := provider.Start(checkCtx, deviceName)
	if err != nil {
		cancel()
		_ = provider.Close()
		diag.Warnf("speaking loopback check start failed segment=%d device=%q err=%v", segmentID, deviceName, err)
		return func() {}
	}

	var peak atomic.Int64
	var frames atomic.Int64
	started := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer provider.Close()
		for {
			select {
			case <-checkCtx.Done():
				return
			case frame, ok := <-stream:
				if !ok {
					return
				}
				frames.Add(1)
				p := int64(audio.PCMPeak(frame))
				for {
					old := peak.Load()
					if p <= old || peak.CompareAndSwap(old, p) {
						break
					}
				}
			}
		}
	}()

	return func() {
		select {
		case <-ctx.Done():
		case <-time.After(1200 * time.Millisecond):
		}
		cancel()
		select {
		case <-done:
		case <-time.After(300 * time.Millisecond):
		}
		diag.Infof("speaking loopback check done segment=%d device=%q elapsed=%s frames=%d loopback_peak=%d", segmentID, deviceName, diag.Since(started), frames.Load(), peak.Load())
	}
}
