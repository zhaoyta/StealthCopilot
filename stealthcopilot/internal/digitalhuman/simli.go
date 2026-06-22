// Package digitalhuman — simli.go 实现 Simli AI 数字人驱动。
//
// Simli AI 是纯视频驱动型数字人：说话链的 TTS PCM 音频同时写入虚拟麦克风（面试官
// 直接听到）并发往 Simli 进行唇形同步；Simli 通过 WebRTC 返回对齐的视频帧写入
// 虚拟摄像头，面试官看到的是口型同步的视频画面。
//
// 音频输入格式：PCM16 16kHz 单声道（讯飞 TTS 输出 24kHz 后在 SendAudio 内重采样）。
// 视频输出协议：WebRTC，SDP offer/answer 通过 WebSocket 交换，视频帧由 simli_video.go 解码。
package digitalhuman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/video"
)

const (
	// simliTokenEndpoint 获取 WebRTC 会话 token 的 HTTP 端点。
	simliTokenEndpoint = "https://api.simli.ai/compose/token"
	// simliWSEndpoint WebSocket 端点，用于 SDP 信令交换和音频发送。
	simliWSEndpoint = "wss://api.simli.ai/compose/webrtc/p2p"

	// simliAudioSampleRate Simli 要求的 PCM 输入采样率（Hz）。
	simliAudioSampleRate = 16000
	// ttsOutputSampleRate 讯飞声音复刻 TTS 输出采样率（Hz），需降采样后发给 Simli。
	ttsOutputSampleRate = 24000

	// simliICEGatherTimeout ICE 候选收集超时。
	simliICEGatherTimeout = 15 * time.Second
)

// SimliConfig 持有 Simli AI 数字人驱动的全部配置。
type SimliConfig struct {
	// APIKey Simli AI API Key（x-simli-api-key 请求头）。
	APIKey string
	// FaceID 数字人人脸 ID（在 Simli 控制台创建）。
	FaceID string
	// VirtualCameraWriter 接收解码后的 BGRA 视频帧并写入本机虚拟摄像头；nil 时跳过视频输出。
	VirtualCameraWriter video.VirtualCameraWriter
	// HTTPClient 可替换的 HTTP 客户端（nil 时使用默认）。
	HTTPClient *http.Client

	// --- 测试钩子（生产代码保持 nil） ---
	tokenFetcher func(ctx context.Context, apiKey, faceID string) (string, error)
	wsDialer     func(ctx context.Context, token string) (*websocket.Conn, error)
	pcFactory    func() (*webrtc.PeerConnection, error)
}

// SimliConfigReady 检查 Simli 驱动启动所需的最少配置是否已填写。
func SimliConfigReady(cfg SimliConfig) bool {
	return strings.TrimSpace(cfg.APIKey) != "" && strings.TrimSpace(cfg.FaceID) != ""
}

// SimliDriver 实现 Driver 接口，将 TTS PCM 音频发送给 Simli AI 进行唇形同步，
// 并通过 WebRTC 接收同步后的视频帧写入虚拟摄像头。
// 与 ZegoDriver（音视频均由云端生成）不同，SimliDriver 是纯视频驱动——
// TTS 音频仍由说话链直接写入虚拟麦克风，本驱动只负责视频同步。
type SimliDriver struct {
	cfg SimliConfig

	mu     sync.Mutex
	conn   *websocket.Conn
	pc     *webrtc.PeerConnection
	cancel context.CancelFunc
}

// NewSimliDriver 根据配置创建 SimliDriver 实例。
func NewSimliDriver(cfg SimliConfig) *SimliDriver {
	return &SimliDriver{cfg: cfg}
}

// SuppressDirectAudio 返回 false：Simli 仅提供视频，TTS 音频须由说话链直接写入虚拟麦克风。
func (*SimliDriver) SuppressDirectAudio() bool { return false }

// TestFetchToken 仅用于连接测试：获取一个会话 token 并立即丢弃，验证 API Key 是否有效。
func (d *SimliDriver) TestFetchToken(ctx context.Context) (string, error) {
	return d.fetchToken(ctx)
}

// Start 获取 Simli 会话 token、建立 WebSocket 连接并完成 WebRTC SDP 协商，
// 随后启动视频接收管道将解码帧写入虚拟摄像头。
// audioSink 对 Simli 无意义（Simli 不返回音频），传入 nil 即可。
func (d *SimliDriver) Start(ctx context.Context, _ func([]byte)) error {
	if !SimliConfigReady(d.cfg) {
		return errors.New("Simli 配置不完整：请配置 API Key 和 Face ID")
	}
	// 整体启动超时：token 获取 + WebSocket 握手 + ICE 收集 + SDP 交换，合计最长 30s。
	startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startCancel()
	ctx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	// 1. 获取会话 token
	token, err := d.fetchToken(startCtx)
	if err != nil {
		cancel()
		return fmt.Errorf("Simli 获取会话 token 失败：%w", err)
	}
	diag.Infof("simli token obtained face_id=%s", d.cfg.FaceID)

	// 2. 建立 WebSocket（用于 SDP 交换 + 音频发送）
	conn, err := d.dial(startCtx, token)
	if err != nil {
		cancel()
		return fmt.Errorf("Simli WebSocket 连接失败：%w", err)
	}

	// 3. 创建 WebRTC peer connection（用于接收视频）
	pc, err := d.newPeerConnection()
	if err != nil {
		_ = conn.Close()
		cancel()
		return fmt.Errorf("Simli WebRTC 初始化失败：%w", err)
	}

	// 4. 添加 audio + video recvonly transceiver：两者都必须在 CreateOffer 之前添加。
	// Simli 的 JS SDK 同时添加这两种 transceiver；缺少任何一个，SDP offer 里就缺少对应
	// m-line，服务端会返回 SERVER ERROR IN INITIALIZATION。
	for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeAudio, webrtc.RTPCodecTypeVideo} {
		if _, err := pc.AddTransceiverFromKind(kind, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			_ = conn.Close()
			_ = pc.Close()
			cancel()
			return fmt.Errorf("Simli 添加 %s transceiver 失败：%w", kind, err)
		}
	}

	// 5. 注册视频轨道处理器
	if d.cfg.VirtualCameraWriter != nil {
		cw := d.cfg.VirtualCameraWriter
		pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			if track.Kind() == webrtc.RTPCodecTypeVideo {
				diag.Infof("simli video track received codec=%s", track.Codec().MimeType)
				startSimliVideoReceive(ctx, track, cw)
			}
		})
	}

	// 6. 创建 SDP offer，等待 ICE 候选收集完毕后发送
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		_ = conn.Close()
		_ = pc.Close()
		cancel()
		return fmt.Errorf("Simli WebRTC offer 创建失败：%w", err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		_ = conn.Close()
		_ = pc.Close()
		cancel()
		return fmt.Errorf("Simli 设置本地描述失败：%w", err)
	}

	gatherDone := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherDone:
	case <-startCtx.Done():
		_ = conn.Close()
		_ = pc.Close()
		cancel()
		return errors.New("Simli 启动超时（30s）：ICE 候选收集未完成")
	}

	localDesc := pc.LocalDescription()
	offerMsg := map[string]string{"type": localDesc.Type.String(), "sdp": localDesc.SDP}
	if err := conn.WriteJSON(offerMsg); err != nil {
		_ = conn.Close()
		_ = pc.Close()
		cancel()
		return fmt.Errorf("Simli 发送 WebRTC offer 失败：%w", err)
	}
	diag.Infof("simli sdp offer sent")

	// 7. 读取 SDP answer
	// 服务端在返回 answer 之前可能先发 pong/START 等事件消息，需循环直到读到 type=answer。
	// 用 startCtx deadline 限制整体等待时间。
	if dl, ok := startCtx.Deadline(); ok {
		_ = conn.SetReadDeadline(dl)
	}
	answer, err := d.readAnswerMsg(conn)
	_ = conn.SetReadDeadline(time.Time{}) // 清除 deadline，避免影响后续 drainEvents
	if err != nil {
		_ = conn.Close()
		_ = pc.Close()
		cancel()
		diag.Errorf("simli sdp answer failed err=%v", err)
		return fmt.Errorf("Simli 读取 WebRTC answer 失败：%w", err)
	}
	if err := pc.SetRemoteDescription(answer); err != nil {
		_ = conn.Close()
		_ = pc.Close()
		cancel()
		return fmt.Errorf("Simli 设置远端描述失败：%w", err)
	}
	diag.Infof("simli sdp answer applied peer_connection=ready")

	d.mu.Lock()
	d.conn = conn
	d.pc = pc
	d.mu.Unlock()

	// 8. 启动 WebSocket 事件读取 goroutine（START/STOP/ACK 等服务端通知）
	go d.drainEvents(ctx)

	return nil
}

// SendAudio 接收 TTS 输出的 PCM（24kHz 16bit mono），重采样到 16kHz 后
// 以二进制 WebSocket 消息发送给 Simli 进行唇形同步。
func (d *SimliDriver) SendAudio(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return errors.New("Simli WebSocket 未连接")
	}
	resampled := resample24to16(chunk)
	if len(resampled) == 0 {
		return nil
	}
	return conn.WriteMessage(websocket.BinaryMessage, resampled)
}

// Close 向 Simli 发送 DONE 信号并关闭 WebSocket、WebRTC peer connection。
func (d *SimliDriver) Close() error {
	if d.cancel != nil {
		d.cancel()
	}
	d.mu.Lock()
	conn := d.conn
	pc := d.pc
	d.conn = nil
	d.pc = nil
	d.mu.Unlock()

	if conn != nil {
		// DONE 通知 Simli 音频流已结束
		_ = conn.WriteMessage(websocket.TextMessage, []byte("DONE"))
		_ = conn.Close()
	}
	if pc != nil {
		_ = pc.Close()
	}
	if d.cfg.VirtualCameraWriter != nil {
		_ = d.cfg.VirtualCameraWriter.Close()
	}
	return nil
}

// drainEvents 持续读取 WebSocket 文本消息（START/STOP/ACK/SPEAK/SILENT 等服务端事件）
// 并记录日志；连接关闭或 ctx 取消时退出。
func (d *SimliDriver) drainEvents(ctx context.Context) {
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				diag.Warnf("simli ws event read err=%v", err)
			}
			return
		}
		text := strings.TrimSpace(string(msg))
		if text != "" {
			diag.Infof("simli event=%s", text)
		}
	}
}

// ---- SDP answer 读取 ----

// readAnswerMsg 循环读取 WebSocket 消息，直到读到 SDP answer 后返回 SessionDescription。
//
// Simli 协议说明（来自官方 JS SDK BaseTransport.ts）：
//   - 所有信令消息均为 TEXT frame
//   - SDP answer 是 JSON 文本，其第一个 "word"（大写后按空格切分）包含 "SDP"
//   - 其他事件为纯文本：START/ACK/STOP/SPEAK/SILENT
//   - ERROR:/RATE:/CLOSING: 开头的文本表示服务端终止错误
//   - destination 信息也是 JSON 文本，但无 "sdp" 字段
//
// 策略：收到 TEXT frame 时，优先检查错误前缀；再尝试按 JSON 解析；
// 若解析到 type=answer 且含 sdp 字段，即为 SDP answer；否则记录日志并继续。
func (d *SimliDriver) readAnswerMsg(conn *websocket.Conn) (webrtc.SessionDescription, error) {
	for {
		msgType, raw, err := conn.ReadMessage()
		if err != nil {
			return webrtc.SessionDescription{}, err
		}

		text := strings.TrimSpace(string(raw))

		// Binary frame：Simli 信令阶段不应发送 binary，忽略。
		if msgType == websocket.BinaryMessage {
			diag.Warnf("simli unexpected binary msg len=%d during signaling, skip", len(raw))
			continue
		}

		// ERROR:/RATE:/CLOSING: 前缀 → 服务端终止错误，立即返回。
		upper := strings.ToUpper(text)
		firstWord := strings.SplitN(upper, " ", 2)[0]
		if strings.HasPrefix(firstWord, "ERROR:") || firstWord == "ERROR" ||
			strings.HasPrefix(firstWord, "RATE:") ||
			strings.HasPrefix(firstWord, "CLOSING:") {
			return webrtc.SessionDescription{}, fmt.Errorf("Simli 服务端错误：%s", text)
		}

		// 尝试解析为 JSON，检查是否为 SDP answer。
		var msg struct {
			Type string `json:"type"`
			SDP  string `json:"sdp"`
		}
		if err := json.Unmarshal(raw, &msg); err == nil && msg.Type == "answer" && msg.SDP != "" {
			return webrtc.SessionDescription{
				Type: webrtc.NewSDPType(msg.Type),
				SDP:  msg.SDP,
			}, nil
		}

		// 其余消息（START/ACK/destination JSON 等）记录日志后跳过。
		diag.Infof("simli pre-answer msg=%s", text)
	}
}

// ---- token / WebSocket / peer connection 创建 ----

type simliTokenResp struct {
	SessionToken string `json:"session_token"`
	Detail       string `json:"detail,omitempty"`
}

func (d *SimliDriver) fetchToken(ctx context.Context) (string, error) {
	if d.cfg.tokenFetcher != nil {
		return d.cfg.tokenFetcher(ctx, d.cfg.APIKey, d.cfg.FaceID)
	}
	payload, _ := json.Marshal(map[string]any{
		"faceId":           d.cfg.FaceID,
		"audioInputFormat": "pcm16",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, simliTokenEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-simli-api-key", d.cfg.APIKey)
	client := d.cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var tr simliTokenResp
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("解析 Simli token 响应失败：%w", err)
	}
	tok := strings.TrimSpace(tr.SessionToken)
	if tok == "" || tok == "FAIL TOKEN" {
		detail := strings.TrimSpace(tr.Detail)
		if detail != "" {
			return "", fmt.Errorf("Simli 返回无效 token：%s", detail)
		}
		return "", errors.New("Simli 未返回有效 session token")
	}
	return tok, nil
}

func (d *SimliDriver) dial(ctx context.Context, token string) (*websocket.Conn, error) {
	if d.cfg.wsDialer != nil {
		return d.cfg.wsDialer(ctx, token)
	}
	wsURL := simliWSEndpoint + "?session_token=" + token
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	return conn, err
}

func (d *SimliDriver) newPeerConnection() (*webrtc.PeerConnection, error) {
	if d.cfg.pcFactory != nil {
		return d.cfg.pcFactory()
	}
	return webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
}

// ---- PCM 重采样：24000 Hz → 16000 Hz（3:2 降采样，线性插值）----

// resample24to16 将 24kHz 16bit 单声道 PCM 降采样到 16kHz。
// 每 3 个输入样本（6 字节）产生 2 个输出样本（4 字节）：
//
//	output[0] = input[0]                       (位置 0.0)
//	output[1] = round((input[1]+input[2]) / 2) (位置 1.5，线性插值)
//
// 不足一个完整组（6 字节）的尾部数据被丢弃。
func resample24to16(pcm []byte) []byte {
	groups := len(pcm) / 6 // 每组 3 × 2 字节
	if groups == 0 {
		return nil
	}
	out := make([]byte, groups*4) // 每组 2 × 2 字节
	for i, j := 0, 0; i < groups*6; i, j = i+6, j+4 {
		s0 := int32(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		s1 := int32(int16(uint16(pcm[i+2]) | uint16(pcm[i+3])<<8))
		s2 := int32(int16(uint16(pcm[i+4]) | uint16(pcm[i+5])<<8))

		// 输出样本 0：直接取 s0
		o0 := int16(s0)
		out[j] = byte(uint16(o0))
		out[j+1] = byte(uint16(o0) >> 8)

		// 输出样本 1：s1 与 s2 的线性插值（四舍五入）
		avg := int16((s1 + s2 + 1) / 2)
		out[j+2] = byte(uint16(avg))
		out[j+3] = byte(uint16(avg) >> 8)
	}
	return out
}
