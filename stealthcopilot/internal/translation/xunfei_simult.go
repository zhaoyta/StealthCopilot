package translation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const (
	xunfeiSimultHost = "ws-api.xf-yun.com"
	xunfeiSimultPath = "/v1/private/simult_interpretation"
	xunfeiSimultWSS  = "wss://" + xunfeiSimultHost + xunfeiSimultPath
)

type XunfeiSimultConfig struct {
	AppID      string
	APIKey     string
	APISecret  string
	SourceLang string
	TargetLang string
}

type XunfeiSimultProvider struct {
	cfg XunfeiSimultConfig
}

func NewXunfeiSimultProvider(cfg XunfeiSimultConfig) *XunfeiSimultProvider {
	return &XunfeiSimultProvider{cfg: cfg}
}

func ProbeXunfeiSimultConnection(ctx context.Context, cfg XunfeiSimultConfig) error {
	if !XunfeiSimultConfigReady(cfg) {
		return fmt.Errorf("xunfei_simult: incomplete config")
	}
	if !XunfeiSimultLangPairSupported(cfg.SourceLang, cfg.TargetLang) {
		return fmt.Errorf("xunfei_simult: unsupported language pair source=%s target=%s", cfg.SourceLang, cfg.TargetLang)
	}
	endpoint := buildXunfeiSimultURL(cfg, time.Now().UTC())
	conn, resp, err := xunfeiWebSocketDialer().DialContext(ctx, endpoint, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("xunfei_simult: dial: %w", err)
	}
	return conn.Close()
}

func (p *XunfeiSimultProvider) Translate(ctx context.Context, audioStream <-chan []byte) (<-chan DualResult, error) {
	if !XunfeiSimultConfigReady(p.cfg) {
		return nil, fmt.Errorf("xunfei_simult: incomplete config")
	}
	if !XunfeiSimultLangPairSupported(p.cfg.SourceLang, p.cfg.TargetLang) {
		return nil, fmt.Errorf("xunfei_simult: unsupported language pair source=%s target=%s", p.cfg.SourceLang, p.cfg.TargetLang)
	}
	out := make(chan DualResult, 32)
	go p.run(ctx, audioStream, out)
	return out, nil
}

func (p *XunfeiSimultProvider) Close() error { return nil }

func (p *XunfeiSimultProvider) run(ctx context.Context, audioStream <-chan []byte, out chan<- DualResult) {
	defer close(out)
	endpoint := buildXunfeiSimultURL(p.cfg, time.Now().UTC())
	conn, resp, err := xunfeiWebSocketDialer().DialContext(ctx, endpoint, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		diag.Warnf("xunfei_simult dial failed err=%v", err)
		return
	}
	defer conn.Close()

	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		receiveXunfeiSimultLoop(conn, out)
	}()

	sendXunfeiSimultStream(ctx, conn, p.cfg, audioStream)
	select {
	case <-recvDone:
	case <-time.After(3 * time.Second):
	}
}

type XunfeiSimultSpeakProvider struct {
	cfg XunfeiSimultConfig
}

func NewXunfeiSimultSpeakProvider(cfg XunfeiSimultConfig) *XunfeiSimultSpeakProvider {
	return &XunfeiSimultSpeakProvider{cfg: cfg}
}

func (p *XunfeiSimultSpeakProvider) Translate(ctx context.Context, pcmData []byte) (DualResult, error) {
	if !XunfeiSimultConfigReady(p.cfg) {
		return DualResult{}, fmt.Errorf("xunfei_simult: incomplete config")
	}
	if !XunfeiSimultLangPairSupported(p.cfg.SourceLang, p.cfg.TargetLang) {
		return DualResult{}, fmt.Errorf("xunfei_simult: unsupported language pair source=%s target=%s", p.cfg.SourceLang, p.cfg.TargetLang)
	}
	diag.Infof(
		"xunfei_simult_speak start pcm_bytes=%d approx_ms=%d peak=%d source_lang=%s target_lang=%s",
		len(pcmData), pcmDurationMs(pcmData), pcmPeak(pcmData), p.cfg.SourceLang, p.cfg.TargetLang,
	)
	timeoutCtx, cancel := context.WithTimeout(ctx, xunfeiSimultSpeakTimeout(pcmData))
	defer cancel()

	endpoint := buildXunfeiSimultURL(p.cfg, time.Now().UTC())
	conn, resp, err := xunfeiWebSocketDialer().DialContext(timeoutCtx, endpoint, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return DualResult{}, fmt.Errorf("xunfei_simult: dial: %w", err)
	}
	defer conn.Close()

	framesSent, err := sendXunfeiSimultPCM(timeoutCtx, conn, p.cfg, pcmData)
	if err != nil {
		return DualResult{}, fmt.Errorf("xunfei_simult: send frames: %w", err)
	}
	diag.Infof("xunfei_simult_speak sent frames=%d pcm_bytes=%d", framesSent, len(pcmData))

	result, err := waitXunfeiSimultFinal(conn, p.cfg)
	if err != nil {
		return DualResult{}, err
	}
	if result.SrcText == "" && result.DstText == "" {
		return DualResult{}, ErrNoSpeechRecognized
	}
	if result.DstText == "" && !xunfeiSimultNeedsTranslation(p.cfg) {
		result.DstText = result.SrcText
	}
	result.IsFinal = true
	diag.Infof("xunfei_simult_speak ok src_chars=%d dst_chars=%d", len(result.SrcText), len(result.DstText))
	return result, nil
}

func xunfeiSimultSpeakTimeout(pcmData []byte) time.Duration {
	timeout := time.Duration(pcmDurationMs(pcmData))*time.Millisecond + 12*time.Second
	if timeout < 20*time.Second {
		return 20 * time.Second
	}
	if timeout > 90*time.Second {
		return 90 * time.Second
	}
	return timeout
}

func buildXunfeiSimultURL(cfg XunfeiSimultConfig, now time.Time) string {
	date := now.Format(time.RFC1123)
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET %s HTTP/1.1", xunfeiSimultHost, date, xunfeiSimultPath)
	mac := hmac.New(sha256.New, []byte(cfg.APISecret))
	_, _ = mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authorizationOrigin := fmt.Sprintf(
		`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		cfg.APIKey,
		signature,
	)
	params := url.Values{}
	params.Set("authorization", base64.StdEncoding.EncodeToString([]byte(authorizationOrigin)))
	params.Set("host", xunfeiSimultHost)
	params.Set("date", date)
	params.Set("serviceId", "simult_interpretation")
	return xunfeiSimultWSS + "?" + params.Encode()
}

func sendXunfeiSimultStream(ctx context.Context, conn *websocket.Conn, cfg XunfeiSimultConfig, audioStream <-chan []byte) {
	seq := 0
	for {
		select {
		case <-ctx.Done():
			_ = writeXunfeiSimultFrame(conn, cfg, nil, seq, 2)
			return
		case frame, ok := <-audioStream:
			if !ok {
				_ = writeXunfeiSimultFrame(conn, cfg, nil, seq, 2)
				return
			}
			status := 1
			if seq == 0 {
				status = 0
			}
			if err := writeXunfeiSimultFrame(conn, cfg, frame, seq, status); err != nil {
				return
			}
			seq++
		}
	}
}

func sendXunfeiSimultPCM(ctx context.Context, conn *websocket.Conn, cfg XunfeiSimultConfig, pcmData []byte) (int, error) {
	const frameSize = 1280
	frames := 0
	for offset := 0; offset < len(pcmData); offset += frameSize {
		select {
		case <-ctx.Done():
			return frames, ctx.Err()
		default:
		}
		end := offset + frameSize
		if end > len(pcmData) {
			end = len(pcmData)
		}
		status := 1
		if frames == 0 {
			status = 0
		}
		if err := writeXunfeiSimultFrame(conn, cfg, pcmData[offset:end], frames, status); err != nil {
			return frames, err
		}
		frames++
		time.Sleep(40 * time.Millisecond)
	}
	return frames, writeXunfeiSimultFrame(conn, cfg, nil, frames, 2)
}

func writeXunfeiSimultFrame(conn *websocket.Conn, cfg XunfeiSimultConfig, pcm []byte, seq, status int) error {
	req := xunfeiSimultRequest{
		Header: xunfeiSimultRequestHeader{
			AppID:  cfg.AppID,
			Status: status,
		},
		Parameter: xunfeiSimultRequestParameter{
			IST: xunfeiSimultISTParameter{
				Accent:   "mandarin",
				Domain:   "ist_ed_open",
				Language: xunfeiSimultRecognitionLang(cfg.SourceLang),
				VTO:      15000,
				EOS:      150000,
			},
			StreamTrans: xunfeiSimultStreamTransParameter{
				From: normalizeXunfeiSimultLang(cfg.SourceLang),
				To:   normalizeXunfeiSimultLang(cfg.TargetLang),
			},
			TTS: xunfeiSimultTTSParameter{
				VCN: "x2_john",
				Results: xunfeiSimultTTSResultsParameter{
					Encoding:   "raw",
					SampleRate: 16000,
					Channels:   1,
					BitDepth:   16,
				},
			},
		},
		Payload: xunfeiSimultPayload{
			Data: xunfeiSimultAudioPayload{
				Audio:      base64.StdEncoding.EncodeToString(pcm),
				Encoding:   "raw",
				SampleRate: 16000,
				Seq:        seq,
				Status:     status,
			},
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func receiveXunfeiSimultLoop(conn *websocket.Conn, out chan<- DualResult) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		result, ok := parseXunfeiSimultResponse(data)
		if !ok {
			if responseErr := parseXunfeiSimultError(data); responseErr != nil {
				diag.Warnf("xunfei_simult response error err=%v preview=%q", responseErr, previewResponse(data))
			}
			if !isXunfeiSimultEmptySuccess(data) {
				diag.Warnf("xunfei_simult response ignored bytes=%d preview=%q", len(data), previewResponse(data))
			}
			continue
		}
		select {
		case out <- result:
		default:
		}
	}
}

func waitXunfeiSimultFinal(conn *websocket.Conn, cfg XunfeiSimultConfig) (DualResult, error) {
	const idleWait = 3 * time.Second
	_ = conn.SetReadDeadline(time.Now().Add(idleWait))
	var last DualResult
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if last.DstText != "" || (last.SrcText != "" && !xunfeiSimultNeedsTranslation(cfg)) {
				last.IsFinal = true
				diag.Infof("xunfei_simult returning last text after idle src_chars=%d dst_chars=%d", len(last.SrcText), len(last.DstText))
				return last, nil
			}
			if last.SrcText != "" && xunfeiSimultNeedsTranslation(cfg) {
				return DualResult{}, ErrNoTranslationReturned
			}
			if isTimeoutErr(err) {
				return DualResult{}, ErrNoSpeechRecognized
			}
			return DualResult{}, fmt.Errorf("xunfei_simult: read: %w", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(idleWait))
		result, ok := parseXunfeiSimultResponse(data)
		if !ok {
			if responseErr := parseXunfeiSimultError(data); responseErr != nil {
				return DualResult{}, responseErr
			}
			if !isXunfeiSimultEmptySuccess(data) {
				diag.Warnf("xunfei_simult response ignored bytes=%d preview=%q", len(data), previewResponse(data))
			}
			continue
		}
		if result.SrcText != "" {
			last.SrcText = result.SrcText
		}
		if result.DstText != "" {
			last.DstText = result.DstText
		}
		if result.IsFinal {
			last.IsFinal = true
		}
		if result.SrcText != "" || result.DstText != "" {
			diag.Infof("xunfei_simult response text final=%t src_chars=%d dst_chars=%d", result.IsFinal, len(result.SrcText), len(result.DstText))
		}
		if result.IsFinal && (last.DstText != "" || (last.SrcText != "" && !xunfeiSimultNeedsTranslation(cfg))) {
			return last, nil
		}
	}
}

func isTimeoutErr(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "i/o timeout")
}

func isXunfeiBlankASRData(data []byte) bool {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false
	}
	hasWordField := false
	hasNonEmptyWord := false
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			if word, ok := typed["w"].(string); ok {
				hasWordField = true
				if strings.TrimSpace(word) != "" {
					hasNonEmptyWord = true
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return hasWordField && !hasNonEmptyWord
}

func xunfeiSimultNeedsTranslation(cfg XunfeiSimultConfig) bool {
	return normalizeXunfeiSimultLang(cfg.SourceLang) != normalizeXunfeiSimultLang(cfg.TargetLang)
}

type xunfeiSimultRequest struct {
	Header    xunfeiSimultRequestHeader    `json:"header"`
	Parameter xunfeiSimultRequestParameter `json:"parameter"`
	Payload   xunfeiSimultPayload          `json:"payload"`
}

type xunfeiSimultRequestHeader struct {
	AppID  string `json:"app_id"`
	Status int    `json:"status"`
}

type xunfeiSimultRequestParameter struct {
	IST         xunfeiSimultISTParameter         `json:"ist"`
	StreamTrans xunfeiSimultStreamTransParameter `json:"streamtrans"`
	TTS         xunfeiSimultTTSParameter         `json:"tts"`
}

type xunfeiSimultISTParameter struct {
	Accent   string `json:"accent"`
	Domain   string `json:"domain"`
	Language string `json:"language"`
	VTO      int    `json:"vto"`
	EOS      int    `json:"eos"`
}

type xunfeiSimultStreamTransParameter struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type xunfeiSimultTTSParameter struct {
	VCN     string                          `json:"vcn"`
	Results xunfeiSimultTTSResultsParameter `json:"tts_results"`
}

type xunfeiSimultTTSResultsParameter struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
	BitDepth   int    `json:"bit_depth"`
}

type xunfeiSimultPayload struct {
	Data xunfeiSimultAudioPayload `json:"data"`
}

type xunfeiSimultAudioPayload struct {
	Audio      string `json:"audio"`
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
	Seq        int    `json:"seq"`
	Status     int    `json:"status"`
}

type xunfeiSimultResponse struct {
	Header struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Sid     string `json:"sid"`
		Status  int    `json:"status"`
	} `json:"header"`
	Payload struct {
		RecognitionResults *xunfeiSimultEncodedResult `json:"recognition_results"`
		StreamTransResults *xunfeiSimultEncodedResult `json:"streamtrans_results"`
		TTSResults         *xunfeiSimultTTSResult     `json:"tts_results"`
	} `json:"payload"`
}

type xunfeiSimultEncodedResult struct {
	Text   string          `json:"text"`
	Status json.RawMessage `json:"status"`
}

type xunfeiSimultTTSResult struct {
	Audio  string          `json:"audio"`
	Status json.RawMessage `json:"status"`
}

type xunfeiSimultTransText struct {
	Src     string `json:"src"`
	Dst     string `json:"dst"`
	IsFinal int    `json:"is_final"`
}

func parseXunfeiSimultResponse(data []byte) (DualResult, bool) {
	var resp xunfeiSimultResponse
	if err := json.Unmarshal(data, &resp); err != nil || resp.Header.Code != 0 {
		return DualResult{}, false
	}
	if resp.Payload.TTSResults != nil && resp.Payload.TTSResults.Audio != "" {
		audio, err := base64.StdEncoding.DecodeString(resp.Payload.TTSResults.Audio)
		if err != nil {
			return DualResult{}, false
		}
		return DualResult{AudioPCM: audio}, len(audio) > 0
	}
	if resp.Payload.StreamTransResults != nil && resp.Payload.StreamTransResults.Text != "" {
		decoded, err := base64.StdEncoding.DecodeString(resp.Payload.StreamTransResults.Text)
		if err != nil {
			return DualResult{}, false
		}
		var trans xunfeiSimultTransText
		if err := json.Unmarshal(decoded, &trans); err != nil {
			return DualResult{}, false
		}
		return DualResult{
			SrcText: strings.TrimSpace(trans.Src),
			DstText: strings.TrimSpace(trans.Dst),
			IsFinal: trans.IsFinal == 1,
		}, trans.Src != "" || trans.Dst != ""
	}
	if resp.Payload.RecognitionResults != nil && resp.Payload.RecognitionResults.Text != "" {
		decoded, err := base64.StdEncoding.DecodeString(resp.Payload.RecognitionResults.Text)
		if err != nil {
			diag.Warnf("xunfei_simult recognition decode failed err=%v", err)
			return DualResult{}, false
		}
		result, ok := parseXunfeiASRData(decoded)
		if !ok {
			if isXunfeiBlankASRData(decoded) {
				return DualResult{}, false
			}
			diag.Warnf("xunfei_simult recognition parse failed decoded=%q", trimLogString(string(decoded), 500))
			return DualResult{}, false
		}
		result.DstText = ""
		return result, result.SrcText != ""
	}
	return DualResult{}, false
}

func parseXunfeiSimultError(data []byte) error {
	var resp xunfeiSimultResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	if resp.Header.Code == 0 {
		return nil
	}
	return fmt.Errorf("xunfei_simult: code=%d message=%s", resp.Header.Code, strings.TrimSpace(resp.Header.Message))
}

func isXunfeiSimultEmptySuccess(data []byte) bool {
	var resp xunfeiSimultResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return false
	}
	return resp.Header.Code == 0 &&
		resp.Payload.RecognitionResults == nil &&
		resp.Payload.StreamTransResults == nil &&
		resp.Payload.TTSResults == nil
}

func trimLogString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func normalizeXunfeiSimultLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh", "zh-cn", "zh_cn", "cn":
		return "cn"
	case "en", "en-us", "en_us":
		return "en"
	default:
		return strings.ToLower(strings.TrimSpace(lang))
	}
}

func XunfeiSimultLangPairSupported(sourceLang, targetLang string) bool {
	return normalizeXunfeiSimultLang(sourceLang) == "cn" && normalizeXunfeiSimultLang(targetLang) == "en"
}

func xunfeiSimultRecognitionLang(lang string) string {
	switch normalizeXunfeiSimultLang(lang) {
	case "cn":
		return "zh_cn"
	case "en":
		return "en_us"
	default:
		return "zh_cn"
	}
}

func XunfeiSimultConfigReady(cfg XunfeiSimultConfig) bool {
	return strings.TrimSpace(cfg.AppID) != "" &&
		strings.TrimSpace(cfg.APIKey) != "" &&
		strings.TrimSpace(cfg.APISecret) != "" &&
		strings.TrimSpace(cfg.SourceLang) != "" &&
		strings.TrimSpace(cfg.TargetLang) != ""
}
