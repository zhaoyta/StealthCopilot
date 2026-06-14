// Package circuit 实现 StealthCopilot 的熔断器。
// 状态机：Closed（正常）→ Open（降级直通）→ HalfOpen（尝试恢复）→ Closed
// 触发条件：连续 3 次 UDP 心跳丢失（150ms）或视频 PTS 落后音频 >300ms。
// 切换延迟 ≤10ms（原子 bool 切换输出端，无锁路径）。
package circuit

import (
	"context"
	"net"
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
	heartbeatInterval    = 50 * time.Millisecond // 心跳发送间隔
	tripThreshold        = 3                     // 连续丢失触发熔断
	recoverThreshold     = 3                     // 连续恢复关闭熔断
	heartbeatDialTimeout = 20 * time.Millisecond // UDP 心跳响应超时
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
	targetAddr    string        // UDP 心跳目标地址
	onStateChange OnStateChange // 状态变化回调

	bypass atomic.Bool // true = 直通模式（Open），false = 云端模式（Closed）

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewBreaker 创建熔断器。
// targetAddr 为 UDP 心跳目标地址（Simli API 心跳端点，格式 "host:port"）；
// onStateChange 在状态变化时回调，可为 nil。
func NewBreaker(targetAddr string, onStateChange OnStateChange) *Breaker {
	b := &Breaker{
		targetAddr:    targetAddr,
		onStateChange: onStateChange,
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

// heartbeatLoop 每 50ms 发送一次 UDP 心跳，维护连续丢包/恢复计数器。
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

// sendHeartbeat 发送一次 UDP ping，返回是否收到响应。
func (b *Breaker) sendHeartbeat() bool {
	if b.targetAddr == "" {
		return true // 无目标地址时视为存活（避免空地址触发误熔断）
	}
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
			// 进入 HalfOpen：等待连续恢复确认
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
			// 恢复失败，重新打开
			b.recoverCount = 0
			b.trip()
		}
	}
}

// trip 触发熔断（Closed/HalfOpen → Open）。
func (b *Breaker) trip() {
	prev := State(b.state.Swap(int32(StateOpen)))
	if prev == StateOpen {
		return // 已经是 Open，不重复回调
	}
	b.bypass.Store(true) // ≤1ms 原子切换，热路径无锁
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
