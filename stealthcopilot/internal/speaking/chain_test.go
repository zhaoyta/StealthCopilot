package speaking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/vad"
)

func TestChain_StartStop(t *testing.T) {
	c := &Chain{}
	// 空配置（NullProvider 降级），Start 应返回空字符串（成功）
	ctx := context.Background()
	result := c.Start(ctx, ChainConfig{SilenceThresholdMs: 400})
	if result != "" {
		t.Errorf("Start with null providers: expected empty string, got %q", result)
	}

	// Stop 应无死锁
	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Stop did not return within 2 seconds (possible deadlock)")
	}
}

func TestChain_StartIdempotent(t *testing.T) {
	c := &Chain{}
	ctx := context.Background()

	// 连续启动两次，第二次应先停止旧链，不 panic
	c.Start(ctx, ChainConfig{SilenceThresholdMs: 400})
	result := c.Start(ctx, ChainConfig{SilenceThresholdMs: 800})
	if result != "" {
		t.Errorf("second Start: expected empty string, got %q", result)
	}
	c.Stop()
}

func TestChain_SetSilenceThreshold_BeforeStart(t *testing.T) {
	c := &Chain{}
	// 在 Start 之前调用 SetSilenceThreshold 不应 panic（vadDetect 为 nil）
	c.SetSilenceThreshold(500)
}

func TestChain_SetSilenceThreshold_AfterStart(t *testing.T) {
	c := &Chain{}
	ctx := context.Background()
	c.Start(ctx, ChainConfig{SilenceThresholdMs: 400})

	// 运行时更新阈值，不应 panic
	c.SetSilenceThreshold(600)
	c.Stop()
}

func TestChain_StartWithVirtualMicRequiresRealWriter(t *testing.T) {
	t.Setenv("PATH", "")
	c := &Chain{}
	result := c.Start(context.Background(), ChainConfig{
		SilenceThresholdMs: 400,
		VirtualMicDevice:   "1",
	})
	if result == "" {
		c.Stop()
		t.Fatal("expected startup error when virtual mic is configured but ffmpeg is unavailable")
	}
}

func TestChain_StartWithPhysicalMicRequiresSimultConfig(t *testing.T) {
	t.Setenv("PATH", "")
	c := &Chain{}
	result := c.Start(context.Background(), ChainConfig{
		SilenceThresholdMs: 400,
		PhysicalMicDevice:  "0",
	})
	if result == "" {
		c.Stop()
		t.Fatal("expected startup error for missing Xunfei config")
	}
}

func TestChain_StartWithVirtualMicRequiresXunfeiVoiceCloneConfig(t *testing.T) {
	c := &Chain{}
	result := c.Start(context.Background(), ChainConfig{
		SilenceThresholdMs: 400,
		VirtualMicDevice:   "1",
	})
	if result == "" {
		c.Stop()
		t.Fatal("expected startup error for missing Xunfei VoiceClone config")
	}
}

func TestIsContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !isContextCanceled(ctx, errors.New("xunfei_simult: dial: operation was canceled")) {
		t.Fatal("canceled context should suppress speaking error events")
	}
	if !isContextCanceled(context.Background(), context.Canceled) {
		t.Fatal("context.Canceled error should be treated as cancellation")
	}
	if isContextCanceled(context.Background(), errors.New("network failed")) {
		t.Fatal("ordinary errors should still be reported")
	}
}

func TestSrcTextOrTargetPrefersCandidateSourceLanguage(t *testing.T) {
	if got := srcTextOrTarget("我负责支付系统架构", "I owned the payment architecture."); got != "我负责支付系统架构" {
		t.Fatalf("srcTextOrTarget should prefer source text, got %q", got)
	}
	if got := srcTextOrTarget("", "I owned the payment architecture."); got != "I owned the payment architecture." {
		t.Fatalf("srcTextOrTarget should fall back to target text, got %q", got)
	}
}

func TestSplitSegmentForSpeaking(t *testing.T) {
	pcm := make([]byte, audio.FrameBytes*200)
	parts := splitSegmentForSpeaking(vad.SpeechSegment{
		PCM:        pcm,
		DurationMs: pcmDurationMs(pcm),
	})
	if len(parts) != 4 {
		t.Fatalf("len(parts) = %d, want 4", len(parts))
	}
	total := 0
	for i, part := range parts {
		if got := pcmDurationMs(part.PCM); got > speakingMaxSpeechMs {
			t.Fatalf("part %d duration = %d, want <= %d", i, got, speakingMaxSpeechMs)
		}
		total += len(part.PCM)
	}
	if total != len(pcm) {
		t.Fatalf("total bytes = %d, want %d", total, len(pcm))
	}
}
