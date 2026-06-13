package speaking

import (
	"context"
	"testing"
	"time"
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
