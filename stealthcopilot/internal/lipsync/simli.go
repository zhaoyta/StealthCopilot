// Package lipsync — simli.go 实现 Simli AI 实时口型同步 Provider。
// 协议：WebSocket 流式双向传输；输入端发送 PCM 音频 chunk + 时间戳，
// 输出端接收口型同步后的 JPEG 视频帧（携带对应 PTS），Go 后端解码写入 A/V 环形缓冲区。
// 连接断开时以指数退避自动重连（最多 3 次）。
package lipsync

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Simli API 常量
const (
	simliWSBaseURL     = "wss://api.simli.ai/startAudioToVideoSession"
	simliMaxRetries    = 3
	simliBaseBackoff   = 1 * time.Second
	simliMaxBackoff    = 8 * time.Second
	simliOutputChanCap = 64
)

// SimliConfig Simli AI API 连接配置。
type SimliConfig struct {
	APIKey string // Simli API Key
	FaceID string // 用户配置的 Face ID
}

// simliInitMsg 建立会话的初始化消息（发送给 Simli WebSocket）。
type simliInitMsg struct {
	APIKey    string `json:"apiKey"`
	FaceID    string `json:"faceId"`
	SyncAudio bool   `json:"syncAudio"`
}

// simliAudioMsg 音频帧消息（二进制帧；type 字段区分消息类型）。
// 实际传输为二进制帧：[4字节 PTS int64 big-endian][PCM 数据]
// 本结构仅用于注释说明；实际序列化见 marshalAudioFrame。

// SimliProvider 实现 Provider 接口，通过 Simli AI WebSocket API 做实时口型同步。
type SimliProvider struct {
	cfg    SimliConfig
	mu     sync.Mutex
	conn   *websocket.Conn
	output chan VideoFrame
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSimliProvider 创建 SimliProvider 实例（未建立连接）。
func NewSimliProvider(cfg SimliConfig) *SimliProvider {
	return &SimliProvider{
		cfg:    cfg,
		output: make(chan VideoFrame, simliOutputChanCap),
	}
}

// Start 建立 Simli WebSocket 会话，启动接收 goroutine。
// 若 API Key 或 Face ID 未配置则立即返回错误（降级为无口型同步模式）。
func (p *SimliProvider) Start(ctx context.Context, faceID string) error {
	if p.cfg.APIKey == "" || faceID == "" {
		return fmt.Errorf("simli: APIKey 或 FaceID 未配置")
	}
	if faceID != "" {
		p.cfg.FaceID = faceID
	}

	ctx2, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	conn, err := p.connectWithRetry(ctx2)
	if err != nil {
		cancel()
		return fmt.Errorf("simli: 连接失败: %w", err)
	}

	p.mu.Lock()
	p.conn = conn
	p.mu.Unlock()

	p.wg.Add(1)
	go p.receiveLoop(ctx2, conn)

	return nil
}

// connectWithRetry 以指数退避重连 Simli WebSocket（最多 simliMaxRetries 次）。
func (p *SimliProvider) connectWithRetry(ctx context.Context) (*websocket.Conn, error) {
	var lastErr error
	backoff := simliBaseBackoff

	for attempt := 0; attempt <= simliMaxRetries; attempt++ {
		if attempt > 0 {
			// 加随机抖动避免同时重连
			jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + jitter):
			}
			backoff *= 2
			if backoff > simliMaxBackoff {
				backoff = simliMaxBackoff
			}
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, simliWSBaseURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		// 发送初始化消息
		initMsg := simliInitMsg{
			APIKey:    p.cfg.APIKey,
			FaceID:    p.cfg.FaceID,
			SyncAudio: true,
		}
		data, _ := json.Marshal(initMsg)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			_ = conn.Close()
			lastErr = err
			continue
		}
		return conn, nil
	}
	return nil, fmt.Errorf("simli: 已达最大重试次数 (%d): %w", simliMaxRetries, lastErr)
}

// SendAudio 向 Simli 发送一段 PCM 音频 chunk（二进制帧：8字节 PTS + PCM 数据）。
func (p *SimliProvider) SendAudio(chunk AudioChunk) error {
	p.mu.Lock()
	conn := p.conn
	p.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("simli: 连接未建立")
	}
	payload := marshalAudioFrame(chunk.PTS, chunk.Data)
	return conn.WriteMessage(websocket.BinaryMessage, payload)
}

// SendVideo 向 Simli 发送原始视频帧（此接口在 Simli API v1 中可选；口型同步依赖音频驱动）。
// Simli 通过 Face ID 加载预置面部模型，实际上不需要实时传入视频帧；此方法为接口占位。
func (p *SimliProvider) SendVideo(_ VideoFrame) error {
	return nil // Simli v1：face_id 指定预置模型，无需传入原始视频帧
}

// Output 返回口型同步后的视频帧 channel（接收方可直接写入虚拟摄像头）。
func (p *SimliProvider) Output() <-chan VideoFrame {
	return p.output
}

// Close 关闭 WebSocket 连接，等待接收 goroutine 退出。
func (p *SimliProvider) Close() error {
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Lock()
	conn := p.conn
	p.conn = nil
	p.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	p.wg.Wait()
	return nil
}

// receiveLoop 持续接收 Simli 返回的视频帧，解析后写入 output channel。
// 连接断开时尝试自动重连（指数退避）；ctx 取消时退出。
func (p *SimliProvider) receiveLoop(ctx context.Context, conn *websocket.Conn) {
	defer p.wg.Done()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			// 连接断开：尝试重连
			select {
			case <-ctx.Done():
				return
			default:
			}
			newConn, retryErr := p.connectWithRetry(ctx)
			if retryErr != nil {
				// 重连失败：关闭 output，触发上层熔断
				close(p.output)
				return
			}
			p.mu.Lock()
			p.conn = newConn
			conn = newConn
			p.mu.Unlock()
			continue
		}

		frame, parseErr := parseSimliFrame(data)
		if parseErr != nil {
			continue // 忽略无法解析的帧
		}

		select {
		case p.output <- frame:
		case <-ctx.Done():
			return
		default:
			// output 满时丢帧（下游 ring buffer 有溢出保护）
		}
	}
}

// NullLipSyncProvider 是口型同步不可用时的空实现（Simli 未配置时降级）。
// 将原始视频帧直接转发到输出 channel，不做口型处理。
type NullLipSyncProvider struct {
	output chan VideoFrame
	once   sync.Once
}

// NewNullLipSyncProvider 创建 NullLipSyncProvider。
func NewNullLipSyncProvider() *NullLipSyncProvider {
	return &NullLipSyncProvider{output: make(chan VideoFrame, simliOutputChanCap)}
}

// Start 无需操作（无需连接）。
func (n *NullLipSyncProvider) Start(_ context.Context, _ string) error { return nil }

// SendAudio 丢弃音频输入（无口型处理）。
func (n *NullLipSyncProvider) SendAudio(_ AudioChunk) error { return nil }

// SendVideo 将原始视频帧直接转发到输出 channel（passthrough 模式）。
func (n *NullLipSyncProvider) SendVideo(frame VideoFrame) error {
	select {
	case n.output <- frame:
	default:
	}
	return nil
}

// Output 返回视频帧 channel（原始帧直通）。
func (n *NullLipSyncProvider) Output() <-chan VideoFrame { return n.output }

// Close 关闭输出 channel。
func (n *NullLipSyncProvider) Close() error {
	n.once.Do(func() { close(n.output) })
	return nil
}

// marshalAudioFrame 将 PTS 和 PCM 数据打包为二进制帧（8字节 PTS big-endian + PCM）。
func marshalAudioFrame(pts int64, pcm []byte) []byte {
	buf := make([]byte, 8+len(pcm))
	buf[0] = byte(pts >> 56)
	buf[1] = byte(pts >> 48)
	buf[2] = byte(pts >> 40)
	buf[3] = byte(pts >> 32)
	buf[4] = byte(pts >> 24)
	buf[5] = byte(pts >> 16)
	buf[6] = byte(pts >> 8)
	buf[7] = byte(pts)
	copy(buf[8:], pcm)
	return buf
}

// parseSimliFrame 解析 Simli 返回的视频帧二进制消息。
// 格式：8字节 PTS big-endian + JPEG 数据。
func parseSimliFrame(data []byte) (VideoFrame, error) {
	if len(data) < 9 {
		return VideoFrame{}, fmt.Errorf("simli: frame too short (%d bytes)", len(data))
	}
	pts := int64(data[0])<<56 | int64(data[1])<<48 | int64(data[2])<<40 | int64(data[3])<<32 |
		int64(data[4])<<24 | int64(data[5])<<16 | int64(data[6])<<8 | int64(data[7])
	jpeg := make([]byte, len(data)-8)
	copy(jpeg, data[8:])
	return VideoFrame{Data: jpeg, PTS: pts}, nil
}
