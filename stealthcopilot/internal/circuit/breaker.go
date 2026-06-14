// Package circuit 实现 StealthCopilot 的熔断器。
// 状态机：Closed（正常）→ Open（降级直通）→ HalfOpen（尝试恢复）→ Closed
// 触发条件：连续 3 次心跳丢失（50ms 间隔）或视频 PTS 落后音频 >300ms。
// 心跳协议自动路由：
//   - targetAddr 以 "http" 开头 → HTTP HEAD 健康检查（适用于 Simli 等云端 HTTP API）
//   - targetAddr 为 "host:port" 格式 → UDP ping/pong
//   - targetAddr 为空 → 视为存活（禁用心跳检测，不触发误熔断）
//
// 切换延迟 ≤10ms（原子 bool 切换输出端，无锁路径）。
package circuit

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// State 熔断器状态
type State int32

const (
	// StateClosed 正常状态，云端管道输出
	StateClosed State = iota
	// StateOpen 熔断状态，直通真实硬件
	StateOpen
	// StateHalfOpen 尝试恢复，等待连续心跳确认
	StateHalfOpen
)

// 心跳相关常量
const (
	heartbeatInterval    = 50 * time.Millisecond  // 心跳发送间隔
	tripThreshold        = 3                      // 连续丢失触发熔断
	recoverThreshold     = 3                      // 连续恢复关闭熔断
	heartbeatDialTimeout = 20 * time.Millisecond  // UDP 心跳响应超时
	httpHealthTimeout    = 300 * time.Millisecond // HTTP 健康检查超时（含 DNS + TCP + 响应）
)

// Wails 事件名常量
const (
	EventCircuitOpen   = "circuit:open"   // 熔断触发事件
	EventCircuitClosed = "circuit:closed" // 熔断恢复事件
)

// OnStateChange 熔断状态变化回调函数类型（用于 Wails EventsEmit）。
type OnStateChange func(newState State)

// Breaker 熔断器，负责心跳检测和状态切换。
type Breaker struct {
	state         atomic.Int32  // 当前状态（State 类型）
	missCount     int           // 连续心跳丢失次数（仅主循环读写）
	recoverCount  int           // 连续心跳恢复次数（仅主循环读写）
	targetAddr    string        // 心跳目标：http(s) URL 或 UDP "host:port"
	onStateChange OnStateChange // 状态变化回调

	bypass atomic.Bool // true = 直通模式（Open），false = 云端模式（Closed）

	mu         sync.Mutex
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	httpClient *http.Client // HTTP 健康检查专用客户端（复用连接）
}

// NewBreaker 创建熔断器。
//
// targetAddr 支持两种格式：
//   - "https://api.simli.ai" 等 HTTP(S) URL → 心跳改走 HTTP HEAD
//   - "1.2.3.4:9000" 等 UDP 地址 → 沿用 UDP ping/pong
//   - "" → 禁用心跳，适合纯靠 TripFromLag 触发的场景
func NewBreaker(targetAddr string, onStateChange OnStateChange) *Breaker {
	b := &Breaker{
		targetAddr:    targetAddr,
		onStateChange: onStateChange,
		httpClient: &http.Client{
			Timeout: httpHealthTimeout,
			// 不跟随重定向，避免超时累积
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	b.state.Store(int32(StateClosed))
	return b
}

// Start 启动心跳检测循环（非阻塞）。
func (b *Breaker) Start(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
		b.wg.Wait()
	}
	ctx2, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.wg.Add(1)
	go b.heartbeatLoop(ctx2)
}

// Stop 停止心跳检测，等待 goroutine 退出。
func (b *Breaker) Stop() {
	b.mu.Lock()
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	b.mu.Unlock()
	b.wg.Wait()
}

// TripFromLag 由 Ring Buffer 延迟监控调用，直接触发熔断（无需等待心跳丢失）。
func (b *Breaker) TripFromLag(lagMs int64) {
	_ = lagMs
	b.trip()
}

// IsOpen 返回熔断器是否处于 Open（直通）状态（原子读，热路径无锁）。
func (b *Breaker) IsOpen() bool { return b.bypass.Load() }

// CurrentState 返回当前状态（用于 UI 展示，非热路径）。
func (b *Breaker) CurrentState() State { return State(b.state.Load()) }

// heartbeatLoop 每 50ms 发送一次心跳，维护连续丢包/恢复计数器。
func (b *Breaker) heartbeatLoop(ctx context.Context) {
	defer b.wg.Done()
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			alive := b.sendHeartbeat()
			b.handleHeartbeat(alive)
		}
	}
}

// sendHeartbeat 根据 targetAddr 格式选择心跳协议：HTTP HEAD 或 UDP ping。
// targetAddr 为空时视为存活（避免误熔断）。
func (b *Breaker) sendHeartbeat() bool {
	if b.targetAddr == "" {
		return true
	}
	if strings.HasPrefix(b.targetAddr, "http://") || strings.HasPrefix(b.targetAddr, "https://") {
		return b.pingHTTP()
	}
	return b.pingUDP()
}

// pingHTTP 向目标 URL 发送 HEAD 请求检查连通性。
// 任何 HTTP 响应（含 4xx）表示网络连通；仅网络错误或 5xx 服务端异常视为失联。
func (b *Breaker) pingHTTP() bool {
	req, err := http.NewRequest(http.MethodHead, b.targetAddr, nil)
	if err != nil {
		return false
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	// 5xx 表示服务端异常，视为失联
	return resp.StatusCode < http.StatusInternalServerError
}

// pingUDP 发送 UDP ping，等待 pong 响应。
func (b *Breaker) pingUDP() bool {
	conn, err := net.DialTimeout("udp", b.targetAddr, heartbeatDialTimeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(heartbeatDialTimeout))
	if _, err = conn.Write([]byte("ping")); err != nil {
		return false
	}
	buf := make([]byte, 16)
	_, err = conn.Read(buf)
	return err == nil
}

// handleHeartbeat 根据心跳结果更新状态机。
func (b *Breaker) handleHeartbeat(alive bool) {
	state := State(b.state.Load())
	switch state {
	case StateClosed:
		if !alive {
			b.missCount++
			if b.missCount >= tripThreshold {
				b.trip()
			}
		} else {
			b.missCount = 0
		}
	case StateOpen:
		if alive {
			b.recoverCount++
			if b.recoverCount >= recoverThreshold {
				b.halfOpen()
			}
		} else {
			b.recoverCount = 0
		}
	case StateHalfOpen:
		if alive {
			b.recoverCount++
			if b.recoverCount >= recoverThreshold {
				b.close()
			}
		} else {
			b.recoverCount = 0
			b.trip()
		}
	}
}

// trip 触发熔断（Closed/HalfOpen → Open）。
func (b *Breaker) trip() {
	prev := State(b.state.Swap(int32(StateOpen)))
	if prev == StateOpen {
		return
	}
	b.bypass.Store(true)
	b.missCount = 0
	b.recoverCount = 0
	if b.onStateChange != nil {
		b.onStateChange(StateOpen)
	}
}

// halfOpen 从 Open 进入 HalfOpen（尝试恢复阶段）。
func (b *Breaker) halfOpen() {
	b.state.Store(int32(StateHalfOpen))
	b.recoverCount = 0
}

// close 熔断恢复（HalfOpen → Closed）。
func (b *Breaker) close() {
	b.state.Store(int32(StateClosed))
	b.bypass.Store(false)
	b.missCount = 0
	b.recoverCount = 0
	if b.onStateChange != nil {
		b.onStateChange(StateClosed)
	}
}
