package translation

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const (
	xunfeiRTASRLLMHost = "office-api-ast-dx.iflyaisol.com"
	xunfeiRTASRLLMPath = "/ast/communicate/v1"
	xunfeiRTASRLLMWSS  = "wss://" + xunfeiRTASRLLMHost + xunfeiRTASRLLMPath
)

type XunfeiRTASRLLMProvider struct {
	cfg XunfeiRTASRLLMConfig
}

type XunfeiRTASRLLMConfig struct {
	AppID      string
	APIKey     string
	APISecret  string
	SourceLang string
}

func NewXunfeiRTASRLLMProvider(cfg XunfeiRTASRLLMConfig) *XunfeiRTASRLLMProvider {
	return &XunfeiRTASRLLMProvider{cfg: cfg}
}

func (p *XunfeiRTASRLLMProvider) Translate(ctx context.Context, audioStream <-chan []byte) (<-chan DualResult, error) {
	if !XunfeiRTASRLLMConfigReady(p.cfg) {
		return nil, fmt.Errorf("xunfei_rtasr_llm: incomplete config")
	}
	out := make(chan DualResult, 32)
	go p.run(ctx, audioStream, out)
	return out, nil
}

func (p *XunfeiRTASRLLMProvider) Close() error { return nil }

func (p *XunfeiRTASRLLMProvider) run(ctx context.Context, audioStream <-chan []byte, out chan<- DualResult) {
	defer close(out)
	sessionID := uuid.NewString()
	endpoint := buildXunfeiRTASRLLMURL(p.cfg, sessionID, time.Now())
	conn, resp, err := xunfeiWebSocketDialer().DialContext(ctx, endpoint, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		diag.Warnf("xunfei_rtasr_llm dial failed err=%v", err)
		return
	}
	defer conn.Close()
	diag.Infof("xunfei_rtasr_llm connected session=%s source_lang=%s", sessionID, p.cfg.SourceLang)

	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		receiveXunfeiRTASRLLMLoop(conn, out)
	}()

	sendXunfeiRTASRLLMStream(ctx, conn, audioStream, sessionID)
	select {
	case <-recvDone:
	case <-time.After(3 * time.Second):
	}
}

func buildXunfeiRTASRLLMURL(cfg XunfeiRTASRLLMConfig, sessionID string, now time.Time) string {
	params := map[string]string{
		"accessKeyId":  cfg.APIKey,
		"appId":        cfg.AppID,
		"audio_encode": "pcm_s16le",
		"lang":         rtasrLLMLang(cfg.SourceLang),
		"samplerate":   "16000",
		"utc":          now.Format("2006-01-02T15:04:05-0700"),
		"uuid":         sessionID,
	}
	params["signature"] = signXunfeiRTASRLLM(params, cfg.APISecret)
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return xunfeiRTASRLLMWSS + "?" + values.Encode()
}

func signXunfeiRTASRLLM(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key != "signature" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	values := url.Values{}
	for _, key := range keys {
		values.Set(key, params[key])
	}
	baseString := values.Encode()
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(baseString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func sendXunfeiRTASRLLMStream(ctx context.Context, conn *websocket.Conn, audioStream <-chan []byte, sessionID string) {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteJSON(map[string]any{"end": true, "sessionId": sessionID})
			return
		case frame, ok := <-audioStream:
			if !ok {
				_ = conn.WriteJSON(map[string]any{"end": true, "sessionId": sessionID})
				return
			}
			if len(frame) == 0 {
				continue
			}
			<-ticker.C
			if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
				diag.Warnf("xunfei_rtasr_llm send failed err=%v", err)
				return
			}
		}
	}
}

func receiveXunfeiRTASRLLMLoop(conn *websocket.Conn, out chan<- DualResult) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		result, ok := parseXunfeiRTASRLLMResponse(data)
		if !ok {
			if err := parseXunfeiRTASRLLMError(data); err != nil {
				diag.Warnf("xunfei_rtasr_llm response error err=%v preview=%q", err, previewResponse(data))
			}
			continue
		}
		diag.Infof("xunfei_rtasr_llm text final=%t src_chars=%d", result.IsFinal, len(result.SrcText))
		out <- result
	}
}

type xunfeiRTASRLLMResponse struct {
	MsgType string `json:"msg_type"`
	ResType string `json:"res_type"`
	Code    string `json:"code"`
	Desc    string `json:"desc"`
	Data    struct {
		SegID int  `json:"seg_id"`
		LS    bool `json:"ls"`
		CN    struct {
			ST struct {
				Type string `json:"type"`
				RT   []struct {
					WS []struct {
						CW []struct {
							W  string `json:"w"`
							WP string `json:"wp"`
						} `json:"cw"`
					} `json:"ws"`
				} `json:"rt"`
			} `json:"st"`
		} `json:"cn"`
	} `json:"data"`
}

func parseXunfeiRTASRLLMResponse(data []byte) (DualResult, bool) {
	var resp xunfeiRTASRLLMResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return DualResult{}, false
	}
	if resp.MsgType != "result" || resp.ResType != "asr" {
		return DualResult{}, false
	}
	var b strings.Builder
	for _, rt := range resp.Data.CN.ST.RT {
		for _, ws := range rt.WS {
			for _, cw := range ws.CW {
				if cw.WP == "s" || cw.WP == "g" {
					continue
				}
				b.WriteString(cw.W)
			}
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return DualResult{}, false
	}
	return DualResult{SrcText: text, IsFinal: resp.Data.LS}, true
}

func parseXunfeiRTASRLLMError(data []byte) error {
	var resp xunfeiRTASRLLMResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	if resp.MsgType == "error" || resp.Code != "" || strings.TrimSpace(resp.Desc) != "" {
		return fmt.Errorf("xunfei_rtasr_llm: code=%s desc=%s", resp.Code, strings.TrimSpace(resp.Desc))
	}
	return nil
}

func rtasrLLMLang(lang string) string {
	switch normalizeXunfeiSimultLang(lang) {
	case "en":
		return "autodialect"
	case "cn":
		return "autodialect"
	default:
		return "autominor"
	}
}

func XunfeiRTASRLLMConfigReady(cfg XunfeiRTASRLLMConfig) bool {
	return strings.TrimSpace(cfg.AppID) != "" &&
		strings.TrimSpace(cfg.APIKey) != "" &&
		strings.TrimSpace(cfg.APISecret) != "" &&
		strings.TrimSpace(cfg.SourceLang) != ""
}
