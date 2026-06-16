package translation

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
)

const (
	xunfeiMTNewEndpoint    = "https://itrans.xf-yun.com/v1/its"
	xunfeiMTLegacyEndpoint = "https://itrans.xfyun.cn/v2/its"
	xunfeiMTTimeout        = 5 * time.Second
)

type XunfeiMachineTranslationConfig struct {
	AppID     string
	APIKey    string
	APISecret string
	Endpoint  string
}

type XunfeiTextTranslator struct {
	cfg                 XunfeiMachineTranslationConfig
	client              *http.Client
	allowLegacyFallback bool
}

func NewXunfeiTextTranslator(cfg XunfeiMachineTranslationConfig) *XunfeiTextTranslator {
	allowLegacyFallback := cfg.Endpoint == ""
	if cfg.Endpoint == "" {
		cfg.Endpoint = xunfeiMTNewEndpoint
	}
	return &XunfeiTextTranslator{
		cfg:                 cfg,
		client:              &http.Client{Timeout: xunfeiMTTimeout},
		allowLegacyFallback: allowLegacyFallback,
	}
}

// ProbeXunfeiMachineTranslationConnection verifies the Machine Translation credentials
// with a minimal cn->en request. This consumes a tiny amount of text quota.
func ProbeXunfeiMachineTranslationConnection(ctx context.Context, cfg XunfeiMachineTranslationConfig) error {
	if !XunfeiMachineTranslationConfigReady(cfg) {
		return fmt.Errorf("xunfei_mt: incomplete config")
	}
	_, err := NewXunfeiTextTranslator(cfg).TranslateText(ctx, "测试", "cn", "en")
	return err
}

func (t *XunfeiTextTranslator) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || sourceLang == targetLang {
		return text, nil
	}
	sourceLang = normalizeXunfeiLang(sourceLang)
	targetLang = normalizeXunfeiLang(targetLang)
	translated, err := t.translateNew(ctx, text, sourceLang, targetLang)
	if err == nil {
		return translated, nil
	}
	if t.allowLegacyFallback && isXunfeiMTNotFound(err) {
		fallback, fallbackErr := t.translateLegacy(ctx, text, sourceLang, targetLang)
		if fallbackErr == nil {
			return fallback, nil
		}
		return text, fmt.Errorf("xunfei_mt: v1 failed: %v; v2 fallback failed: %w", err, fallbackErr)
	}
	return text, err
}

func (t *XunfeiTextTranslator) translateNew(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	body, err := json.Marshal(buildXunfeiMTRequest(t.cfg.AppID, text, sourceLang, targetLang))
	if err != nil {
		return "", fmt.Errorf("xunfei_mt: marshal request: %w", err)
	}

	endpoint, err := t.buildAuthURL()
	if err != nil {
		return "", fmt.Errorf("xunfei_mt: build auth URL: %w", err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, xunfeiMTTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("xunfei_mt: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("xunfei_mt: http: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("xunfei_mt: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("xunfei_mt: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	translated, err := parseXunfeiMTResponse(respBody)
	if err != nil {
		return "", err
	}
	return translated, nil
}

func (t *XunfeiTextTranslator) buildAuthURL() (string, error) {
	parsed, err := url.Parse(t.cfg.Endpoint)
	if err != nil {
		return "", err
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	signatureOrigin := strings.Join([]string{
		"host: " + parsed.Host,
		"date: " + date,
		"POST " + path + " HTTP/1.1",
	}, "\n")
	mac := hmac.New(sha256.New, []byte(t.cfg.APISecret))
	mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authOrigin := fmt.Sprintf(`api_key="%s",algorithm="hmac-sha256",headers="host date request-line",signature="%s"`, t.cfg.APIKey, signature)

	query := parsed.Query()
	query.Set("authorization", base64.StdEncoding.EncodeToString([]byte(authOrigin)))
	query.Set("date", date)
	query.Set("host", parsed.Host)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (t *XunfeiTextTranslator) translateLegacy(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	body, err := json.Marshal(buildXunfeiMTLegacyRequest(t.cfg.AppID, text, sourceLang, targetLang))
	if err != nil {
		return "", fmt.Errorf("xunfei_mt_v2: marshal request: %w", err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, xunfeiMTTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, xunfeiMTLegacyEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("xunfei_mt_v2: build request: %w", err)
	}
	if err := t.signLegacyRequest(req, body); err != nil {
		return "", fmt.Errorf("xunfei_mt_v2: sign request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("xunfei_mt_v2: http: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("xunfei_mt_v2: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("xunfei_mt_v2: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return parseXunfeiMTLegacyResponse(respBody)
}

func (t *XunfeiTextTranslator) signLegacyRequest(req *http.Request, body []byte) error {
	digestHash := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(digestHash[:])
	date := time.Now().UTC().Format(http.TimeFormat)
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	signatureOrigin := strings.Join([]string{
		"host: " + req.URL.Host,
		"date: " + date,
		"POST " + path + " HTTP/1.1",
		"digest: " + digest,
	}, "\n")
	mac := hmac.New(sha256.New, []byte(t.cfg.APISecret))
	mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authorization := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line digest", signature="%s"`, t.cfg.APIKey, signature)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json,version=1.0")
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("Date", date)
	req.Header.Set("Digest", digest)
	req.Header.Set("Authorization", authorization)
	return nil
}

type xunfeiMTRequest struct {
	Header struct {
		AppID  string `json:"app_id"`
		Status int    `json:"status"`
	} `json:"header"`
	Parameter struct {
		ITS struct {
			From   string         `json:"from"`
			To     string         `json:"to"`
			Result map[string]any `json:"result"`
		} `json:"its"`
	} `json:"parameter"`
	Payload struct {
		InputData struct {
			Encoding string `json:"encoding"`
			Status   int    `json:"status"`
			Text     string `json:"text"`
		} `json:"input_data"`
	} `json:"payload"`
}

func buildXunfeiMTRequest(appID, text, sourceLang, targetLang string) xunfeiMTRequest {
	req := xunfeiMTRequest{}
	req.Header.AppID = appID
	req.Header.Status = 3
	req.Parameter.ITS.From = sourceLang
	req.Parameter.ITS.To = targetLang
	req.Parameter.ITS.Result = map[string]any{}
	req.Payload.InputData.Encoding = "utf8"
	req.Payload.InputData.Status = 3
	req.Payload.InputData.Text = base64.StdEncoding.EncodeToString([]byte(text))
	return req
}

type xunfeiMTLegacyRequest struct {
	Common struct {
		AppID string `json:"app_id"`
	} `json:"common"`
	Business struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"business"`
	Data struct {
		Text string `json:"text"`
	} `json:"data"`
}

func buildXunfeiMTLegacyRequest(appID, text, sourceLang, targetLang string) xunfeiMTLegacyRequest {
	req := xunfeiMTLegacyRequest{}
	req.Common.AppID = appID
	req.Business.From = sourceLang
	req.Business.To = targetLang
	req.Data.Text = base64.StdEncoding.EncodeToString([]byte(text))
	return req
}

type xunfeiMTResponse struct {
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

type xunfeiMTText struct {
	TransResult struct {
		Dst string `json:"dst"`
		Src string `json:"src"`
	} `json:"trans_result"`
	From string `json:"from"`
	To   string `json:"to"`
}

func parseXunfeiMTResponse(data []byte) (string, error) {
	var resp xunfeiMTResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("xunfei_mt: parse response: %w", err)
	}
	if resp.Header.Code != 0 {
		return "", fmt.Errorf("xunfei_mt: code %d: %s", resp.Header.Code, resp.Header.Message)
	}
	if resp.Payload.Result.Text == "" {
		return "", fmt.Errorf("xunfei_mt: empty result")
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Payload.Result.Text)
	if err != nil {
		return "", fmt.Errorf("xunfei_mt: decode result: %w", err)
	}
	var result xunfeiMTText
	if err := json.Unmarshal(decoded, &result); err != nil {
		return "", fmt.Errorf("xunfei_mt: parse decoded result: %w", err)
	}
	if strings.TrimSpace(result.TransResult.Dst) == "" {
		return "", fmt.Errorf("xunfei_mt: empty translated text")
	}
	return strings.TrimSpace(result.TransResult.Dst), nil
}

type xunfeiMTLegacyResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Sid     string `json:"sid"`
	Data    struct {
		Result xunfeiMTText `json:"result"`
	} `json:"data"`
}

func parseXunfeiMTLegacyResponse(data []byte) (string, error) {
	var resp xunfeiMTLegacyResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("xunfei_mt_v2: parse response: %w", err)
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("xunfei_mt_v2: code %d: %s", resp.Code, resp.Message)
	}
	if strings.TrimSpace(resp.Data.Result.TransResult.Dst) == "" {
		return "", fmt.Errorf("xunfei_mt_v2: empty translated text")
	}
	return strings.TrimSpace(resp.Data.Result.TransResult.Dst), nil
}

func isXunfeiMTNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status 403") && strings.Contains(msg, "not found")
}
