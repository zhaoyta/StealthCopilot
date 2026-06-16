// Package translation 实现讯飞实时语音转写 RTASR WebSocket 接入。
// RTASR 在 WebSocket URL 中完成签名鉴权，连接后直接发送 16k/16bit/mono PCM binary。
// 为控制成本，本实现只启用转写，不启用 RTASR 的实时翻译高级功能。
package translation

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	xunfeiHost = "rtasr.xfyun.cn"
	xunfeiPath = "/v1/ws"
	xunfeiWSS  = "wss://" + xunfeiHost + xunfeiPath
)

// 重连策略常量
const (
	maxRetries    = 3
	retryBaseWait = time.Second // 指数退避基准：1s、2s、4s
)

// XunfeiConfig 讯飞 API 连接配置，由 config.AppConfig 注入。
type XunfeiConfig struct {
	AppID      string // 讯飞控制台 AppID
	APIKey     string // RTASR APIKey，用于 signa 生成
	SourceLang string // 源语言，如 "cn"、"en"
	TargetLang string // 目标语言，如 "en"、"cn"
}

// XunfeiTranslationProvider 实现 Provider 接口，接入讯飞 RTASR WebSocket API。
// 每次 Translate 调用维护一个 WebSocket 长连接，断连时自动指数退避重连。
type XunfeiTranslationProvider struct {
	cfg XunfeiConfig
}

// NewXunfeiProvider 创建讯飞 RTASR Provider。
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

// ProbeXunfeiRTASRConnection verifies RTASR credentials with a WebSocket handshake.
// It closes immediately after the handshake and does not send audio frames.
func ProbeXunfeiRTASRConnection(ctx context.Context, cfg XunfeiConfig) error {
	if !XunfeiConfigReady(cfg) {
		return fmt.Errorf("xunfei_rtasr: incomplete config")
	}
	authURL, err := (&XunfeiTranslationProvider{cfg: cfg}).buildAuthURL()
	if err != nil {
		return fmt.Errorf("xunfei_rtasr: build auth URL: %w", err)
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, authURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("xunfei_rtasr: dial: %w", err)
	}
	return conn.Close()
}

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

	// 发送结束标识，通知服务端识别完毕。
	_ = writeXunfeiEnd(conn)

	// 等待服务端关闭连接（最多 3s），确保最后的文本结果被接收
	select {
	case <-recvDone:
	case <-time.After(3 * time.Second):
	}
	return nil
}

// sendLoop 从 audioStream 读取 PCM 帧，以 binary message 发送给讯飞 RTASR WebSocket。
func (p *XunfeiTranslationProvider) sendLoop(
	ctx context.Context, conn *websocket.Conn, audioStream <-chan []byte,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-audioStream:
			if !ok {
				return
			}
			if err := writeXunfeiAudio(conn, frame); err != nil {
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

func writeXunfeiAudio(conn *websocket.Conn, pcm []byte) error {
	return conn.WriteMessage(websocket.BinaryMessage, pcm)
}

func writeXunfeiEnd(conn *websocket.Conn) error {
	return conn.WriteMessage(websocket.BinaryMessage, []byte(`{"end": true}`))
}

// --- 响应解析 ---

// xunfeiResponse 是讯飞 RTASR 外层响应。data 为普通转写 JSON 字符串。
// 兼容解析开启翻译时的 {"biz":"trans","src":"...","dst":"..."}，但默认不会请求该能力。
type xunfeiResponse struct {
	Action string          `json:"action"`
	Code   json.RawMessage `json:"code"`
	Data   string          `json:"data"`
	Desc   string          `json:"desc"`
	Sid    string          `json:"sid"`
}

type xunfeiTransData struct {
	Biz   string `json:"biz"`
	Src   string `json:"src"`
	Dst   string `json:"dst"`
	IsEnd bool   `json:"isEnd"`
	Type  int    `json:"type"`
}

type xunfeiASRData struct {
	CN struct {
		ST struct {
			Type string `json:"type"`
			RT   []struct {
				WS []struct {
					CW []struct {
						W string `json:"w"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"rt"`
		} `json:"st"`
	} `json:"cn"`
}

// parseXunfeiResponse 解析讯飞 WebSocket 响应，转换为 DualResult。
// code != 0 或内容为空时返回 false（跳过该消息）。
func parseXunfeiResponse(data []byte) (DualResult, bool) {
	var resp xunfeiResponse
	if err := json.Unmarshal(data, &resp); err != nil || !xunfeiCodeOK(resp.Code) {
		return DualResult{}, false
	}
	if resp.Action != "" && resp.Action != "result" {
		return DualResult{}, false
	}
	if resp.Data == "" {
		return DualResult{}, false
	}
	if result, ok := parseXunfeiTransData([]byte(resp.Data)); ok {
		return result, true
	}
	return parseXunfeiASRData([]byte(resp.Data))
}

func xunfeiCodeOK(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s == "0"
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n == 0
	}
	return false
}

func parseXunfeiTransData(data []byte) (DualResult, bool) {
	var trans xunfeiTransData
	if err := json.Unmarshal(data, &trans); err != nil || trans.Biz != "trans" {
		return DualResult{}, false
	}
	if trans.Src == "" && trans.Dst == "" {
		return DualResult{}, false
	}
	dst := trans.Dst
	if dst == "" {
		dst = trans.Src
	}
	return DualResult{SrcText: trans.Src, DstText: dst, IsFinal: trans.IsEnd}, true
}

func parseXunfeiASRData(data []byte) (DualResult, bool) {
	var asr xunfeiASRData
	if err := json.Unmarshal(data, &asr); err != nil {
		return DualResult{}, false
	}
	var b strings.Builder
	for _, rt := range asr.CN.ST.RT {
		for _, ws := range rt.WS {
			if len(ws.CW) > 0 {
				b.WriteString(ws.CW[0].W)
			}
		}
	}
	text := b.String()
	if text == "" {
		return DualResult{}, false
	}
	return DualResult{SrcText: text, DstText: text, IsFinal: asr.CN.ST.Type == "0"}, true
}

// --- URL 鉴权 ---

// buildAuthURL 生成讯飞 RTASR WebSocket URL。
// 签名算法：signa = base64(HMAC-SHA1(MD5(appid + ts), api_key))。
func (p *XunfeiTranslationProvider) buildAuthURL() (string, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	signa := buildXunfeiSigna(p.cfg.AppID, p.cfg.APIKey, ts)
	params := url.Values{}
	params.Set("appid", p.cfg.AppID)
	params.Set("ts", ts)
	params.Set("signa", signa)
	if p.cfg.SourceLang != "" {
		params.Set("lang", normalizeXunfeiLang(p.cfg.SourceLang))
	}
	return xunfeiWSS + "?" + params.Encode(), nil
}

func buildXunfeiSigna(appID, apiKey, ts string) string {
	sum := md5.Sum([]byte(appID + ts))
	md5Hex := hex.EncodeToString(sum[:])
	mac := hmac.New(sha1.New, []byte(apiKey))
	mac.Write([]byte(md5Hex))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func normalizeXunfeiLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh", "zh-cn", "chinese":
		return "cn"
	default:
		return strings.TrimSpace(lang)
	}
}
