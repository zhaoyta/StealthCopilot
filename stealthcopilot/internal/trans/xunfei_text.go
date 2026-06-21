package trans

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/asr"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const (
	xunfeiTextTransHost = "itrans.xf-yun.com"
	xunfeiTextTransPath = "/v1/its"
	xunfeiTextTransURL  = "https://" + xunfeiTextTransHost + xunfeiTextTransPath
)

type XunfeiTextExtension struct {
	cfg    XunfeiTextTransConfig
	client *http.Client
}

type XunfeiTextTransConfig struct {
	AppID      string
	APIKey     string
	APISecret  string
	SourceLang string
	TargetLang string
}

func NewXunfeiTextExtension(cfg XunfeiTextTransConfig) *XunfeiTextExtension {
	return &XunfeiTextExtension{
		cfg:    cfg,
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *XunfeiTextExtension) Process(ctx context.Context, result asr.Result) (asr.Result, error) {
	text := strings.TrimSpace(result.SrcText)
	if text == "" || result.DstText != "" || asr.NormalizeXunfeiSimultLang(s.cfg.SourceLang) == asr.NormalizeXunfeiSimultLang(s.cfg.TargetLang) {
		return result, nil
	}
	translated, err := s.translate(ctx, text)
	if err != nil {
		return result, err
	}
	result.DstText = translated
	return result, nil
}

func (s *XunfeiTextExtension) translate(ctx context.Context, text string) (string, error) {
	if !XunfeiTextTransConfigReady(s.cfg) {
		return "", fmt.Errorf("xunfei_text_trans: incomplete config")
	}
	started := time.Now()
	body, err := json.Marshal(xunfeiTextTransRequest{
		Header: xunfeiTextTransHeader{
			AppID:  s.cfg.AppID,
			Status: 3,
		},
		Parameter: xunfeiTextTransParameter{
			ITS: xunfeiTextTransITSParameter{
				From:   asr.NormalizeXunfeiSimultLang(s.cfg.SourceLang),
				To:     asr.NormalizeXunfeiSimultLang(s.cfg.TargetLang),
				Result: map[string]any{},
			},
		},
		Payload: xunfeiTextTransPayload{
			InputData: xunfeiTextTransInputData{
				Encoding: "utf8",
				Status:   3,
				Text:     base64.StdEncoding.EncodeToString([]byte(text)),
			},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildXunfeiTextTransURL(s.cfg, time.Now().UTC()), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("xunfei_text_trans: request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("xunfei_text_trans: http %d: %s", resp.StatusCode, previewResponse(raw))
	}
	translated, err := parseXunfeiTextTransResponse(raw)
	if err != nil {
		return "", err
	}
	diag.Infof("xunfei_text_trans ok elapsed=%s src_chars=%d dst_chars=%d", diag.Since(started), len(text), len(translated))
	return translated, nil
}

func buildXunfeiTextTransURL(cfg XunfeiTextTransConfig, now time.Time) string {
	date := now.Format(time.RFC1123)
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nPOST %s HTTP/1.1", xunfeiTextTransHost, date, xunfeiTextTransPath)
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
	params.Set("host", xunfeiTextTransHost)
	params.Set("date", date)
	return xunfeiTextTransURL + "?" + params.Encode()
}

func XunfeiTextTransConfigReady(cfg XunfeiTextTransConfig) bool {
	return strings.TrimSpace(cfg.AppID) != "" &&
		strings.TrimSpace(cfg.APIKey) != "" &&
		strings.TrimSpace(cfg.APISecret) != "" &&
		strings.TrimSpace(cfg.SourceLang) != "" &&
		strings.TrimSpace(cfg.TargetLang) != ""
}

type xunfeiTextTransRequest struct {
	Header    xunfeiTextTransHeader    `json:"header"`
	Parameter xunfeiTextTransParameter `json:"parameter"`
	Payload   xunfeiTextTransPayload   `json:"payload"`
}

type xunfeiTextTransHeader struct {
	AppID  string `json:"app_id"`
	Status int    `json:"status"`
}

type xunfeiTextTransParameter struct {
	ITS xunfeiTextTransITSParameter `json:"its"`
}

type xunfeiTextTransITSParameter struct {
	From   string         `json:"from"`
	To     string         `json:"to"`
	Result map[string]any `json:"result"`
}

type xunfeiTextTransPayload struct {
	InputData xunfeiTextTransInputData `json:"input_data"`
}

type xunfeiTextTransInputData struct {
	Encoding string `json:"encoding"`
	Status   int    `json:"status"`
	Text     string `json:"text"`
}

type xunfeiTextTransResponse struct {
	Header struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Sid     string `json:"sid"`
	} `json:"header"`
	Payload struct {
		Result struct {
			Text string `json:"text"`
		} `json:"result"`
	} `json:"payload"`
}

type xunfeiTextTransDecoded struct {
	TransResult struct {
		Dst string `json:"dst"`
		Src string `json:"src"`
	} `json:"trans_result"`
	From string `json:"from"`
	To   string `json:"to"`
}

func parseXunfeiTextTransResponse(raw []byte) (string, error) {
	var resp xunfeiTextTransResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	if resp.Header.Code != 0 {
		return "", fmt.Errorf("xunfei_text_trans: code=%d message=%s", resp.Header.Code, strings.TrimSpace(resp.Header.Message))
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Payload.Result.Text)
	if err != nil {
		return "", err
	}
	var result xunfeiTextTransDecoded
	if err := json.Unmarshal(decoded, &result); err != nil {
		return "", err
	}
	dst := strings.TrimSpace(result.TransResult.Dst)
	if dst == "" {
		return "", asr.ErrNoTranslationReturned
	}
	return dst, nil
}

func previewResponse(data []byte) string {
	const max = 240
	text := string(data)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
