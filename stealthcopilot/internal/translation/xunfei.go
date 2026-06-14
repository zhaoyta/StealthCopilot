// Package translation 实现讯飞实时语音翻译 WebSocket 接入。
// 认证方式：HMAC-SHA256 签名 URL（讯飞 WebSocket 鉴权标准方案）。
// 单次 WebSocket 连接同时返回 src_text（源语言）和 dst_text（目标语言）双路输出。
package translation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	xunfeiHost = "itrans.xf-yun.com"
	xunfeiPath = "/v1/its"
	xunfeiWSS  = "wss://" + xunfeiHost + xunfeiPath
)

// 音频帧状态常量（讯飞协议）
const (
	frameStatusFirst = 0 // 第一帧，携带 common + business 参数
	frameStatusCont  = 1 // 中间帧，仅携带音频数据
	frameStatusLast  = 2 // 最后帧，audio 为空，通知服务端结束识别
)

// 重连策略常量
const (
	maxRetries    = 3
	retryBaseWait = time.Second // 指数退避基准：1s、2s、4s
)

// XunfeiConfig 讯飞 API 连接配置，由 config.AppConfig 注入。
type XunfeiConfig struct {
	AppID      string // 讯飞控制台 AppID
	APIKey     string // 讯飞控制台 APIKey
	APISecret  string // 讯飞控制台 APISecret
	SourceLang string // 源语言，如 "en"
	TargetLang string // 目标语言，如 "zh"
}

// XunfeiTranslationProvider 实现 Provider 接口，接入讯飞实时语音翻译 WebSocket API。
// 每次 Translate 调用维护一个 WebSocket 长连接，断连时自动指数退避重连。
type XunfeiTranslationProvider struct {
	cfg XunfeiConfig
}

// NewXunfeiProvider 创建讯飞翻译 Provider。
func NewXunfeiProvider(cfg XunfeiConfig) *XunfeiTranslationProvider {
	return &XunfeiTranslationProvider{cfg: cfg}
}

// Translate 启动讯飞 WebSocket 连接，从 audioStream 读取 PCM 帧发送，返回 DualResult channel。
// 断连时以指数退避最多重连 maxRetries 次；ctx 取消时关闭所有 goroutine 和 channel。
func (p *XunfeiTranslationProvider) Translate(
	ctx context.Context, audioStream <-chan []byte,
) (<-chan DualResult, error) {
	out := make(chan DualResult, 32)
	go p.run(ctx, audioStream, out)
	return out, nil
}

// Close 无需额外资源释放（连接生命周期由 ctx 控制）。
func (p *XunfeiTranslationProvider) Close() error { return nil }

// run 管理 WebSocket 连接生命周期，是 Translate 的主 goroutine。
// 连接失败时指数退避重连，超过最大次数后关闭 out channel 并退出。
func (p *XunfeiTranslationProvider) run(
	ctx context.Context, audioStream <-chan []byte, out chan<- DualResult,
) {
	defer close(out)
	for retries := 0; ; retries++ {
		if ctx.Err() != nil {
			return
		}
		_ = p.session(ctx, audioStream, out)
		if ctx.Err() != nil {
			return
		}
		if retries >= maxRetries {
			return
		}
		// 指数退避：1s、2s、4s
		wait := retryBaseWait * (1 << uint(retries))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// session 建立一次 WebSocket 连接，发送音频并接收翻译结果直到出错或 ctx 取消。
// 返回非 nil error 表示需要重连。
func (p *XunfeiTranslationProvider) session(
	ctx context.Context, audioStream <-chan []byte, out chan<- DualResult,
) error {
	authURL, err := p.buildAuthURL()
	if err != nil {
		return fmt.Errorf("xunfei: build auth URL: %w", err)
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, authURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("xunfei: dial: %w", err)
	}
	defer conn.Close()

	// 接收 goroutine：WebSocket 消息 → out channel
	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		p.receiveLoop(conn, out)
	}()

	// 发送 goroutine 在当前 goroutine 中运行（保持 sendLoop 阻塞直到 ctx 取消或 audioStream 关闭）
	p.sendLoop(ctx, conn, audioStream)

	// 发送结束帧，通知服务端识别完毕
	_ = conn.WriteJSON(buildLastFrame())

	// 等待服务端关闭连接（最多 3s），确保最后的文本结果被接收
	select {
	case <-recvDone:
	case <-time.After(3 * time.Second):
	}
	return nil
}

// sendLoop 从 audioStream 读取 PCM 帧，以 JSON 消息发送给讯飞 WebSocket。
// 第一帧携带 common + business 参数，后续帧只发送音频数据。
func (p *XunfeiTranslationProvider) sendLoop(
	ctx context.Context, conn *websocket.Conn, audioStream <-chan []byte,
) {
	first := true
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-audioStream:
			if !ok {
				return
			}
			var msg any
			if first {
				msg = p.buildFirstFrame(frame)
				first = false
			} else {
				msg = buildContFrame(frame)
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}

// receiveLoop 持续读取 WebSocket 消息，解析后写入 out channel。
// 写入失败时丢弃（下游慢时不阻塞读取，保持实时性）。
func (p *XunfeiTranslationProvider) receiveLoop(conn *websocket.Conn, out chan<- DualResult) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		result, ok := parseXunfeiResponse(data)
		if !ok {
			continue
		}
		select {
		case out <- result:
		default:
		}
	}
}

// --- 消息构造 ---

// xunfeiFirstMsg 是第一帧消息，携带鉴权参数和业务参数。
type xunfeiFirstMsg struct {
	Common struct {
		AppID string `json:"app_id"`
	} `json:"common"`
	Business struct {
		From string `json:"from"`
		To   string `json:"to"`
		Ptt  int    `json:"ptt"` // 标点符号：1=保留
	} `json:"business"`
	Data xunfeiAudioData `json:"data"`
}

// xunfeiContMsg 是中间帧和结束帧消息。
type xunfeiContMsg struct {
	Data xunfeiAudioData `json:"data"`
}

// xunfeiAudioData 携带音频分片（base64 编码的 PCM）。
type xunfeiAudioData struct {
	Status   int    `json:"status"`
	Audio    string `json:"audio"`    // base64(PCM raw)
	Encoding string `json:"encoding"` // "raw"
}

func (p *XunfeiTranslationProvider) buildFirstFrame(pcm []byte) xunfeiFirstMsg {
	msg := xunfeiFirstMsg{}
	msg.Common.AppID = p.cfg.AppID
	msg.Business.From = p.cfg.SourceLang
	msg.Business.To = p.cfg.TargetLang
	msg.Business.Ptt = 1
	msg.Data.Status = frameStatusFirst
	msg.Data.Audio = base64.StdEncoding.EncodeToString(pcm)
	msg.Data.Encoding = "raw"
	return msg
}

func buildContFrame(pcm []byte) xunfeiContMsg {
	return xunfeiContMsg{Data: xunfeiAudioData{
		Status:   frameStatusCont,
		Audio:    base64.StdEncoding.EncodeToString(pcm),
		Encoding: "raw",
	}}
}

func buildLastFrame() xunfeiContMsg {
	return xunfeiContMsg{Data: xunfeiAudioData{Status: frameStatusLast}}
}

// --- 响应解析 ---

// xunfeiResponse 是讯飞实时语音翻译 API 响应的 JSON 结构。
// 字段名基于讯飞 WebSocket 实时语音翻译 API v2（实际字段需以 API 返回为准）。
type xunfeiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Sid     string `json:"sid"`
	Data    struct {
		Status int `json:"status"`
		Result struct {
			Src   string `json:"src"`    // 源语言原文（面试官语言）
			Dst   string `json:"dst"`    // 目标语言译文（用户语言）
			IsEnd int    `json:"is_end"` // 1=当前句子已完整（触发意图识别）
		} `json:"result"`
	} `json:"data"`
}

// parseXunfeiResponse 解析讯飞 WebSocket 响应，转换为 DualResult。
// code != 0 或内容为空时返回 false（跳过该消息）。
func parseXunfeiResponse(data []byte) (DualResult, bool) {
	var resp xunfeiResponse
	if err := json.Unmarshal(data, &resp); err != nil || resp.Code != 0 {
		return DualResult{}, false
	}
	res := resp.Data.Result
	if res.Src == "" && res.Dst == "" {
		return DualResult{}, false
	}
	return DualResult{
		SrcText: res.Src,
		DstText: res.Dst,
		IsFinal: res.IsEnd == 1,
	}, true
}

// --- URL 鉴权 ---

// buildAuthURL 生成带 HMAC-SHA256 签名的讯飞 WebSocket URL。
// 签名算法：
//  1. signatureOrigin = "host: {host}\ndate: {RFC1123}\nGET {path} HTTP/1.1"
//  2. signature = base64(HMAC-SHA256(signatureOrigin, apiSecret))
//  3. authorizationOrigin = `api_key="...", algorithm="...", headers="...", signature="..."`
//  4. authorization = base64(authorizationOrigin)
func (p *XunfeiTranslationProvider) buildAuthURL() (string, error) {
	date := time.Now().UTC().Format(http.TimeFormat) // RFC1123
	signatureOrigin := strings.Join([]string{
		"host: " + xunfeiHost,
		"date: " + date,
		"GET " + xunfeiPath + " HTTP/1.1",
	}, "\n")

	mac := hmac.New(sha256.New, []byte(p.cfg.APISecret))
	mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	authOrigin := fmt.Sprintf(
		`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		p.cfg.APIKey, signature,
	)
	authorization := base64.StdEncoding.EncodeToString([]byte(authOrigin))

	params := url.Values{}
	params.Set("authorization", authorization)
	params.Set("date", date)
	params.Set("host", xunfeiHost)
	return xunfeiWSS + "?" + params.Encode(), nil
}
