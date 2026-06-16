// Package translation — xunfei_speak.go 实现说话链的讯飞 RTASR + 文本翻译接入。
// 与听力链（长连接 WebSocket）不同，说话链使用"一次性 WebSocket"模式：
// VAD 触发后，将整段音频批量发送，等待最终识别结果，然后关闭连接并按需调用文本翻译。
package translation

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

const (
	speakTranslateTimeout = 15 * time.Second
	// RTASR 建议 16k/16bit/mono PCM 每 40ms 发送 1280 字节。
	speakFrameBytes   = 1280
	speakSendInterval = 40 * time.Millisecond
)

// XunfeiSpeakConfig 说话链讯飞 RTASR 配置（与听力链复用相同 AppID/Key）。
type XunfeiSpeakConfig = XunfeiConfig

// XunfeiSpeakProvider 将 VAD 捕获的完整 PCM 音频通过讯飞 RTASR 识别，再按需文本翻译。
// 每次调用独立建立和关闭 WebSocket 连接（一次性模式，区别于听力链的长连接模式）。
type XunfeiSpeakProvider struct {
	cfg        XunfeiSpeakConfig
	translator TextTranslator
}

// NewXunfeiSpeakProvider 创建说话链讯飞 RTASR Provider。
func NewXunfeiSpeakProvider(cfg XunfeiSpeakConfig, translators ...TextTranslator) *XunfeiSpeakProvider {
	var translator TextTranslator
	if len(translators) > 0 {
		translator = translators[0]
	}
	return &XunfeiSpeakProvider{cfg: cfg, translator: translator}
}

// Translate 将整段 PCM 音频（pcmData）发送给讯飞 RTASR，并返回目标语言文本。
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

	// RTASR 需要近实时上传：将 pcmData 按 40ms 切片逐帧发送。
	if err := p.sendAllFrames(conn, pcmData); err != nil {
		return "", fmt.Errorf("xunfei_speak: send frames: %w", err)
	}

	// 等待最终识别结果。
	_ = conn.SetReadDeadline(time.Now().Add(speakTranslateTimeout))
	sourceText, err := p.waitFinalResult(conn)
	if err != nil {
		return "", err
	}
	if p.translator == nil || p.cfg.SourceLang == p.cfg.TargetLang {
		return sourceText, nil
	}
	translated, err := p.translator.TranslateText(ctx, sourceText, p.cfg.SourceLang, p.cfg.TargetLang)
	if err != nil {
		return "", fmt.Errorf("xunfei_speak: translate text: %w", err)
	}
	return translated, nil
}

// sendAllFrames 将整段 PCM 数据按帧切割，依次发送给讯飞 RTASR WebSocket。
// 发送顺序：PCM binary frames → {"end": true} binary message。
func (p *XunfeiSpeakProvider) sendAllFrames(conn *websocket.Conn, pcmData []byte) error {
	for offset := 0; offset < len(pcmData); offset += speakFrameBytes {
		end := offset + speakFrameBytes
		if end > len(pcmData) {
			end = len(pcmData)
		}
		frame := pcmData[offset:end]

		if err := writeXunfeiAudio(conn, frame); err != nil {
			return err
		}
		time.Sleep(speakSendInterval)
	}
	return writeXunfeiEnd(conn)
}

// waitFinalResult 循环读取 WebSocket 响应，返回第一个最终 dst 文本。
// 连接关闭或读取超时时返回错误（触发降级）。
func (p *XunfeiSpeakProvider) waitFinalResult(conn *websocket.Conn) (string, error) {
	lastText := ""
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if lastText != "" {
				return lastText, nil
			}
			return "", fmt.Errorf("xunfei_speak: read: %w", err)
		}
		result, ok := parseXunfeiResponse(data)
		if !ok {
			continue
		}
		text := result.DstText
		if text == "" {
			text = result.SrcText
		}
		if text != "" {
			lastText = text
		}
		if result.IsFinal && text != "" {
			return text, nil
		}
	}
}

// ErrTranslateTimeout 表示讯飞说话链识别/翻译超时。
// 调用方收到此错误后应停止 Zero-PCM，恢复真实麦克风直通。
var ErrTranslateTimeout = fmt.Errorf("xunfei_speak: translate timeout")
