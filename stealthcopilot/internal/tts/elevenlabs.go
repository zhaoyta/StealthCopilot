// Package tts 实现 TTS（Text-to-Speech）服务接入。
// 当前实现：ElevenLabs 流式 TTS，使用用户克隆音色，输出 PCM 44100Hz。
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	elevenLabsBaseURL = "https://api.elevenlabs.io/v1"
	// outputFormat 指定 PCM 44100Hz 格式，与虚拟麦克风写入采样率一致
	outputFormat = "pcm_44100"
	// chunkSize 每次从 HTTP 响应体读取的字节数
	chunkSize         = 4096
	elevenLabsTimeout = 30 * time.Second
)

// VirtualMicSampleRate 是虚拟麦克风和 ElevenLabs 输出共用的采样率（Hz）。
const VirtualMicSampleRate = 44100

// ElevenLabsConfig ElevenLabs TTS 连接配置。
type ElevenLabsConfig struct {
	APIKey  string // ElevenLabs API Key
	VoiceID string // 用户克隆的 Voice ID
	ModelID string // TTS 模型（默认 eleven_multilingual_v2）
}

// ElevenLabsProvider 实现 Provider 接口，调用 ElevenLabs 流式 TTS API。
// 使用 PCM 44100Hz 输出格式，直接对接虚拟麦克风写入。
type ElevenLabsProvider struct {
	cfg    ElevenLabsConfig
	client *http.Client
}

// NewElevenLabsProvider 创建 ElevenLabs TTS Provider。
func NewElevenLabsProvider(cfg ElevenLabsConfig) *ElevenLabsProvider {
	if cfg.ModelID == "" {
		cfg.ModelID = "eleven_multilingual_v2"
	}
	return &ElevenLabsProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: elevenLabsTimeout},
	}
}

// Synthesize 调用 ElevenLabs 流式 TTS API，返回 PCM 音频 chunk channel。
// 首帧约 400-500ms 后到达，之后持续流式输出直到合成完毕。
// channel 关闭时表示合成结束（成功或 ctx 取消）。
func (p *ElevenLabsProvider) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	if p.cfg.APIKey == "" || p.cfg.VoiceID == "" {
		return nil, fmt.Errorf("elevenlabs: API Key 或 Voice ID 未配置")
	}

	url := fmt.Sprintf("%s/text-to-speech/%s/stream?output_format=%s",
		elevenLabsBaseURL, p.cfg.VoiceID, outputFormat)

	reqBody := elevenLabsRequest{
		Text:    text,
		ModelID: p.cfg.ModelID,
		VoiceSettings: voiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.75,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("xi-api-key", p.cfg.APIKey)
	httpReq.Header.Set("Accept", "audio/pcm")

	//nolint:bodyclose // The streaming body is consumed and closed by the goroutine below.
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("elevenlabs: status %d", resp.StatusCode)
	}

	ch := make(chan []byte, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		buf := make([]byte, chunkSize)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				select {
				case ch <- chunk:
				case <-ctx.Done():
					return
				}
			}
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return ch, nil
}

// VoiceID 返回当前配置的 Voice ID（用于日志和校验）。
func (p *ElevenLabsProvider) VoiceID() string { return p.cfg.VoiceID }

// Close 无需额外操作（HTTP client 无长连接）。
func (p *ElevenLabsProvider) Close() error { return nil }

// NullTTSProvider 是 TTS 不可用时的空实现，不输出任何音频。
// 用于 ElevenLabs 未配置时的降级运行。
type NullTTSProvider struct{}

// Synthesize 返回立即关闭的空 channel（无音频输出）。
func (n *NullTTSProvider) Synthesize(_ context.Context, _ string) (<-chan []byte, error) {
	ch := make(chan []byte)
	close(ch)
	return ch, nil
}

// VoiceID 返回空字符串（Null 实现无 Voice ID）。
func (n *NullTTSProvider) VoiceID() string { return "" }

// Close 无需操作。
func (n *NullTTSProvider) Close() error { return nil }

// --- ElevenLabs API 数据结构 ---

type elevenLabsRequest struct {
	Text          string        `json:"text"`
	ModelID       string        `json:"model_id"`
	VoiceSettings voiceSettings `json:"voice_settings"`
}

type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
}
