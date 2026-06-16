// Package video — chain.go 实现视频链管道协调器。
// 正常模式：物理摄像头帧 → Simli 口型同步 → Ring Buffer A/V 对齐 → 虚拟摄像头
// 熔断模式：物理摄像头帧直通虚拟摄像头（≤10ms 切换，原子 bool）
// 始终保持物理摄像头捕获（热备），切换仅改变输出端。
package video

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/circuit"
	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
)

// ChainConfig 视频链运行时配置。
type ChainConfig struct {
	SimliAPIKey        string // Simli API Key（空则降级为摄像头直通）
	SilmiFaceID        string // Simli Face ID
	SimliHeartbeatAddr string // UDP 心跳目标地址（Simli API 端点）
	PhysicalCamDevice  string // 物理摄像头设备名
	VirtualCamDevice   string // 虚拟摄像头设备名
	LipSyncProvider    lipsync.Provider
	LipSyncCloudMode   bool
}

// Chain 视频链协调器，持有各组件实例和运行状态。
type Chain struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	breaker   *circuit.Breaker
	lipSync   lipsync.Provider
	ring      *RingBuffer
	cam       CaptureProvider
	vcWriter  VirtualCameraWriter
	startTime time.Time
}

// Start 以给定配置启动视频链（幂等，已运行时先停止再重启）。
// 返回空字符串表示成功，否则返回错误描述。
func (c *Chain) Start(wailsCtx context.Context, cfg ChainConfig) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.wg.Wait()
	}

	ctx, cancel := context.WithCancel(wailsCtx)
	c.cancel = cancel

	// 熔断器（状态变化时推送 Wails 事件）
	breaker := circuit.NewBreaker(cfg.SimliHeartbeatAddr, func(state circuit.State) {
		switch state {
		case circuit.StateOpen:
			runtime.EventsEmit(wailsCtx, circuit.EventCircuitOpen)
		case circuit.StateClosed:
			runtime.EventsEmit(wailsCtx, circuit.EventCircuitClosed)
		}
	})
	c.breaker = breaker
	breaker.Start(ctx)

	// 物理摄像头捕获（热备，始终运行）：用户选择设备时使用系统采集。
	var cam CaptureProvider = &NullCaptureProvider{}
	if cfg.PhysicalCamDevice != "" {
		cam = NewSystemCaptureProvider()
	}
	camCh, err := cam.Start(ctx, cfg.PhysicalCamDevice)
	if err != nil {
		cancel()
		c.cancel = nil
		return "摄像头启动失败：" + err.Error()
	}

	// 虚拟摄像头写入器：用户选择设备时必须是真实 writer，不能静默丢帧。
	vcWriter, writerErr := NewSystemVirtualCameraWriterChecked(cfg.VirtualCamDevice)
	if writerErr != "" {
		cancel()
		_ = cam.Close()
		c.cancel = nil
		return writerErr
	}

	var lipSync lipsync.Provider
	cloudMode := cfg.LipSyncCloudMode
	switch {
	case cfg.LipSyncProvider != nil:
		lipSync = cfg.LipSyncProvider
		if startErr := lipSync.Start(ctx, cfg.SilmiFaceID); startErr != nil {
			lipSync = lipsync.NewNullLipSyncProvider()
			cloudMode = false
		}
	case cfg.SimliAPIKey != "" && cfg.SilmiFaceID != "":
		p := lipsync.NewSimliProvider(lipsync.SimliConfig{
			APIKey: cfg.SimliAPIKey,
			FaceID: cfg.SilmiFaceID,
		})
		if startErr := p.Start(ctx, cfg.SilmiFaceID); startErr != nil {
			// Simli 连接失败，降级为直通
			lipSync = lipsync.NewNullLipSyncProvider()
		} else {
			lipSync = p
			cloudMode = true
		}
	default:
		lipSync = lipsync.NewNullLipSyncProvider()
	}

	// Ring Buffer（延迟超限时触发熔断）
	rb := NewRingBuffer(64, func(lagMs int64) {
		breaker.TripFromLag(lagMs)
	})
	c.lipSync = lipSync
	c.ring = rb
	c.cam = cam
	c.vcWriter = vcWriter
	c.startTime = time.Now()

	c.wg.Add(3)

	// goroutine 1：摄像头帧 → Simli / 直通虚拟摄像头
	go func() {
		defer c.wg.Done()
		for frame := range camCh {
			if breaker.IsOpen() || !cloudMode {
				// 熔断或云端不可用模式：直接写入虚拟摄像头（≤1ms 路径）
				_ = vcWriter.WriteFrame(frame)
				continue
			}
			// 正常模式：发送给 Simli + 写入 Ring Buffer
			_ = lipSync.SendVideo(frame)
		}
	}()

	// goroutine 2：Simli 输出帧写入 Ring Buffer
	go func() {
		defer c.wg.Done()
		for frame := range lipSync.Output() {
			rb.PushVideo(frame)
		}
	}()

	// goroutine 3：Ring Buffer 对齐输出 → 虚拟摄像头（正常模式）
	go func() {
		defer c.wg.Done()
		stopCh := make(chan struct{})

		// 在子 goroutine 监听 ctx 取消
		go func() {
			<-ctx.Done()
			close(stopCh)
		}()

		go rb.RunAligner(stopCh)

		for pair := range rb.Output() {
			if !breaker.IsOpen() {
				_ = vcWriter.WriteFrame(pair.Video)
			}
			// 音频帧（pair.Audio）由说话链/TTS 直接写虚拟麦克风
			// Ring Buffer 对齐仅确保 A/V 时序，不重复写音频
		}
	}()

	// goroutine 4：心跳辅助协程（用于测试 Simli 连接是否仍然活跃）
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 每 5s 检查一次：若熔断后连接已恢复，尝试重建 Simli 连接
				// （实际重连由 circuit.Breaker 的 halfOpen/close 驱动）
			}
		}
	}()

	return ""
}

// Stop 停止视频链，等待所有 goroutine 退出。
func (c *Chain) Stop() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.breaker != nil {
		c.breaker.Stop()
	}
	if c.cam != nil {
		_ = c.cam.Close()
		c.cam = nil
	}
	if c.lipSync != nil {
		_ = c.lipSync.Close()
		c.lipSync = nil
	}
	if c.vcWriter != nil {
		_ = c.vcWriter.Close()
		c.vcWriter = nil
	}
	c.ring = nil
	c.mu.Unlock()
	c.wg.Wait()
}

// SendAudioChunk forwards synthesized speech into Simli and the A/V ring buffer.
// It is intentionally non-blocking for callers: audio output to the virtual mic
// must keep priority over lip-sync enrichment.
func (c *Chain) SendAudioChunk(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	c.mu.Lock()
	lipSync := c.lipSync
	rb := c.ring
	start := c.startTime
	c.mu.Unlock()
	if lipSync == nil || rb == nil || start.IsZero() {
		return
	}
	pts := time.Since(start).Milliseconds()
	audio := lipsync.AudioChunk{Data: chunk, PTS: pts}
	_ = lipSync.SendAudio(audio)
	rb.PushAudio(AudioFrame{Data: chunk, PTS: pts})
}

// TripCircuit 手动触发熔断（用户点击"紧急降级"按钮时调用）。
func (c *Chain) TripCircuit() {
	c.mu.Lock()
	b := c.breaker
	c.mu.Unlock()
	if b != nil {
		b.TripFromLag(0)
	}
}

// IsCircuitOpen 返回熔断器是否处于 Open 状态（供前端查询）。
func (c *Chain) IsCircuitOpen() bool {
	c.mu.Lock()
	b := c.breaker
	c.mu.Unlock()
	if b == nil {
		return false
	}
	return b.IsOpen()
}
