// Package translation — xunfei_speak.go 实现说话链的讯飞语音翻译接入。
// 与听力链（长连接 WebSocket）不同，说话链使用"一次性 WebSocket"模式：
// VAD 触发后，将整段音频批量发送，等待最终翻译结果，然后关闭连接。
// 超时 2s 时取消请求，触发降级（调用方停止 Zero-PCM，恢复真实麦克风直通）。
package translation

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

const (
	speakTranslateTimeout = 2 * time.Second
	// speakMaxSendInterval 批量发送时相邻帧之间的最小间隔，避免 WebSocket 接收端溢出
	speakMaxSendInterval = 5 * time.Millisecond
)

// XunfeiSpeakConfig 说话链讯飞翻译配置（与听力链复用相同 AppID/Key）。
type XunfeiSpeakConfig = XunfeiConfig

// XunfeiSpeakProvider 将 VAD 捕获的完整 PCM 音频通过讯飞 WebSocket 翻译 API 获取目标语言文本。
// 每次调用独立建立和关闭 WebSocket 连接（一次性模式，区别于听力链的长连接模式）。
type XunfeiSpeakProvider struct {
	cfg XunfeiSpeakConfig
}

// NewXunfeiSpeakProvider 创建说话链讯飞翻译 Provider。
func NewXunfeiSpeakProvider(cfg XunfeiSpeakConfig) *XunfeiSpeakProvider {
	return &XunfeiSpeakProvider{cfg: cfg}
}

// Translate 将整段 PCM 音频（pcmData）发送给讯飞实时语音翻译 API，返回目标语言文本。
// 超过 speakTranslateTimeout 时返回 ErrTranslateTimeout（调用方降级处理）。
// 内部使用与听力链相同的 WebSocket 认证和协议，但以批量模式一次性发完所有帧。
func (p *XunfeiSpeakProvider) Translate(ctx context.Context, pcmData []byte) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, speakTranslateTimeout)
	defer cancel()

	authURL, err := (&XunfeiTranslationProvider{cfg: p.cfg}).buildAuthURL()
	if err != nil {
		return "", fmt.Errorf("xunfei_speak: build auth URL: %w", err)
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(timeoutCtx, authURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return "", fmt.Errorf("xunfei_speak: dial: %w", err)
	}
	defer conn.Close()

	// 设置 WebSocket 写超时，确保整个过程不超过 speakTranslateTimeout
	_ = conn.SetWriteDeadline(time.Now().Add(speakTranslateTimeout))

	// 批量发送：将 pcmData 按 FrameBytes 切片逐帧发送
	if err := p.sendAllFrames(conn, pcmData); err != nil {
		return "", fmt.Errorf("xunfei_speak: send frames: %w", err)
	}

	// 等待最终翻译结果（is_end=1）
	_ = conn.SetReadDeadline(time.Now().Add(speakTranslateTimeout))
	return p.waitFinalResult(conn)
}

// sendAllFrames 将整段 PCM 数据按帧切割，依次发送给讯飞 WebSocket。
// 发送顺序：第一帧（携带 common+business 参数）→ 中间帧 → 最后帧（空 audio，status=2）。
func (p *XunfeiSpeakProvider) sendAllFrames(conn *websocket.Conn, pcmData []byte) error {
	inner := &XunfeiTranslationProvider{cfg: p.cfg}
	frameSize := 1280 // 16kHz 16bit 40ms = 1280 bytes（与 audio.FrameBytes 一致）

	first := true
	for offset := 0; offset < len(pcmData); offset += frameSize {
		end := offset + frameSize
		if end > len(pcmData) {
			end = len(pcmData)
		}
		frame := pcmData[offset:end]

		var msg any
		if first {
			msg = inner.buildFirstFrame(frame)
			first = false
		} else {
			msg = buildContFrame(frame)
		}
		if err := conn.WriteJSON(msg); err != nil {
			return err
		}
		time.Sleep(speakMaxSendInterval) // 批量模式轻微限速，避免服务端缓冲溢出
	}
	// 发送结束帧
	return conn.WriteJSON(buildLastFrame())
}

// waitFinalResult 循环读取 WebSocket 响应，返回第一个 is_end=1 的 dst 文本。
// 连接关闭或读取超时时返回错误（触发降级）。
func (p *XunfeiSpeakProvider) waitFinalResult(conn *websocket.Conn) (string, error) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("xunfei_speak: read: %w", err)
		}
		result, ok := parseXunfeiResponse(data)
		if !ok {
			continue
		}
		if result.IsFinal && result.DstText != "" {
			return result.DstText, nil
		}
	}
}

// ErrTranslateTimeout 表示讯飞语音翻译超时（2s）。
// 调用方收到此错误后应停止 Zero-PCM，恢复真实麦克风直通。
var ErrTranslateTimeout = fmt.Errorf("xunfei_speak: translate timeout (2s)")
