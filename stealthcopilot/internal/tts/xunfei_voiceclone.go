package tts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const (
	xunfeiVoiceTokenURL       = "http://avatar-hci.xfyousheng.com/aiauth/v1/token"
	xunfeiVoiceTrainTextURL   = "http://opentrain.xfyousheng.com/voice_train/task/traintext"
	xunfeiVoiceTaskAddURL     = "http://opentrain.xfyousheng.com/voice_train/task/add"
	xunfeiVoiceSubmitAudioURL = "http://opentrain.xfyousheng.com/voice_train/task/submitWithAudio"
	xunfeiVoiceTaskResultURL  = "http://opentrain.xfyousheng.com/voice_train/task/result"
	xunfeiVoiceCloneWSURL     = "wss://cn-huabei-1.xf-yun.com/v1/private/voice_clone"
	xunfeiVoiceTrainTextID    = "5001"
	xunfeiVoiceDefaultSegID   = "1"
	xunfeiVoiceTimeout        = 30 * time.Second
	xunfeiVoiceCloneVCN       = "x6_clone"
)

type XunfeiVoiceCloneConfig struct {
	AppID     string
	APIKey    string
	APISecret string
	AssetID   string
	TaskID    string
}

type XunfeiVoiceCloneProvider struct {
	cfg XunfeiVoiceCloneConfig
}

func NewXunfeiVoiceCloneProvider(cfg XunfeiVoiceCloneConfig) *XunfeiVoiceCloneProvider {
	return &XunfeiVoiceCloneProvider{cfg: cfg}
}

func XunfeiVoiceCloneConfigReady(cfg XunfeiVoiceCloneConfig) bool {
	return strings.TrimSpace(cfg.AppID) != "" &&
		strings.TrimSpace(cfg.APIKey) != "" &&
		strings.TrimSpace(cfg.APISecret) != "" &&
		strings.TrimSpace(cfg.AssetID) != ""
}

func (p *XunfeiVoiceCloneProvider) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	started := time.Now()
	text = strings.TrimSpace(text)
	if text == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	diag.Infof("xunfei_voiceclone synth start chars=%d asset_set=%t", len(text), strings.TrimSpace(p.cfg.AssetID) != "")
	if !XunfeiVoiceCloneConfigReady(p.cfg) {
		return nil, fmt.Errorf("xunfei_voiceclone: AppID/API Key/API Secret/AssetID 未完整配置")
	}
	authURL, err := buildXunfeiVoiceCloneAuthURL(p.cfg)
	if err != nil {
		return nil, err
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, authURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("xunfei_voiceclone: websocket status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("xunfei_voiceclone: websocket: %w", err)
	}
	diag.Infof("xunfei_voiceclone websocket connected elapsed=%s", diag.Since(started))
	if err := conn.WriteJSON(buildXunfeiVoiceCloneSynthesisRequest(p.cfg.AppID, p.cfg.AssetID, text)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("xunfei_voiceclone: send synthesis request: %w", err)
	}
	diag.Infof("xunfei_voiceclone request sent chars=%d", len(text))

	ch := make(chan []byte, 16)
	go func() {
		defer close(ch)
		defer conn.Close()
		chunks := 0
		bytes := 0
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				diag.Warnf("xunfei_voiceclone read ended elapsed=%s chunks=%d bytes=%d err=%v", diag.Since(started), chunks, bytes, err)
				return
			}
			var msg xunfeiVoiceCloneSynthesisResponse
			if err := json.Unmarshal(data, &msg); err != nil {
				diag.Warnf("xunfei_voiceclone response parse failed elapsed=%s err=%v", diag.Since(started), err)
				return
			}
			if msg.Header.Code != 0 {
				diag.Warnf("xunfei_voiceclone response code=%d message=%q elapsed=%s", msg.Header.Code, msg.Header.Message, diag.Since(started))
				return
			}
			if msg.Payload.Audio.Audio != "" {
				audio, err := base64.StdEncoding.DecodeString(msg.Payload.Audio.Audio)
				if err == nil && len(audio) > 0 {
					chunks++
					bytes += len(audio)
					if chunks == 1 || chunks%20 == 0 {
						diag.Infof("xunfei_voiceclone audio chunk chunks=%d bytes=%d last_chunk=%d", chunks, bytes, len(audio))
					}
					select {
					case ch <- audio:
					case <-ctx.Done():
						diag.Warnf("xunfei_voiceclone canceled elapsed=%s chunks=%d bytes=%d", diag.Since(started), chunks, bytes)
						return
					}
				}
			}
			if msg.Header.Status == 2 || msg.Payload.Audio.Status == 2 {
				diag.Infof("xunfei_voiceclone synth done elapsed=%s chunks=%d bytes=%d", diag.Since(started), chunks, bytes)
				return
			}
		}
	}()
	return ch, nil
}

func (p *XunfeiVoiceCloneProvider) VoiceID() string { return p.cfg.AssetID }

func (p *XunfeiVoiceCloneProvider) Close() error { return nil }

type XunfeiVoiceCloneClient struct {
	cfg             XunfeiVoiceCloneConfig
	client          *http.Client
	trainingSignKey string
}

func NewXunfeiVoiceCloneClient(cfg XunfeiVoiceCloneConfig) *XunfeiVoiceCloneClient {
	return &XunfeiVoiceCloneClient{
		cfg:    cfg,
		client: &http.Client{Timeout: xunfeiVoiceTimeout},
	}
}

type XunfeiVoiceTrainText struct {
	TextID    string `json:"text_id"`
	TextSegID string `json:"text_seg_id"`
	Text      string `json:"text"`
}

type XunfeiVoiceTrainResult struct {
	TaskID      string `json:"task_id"`
	AssetID     string `json:"asset_id"`
	TrainStatus int    `json:"train_status"`
	FailedDesc  string `json:"failed_desc"`
}

// ProbeToken 仅获取一次 access token，用于连通性验证，不产生额外业务请求。
func (c *XunfeiVoiceCloneClient) ProbeToken(ctx context.Context) error {
	_, err := c.fetchToken(ctx)
	return err
}

func (c *XunfeiVoiceCloneClient) FetchTrainText(ctx context.Context) (XunfeiVoiceTrainText, error) {
	token, err := c.fetchToken(ctx)
	if err != nil {
		return XunfeiVoiceTrainText{}, err
	}
	body := []byte(`{"textId":` + xunfeiVoiceTrainTextID + `}`)
	var resp xunfeiTrainTextResponse
	if err := c.postTrainingJSON(ctx, xunfeiVoiceTrainTextURL, token, body, &resp); err != nil {
		return XunfeiVoiceTrainText{}, err
	}
	if resp.Code != 0 || !resp.Flag {
		return XunfeiVoiceTrainText{}, fmt.Errorf("xunfei_voiceclone: train text code %d: %s", resp.Code, resp.Desc)
	}
	for _, seg := range resp.Data.TextSegs {
		if strings.TrimSpace(seg.SegText) != "" {
			return XunfeiVoiceTrainText{
				TextID:    strconv.FormatInt(resp.Data.TextID, 10),
				TextSegID: stringOr(seg.SegID.String(), xunfeiVoiceDefaultSegID),
				Text:      strings.TrimSpace(seg.SegText),
			}, nil
		}
	}
	return XunfeiVoiceTrainText{}, fmt.Errorf("xunfei_voiceclone: empty train text")
}

func (c *XunfeiVoiceCloneClient) SubmitTrainingAudio(ctx context.Context, wavAudio []byte) (string, error) {
	if len(wavAudio) == 0 {
		return "", fmt.Errorf("xunfei_voiceclone: empty training audio")
	}
	trainText, err := c.FetchTrainText(ctx)
	if err != nil {
		return "", err
	}
	token, err := c.fetchToken(ctx)
	if err != nil {
		return "", err
	}
	taskID, err := c.createTask(ctx, token)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("taskId", taskID)
	_ = writer.WriteField("textId", trainText.TextID)
	_ = writer.WriteField("textSegId", trainText.TextSegID)
	part, err := writer.CreateFormFile("file", "voice_clone_train.wav")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(wavAudio); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xunfeiVoiceSubmitAudioURL, bytes.NewReader(body.Bytes()))
	if err != nil {
		return "", err
	}
	c.setTrainingHeaders(req, token, body.Bytes())
	req.Header.Set("Content-Type", writer.FormDataContentType())
	respBody, err := c.do(req)
	if err != nil {
		return "", err
	}
	var resp xunfeiCommonResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("xunfei_voiceclone: parse submit response: %w", err)
	}
	if resp.Code != 0 || !resp.Flag {
		return "", fmt.Errorf("xunfei_voiceclone: submit code %d: %s", resp.Code, resp.Desc)
	}
	return taskID, nil
}

func (c *XunfeiVoiceCloneClient) QueryTrainingResult(ctx context.Context, taskID string) (XunfeiVoiceTrainResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = strings.TrimSpace(c.cfg.TaskID)
	}
	if taskID == "" {
		return XunfeiVoiceTrainResult{}, fmt.Errorf("xunfei_voiceclone: task id empty")
	}
	token, err := c.fetchToken(ctx)
	if err != nil {
		return XunfeiVoiceTrainResult{}, err
	}
	body, _ := json.Marshal(map[string]string{"taskId": taskID})
	var resp xunfeiTaskResultResponse
	if err := c.postTrainingJSON(ctx, xunfeiVoiceTaskResultURL, token, body, &resp); err != nil {
		return XunfeiVoiceTrainResult{}, err
	}
	if resp.Code != 0 || !resp.Flag {
		return XunfeiVoiceTrainResult{}, fmt.Errorf("xunfei_voiceclone: result code %d: %s", resp.Code, resp.Desc)
	}
	return XunfeiVoiceTrainResult{
		TaskID:      stringOr(resp.Data.TrainID, taskID),
		AssetID:     resp.Data.AssetID,
		TrainStatus: resp.Data.TrainStatus,
		FailedDesc:  resp.Data.FailedDesc,
	}, nil
}

func (c *XunfeiVoiceCloneClient) fetchToken(ctx context.Context) (string, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	body := buildXunfeiVoiceTokenBody(c.cfg.AppID, timestamp)
	var lastErr error
	retCodes := make([]string, 0, 3)
	for _, candidate := range xunfeiVoiceSignKeyCandidates(c.cfg) {
		token, retCode, err := c.fetchTokenWithBody(ctx, timestamp, body, candidate.value)
		if err == nil {
			c.trainingSignKey = candidate.value
			return token, nil
		}
		lastErr = err
		retCodes = append(retCodes, candidate.name+":"+stringOr(retCode, "unknown"))
		if retCode != "000007" {
			return "", err
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("%w；本地诊断：%s", lastErr, xunfeiVoiceTokenDebugInfo(c.cfg, body, retCodes))
	}
	return "", lastErr
}

func buildXunfeiVoiceTokenBody(appID, timestamp string) string {
	appID = strings.TrimSpace(appID)
	timestamp = strings.TrimSpace(timestamp)
	// 必须与官方 Python demo 的字符串拼接格式逐字节一致（无空格）：
	// 签名是对该 JSON 文本做字符串哈希而非语义比较，服务端按同一模板重建后比对，
	// 任何空格差异都会导致 000007 签名校验失败。
	return fmt.Sprintf(`{"base":{"appid":"%s","version":"v1","timestamp":"%s"},"model":"remote"}`, appID, timestamp)
}

type xunfeiVoiceSignKeyCandidate struct {
	name  string
	value string
}

func xunfeiVoiceSignKeyCandidates(cfg XunfeiVoiceCloneConfig) []xunfeiVoiceSignKeyCandidate {
	seen := map[string]bool{}
	add := func(candidates []xunfeiVoiceSignKeyCandidate, name, value string) []xunfeiVoiceSignKeyCandidate {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return candidates
		}
		seen[value] = true
		return append(candidates, xunfeiVoiceSignKeyCandidate{name: name, value: value})
	}

	candidates := make([]xunfeiVoiceSignKeyCandidate, 0, 3)
	candidates = add(candidates, "api_key", cfg.APIKey)
	candidates = add(candidates, "api_secret", cfg.APISecret)
	if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cfg.APISecret)); err == nil {
		candidates = add(candidates, "api_secret_base64", string(decoded))
	}
	return candidates
}

func xunfeiVoiceTokenDebugInfo(cfg XunfeiVoiceCloneConfig, body string, retCodes []string) string {
	return fmt.Sprintf(
		"app_id_len=%d, api_key_len=%d, api_secret_len=%d, token_body_sha256=%s, sign_modes=%s",
		len(strings.TrimSpace(cfg.AppID)),
		len(strings.TrimSpace(cfg.APIKey)),
		len(strings.TrimSpace(cfg.APISecret)),
		shortSHA256(body),
		strings.Join(retCodes, ","),
	)
}

func shortSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func (c *XunfeiVoiceCloneClient) fetchTokenWithBody(ctx context.Context, timestamp, body, signKey string) (string, string, error) {
	sign := xunfeiVoiceTokenSign(signKey, timestamp, body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xunfeiVoiceTokenURL, strings.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", sign)
	respBody, err := c.do(req)
	if err != nil {
		return "", "", err
	}
	var resp struct {
		RetCode     string `json:"retcode"`
		AccessToken string `json:"accesstoken"`
		ExpiresIn   string `json:"expiresin"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", "", fmt.Errorf("xunfei_voiceclone: parse token response: %w", err)
	}
	if resp.RetCode != "000000" || resp.AccessToken == "" {
		return "", resp.RetCode, fmt.Errorf("xunfei_voiceclone: token retcode %s: %s", resp.RetCode, xunfeiVoiceTokenRetCodeHint(resp.RetCode))
	}
	return resp.AccessToken, resp.RetCode, nil
}

func xunfeiVoiceTokenSign(signKey, timestamp, body string) string {
	keySign := md5Hex([]byte(signKey + timestamp))
	return md5Hex([]byte(keySign + body))
}

func xunfeiVoiceTokenRetCodeHint(retCode string) string {
	switch retCode {
	case "999999":
		return "讯飞服务内部异常，请稍后重试或提交工单"
	case "000004":
		return "请求参数缺失，请检查声音复刻 App ID 是否已保存；若刚修改过配置，请保存后重新测试"
	case "000006":
		return "请求参数格式异常，请检查本机系统时间是否正常"
	case "000007":
		return "签名校验失败，客户端会按官方 demo 的签名格式生成请求；若仍失败，请确认该应用已开通一句话复刻训练服务"
	default:
		return "未知鉴权错误，请检查声音复刻 App ID/API Key 是否正确且服务已开通"
	}
}

func (c *XunfeiVoiceCloneClient) createTask(ctx context.Context, token string) (string, error) {
	body := []byte(`{"engineVersion":"omni_v1","resourceType":12,"resourceName":"StealthCopilot Voice"}`)
	var resp struct {
		Code int    `json:"code"`
		Desc string `json:"desc"`
		Data string `json:"data"`
		Flag bool   `json:"flag"`
	}
	if err := c.postTrainingJSON(ctx, xunfeiVoiceTaskAddURL, token, body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 || !resp.Flag || resp.Data == "" {
		return "", fmt.Errorf("xunfei_voiceclone: create task code %d: %s", resp.Code, resp.Desc)
	}
	return resp.Data, nil
}

func (c *XunfeiVoiceCloneClient) postTrainingJSON(ctx context.Context, endpoint, token string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setTrainingHeaders(req, token, body)
	req.Header.Set("Content-Type", "application/json")
	respBody, err := c.do(req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("xunfei_voiceclone: parse response: %w", err)
	}
	return nil
}

func (c *XunfeiVoiceCloneClient) setTrainingHeaders(req *http.Request, token string, body []byte) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signKey := strings.TrimSpace(c.trainingSignKey)
	if signKey == "" {
		signKey = strings.TrimSpace(c.cfg.APIKey)
	}
	sign := md5Hex([]byte(signKey + timestamp + md5Hex(body)))
	req.Header.Set("X-Sign", sign)
	req.Header.Set("X-Token", token)
	req.Header.Set("X-AppId", strings.TrimSpace(c.cfg.AppID))
	req.Header.Set("X-Time", timestamp)
}

func (c *XunfeiVoiceCloneClient) do(req *http.Request) ([]byte, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xunfei_voiceclone: http: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xunfei_voiceclone: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func buildXunfeiVoiceCloneAuthURL(cfg XunfeiVoiceCloneConfig) (string, error) {
	parsed, err := url.Parse(xunfeiVoiceCloneWSURL)
	if err != nil {
		return "", err
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	signatureOrigin := strings.Join([]string{
		"host: " + parsed.Host,
		"date: " + date,
		"GET " + parsed.EscapedPath() + " HTTP/1.1",
	}, "\n")
	mac := hmac.New(sha256.New, []byte(cfg.APISecret))
	mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`, cfg.APIKey, signature)
	q := parsed.Query()
	q.Set("authorization", base64.StdEncoding.EncodeToString([]byte(authOrigin)))
	q.Set("date", date)
	q.Set("host", parsed.Host)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

func buildXunfeiVoiceCloneSynthesisRequest(appID, assetID, text string) xunfeiVoiceCloneSynthesisRequest {
	return xunfeiVoiceCloneSynthesisRequest{
		Header: xunfeiVoiceCloneSynthesisHeader{
			AppID:  appID,
			Status: 2,
			ResID:  assetID,
		},
		Parameter: xunfeiVoiceCloneSynthesisParameter{
			TTS: xunfeiVoiceCloneTTSParameter{
				VCN:      xunfeiVoiceCloneVCN,
				Volume:   50,
				Speed:    50,
				Pitch:    50,
				PyBuffer: 1,
				Audio: xunfeiVoiceCloneAudioParameter{
					Encoding:   "raw",
					SampleRate: 24000,
				},
			},
		},
		Payload: xunfeiVoiceCloneSynthesisPayload{
			Text: xunfeiVoiceCloneTextPayload{
				Encoding: "utf8",
				Compress: "raw",
				Format:   "plain",
				Status:   2,
				Seq:      0,
				Text:     base64.StdEncoding.EncodeToString([]byte(text)),
			},
		},
	}
}

func md5Hex(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func stringOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

type xunfeiCommonResponse struct {
	Code int    `json:"code"`
	Desc string `json:"desc"`
	Data any    `json:"data"`
	Flag bool   `json:"flag"`
}

type xunfeiTrainTextResponse struct {
	Code int    `json:"code"`
	Desc string `json:"desc"`
	Data struct {
		TextID   int64 `json:"textId"`
		TextSegs []struct {
			// 讯飞实际返回数字类型，用 json.Number 兼容字符串与数字两种格式
			SegID   json.Number `json:"segId"`
			SegText string      `json:"segText"`
		} `json:"textSegs"`
	} `json:"data"`
	Flag bool `json:"flag"`
}

type xunfeiTaskResultResponse struct {
	Code int    `json:"code"`
	Desc string `json:"desc"`
	Data struct {
		AssetID     string `json:"assetId"`
		TrainID     string `json:"trainId"`
		TrainStatus int    `json:"trainStatus"`
		FailedDesc  string `json:"failedDesc"`
	} `json:"data"`
	Flag bool `json:"flag"`
}

type xunfeiVoiceCloneSynthesisRequest struct {
	Header    xunfeiVoiceCloneSynthesisHeader    `json:"header"`
	Parameter xunfeiVoiceCloneSynthesisParameter `json:"parameter"`
	Payload   xunfeiVoiceCloneSynthesisPayload   `json:"payload"`
}

type xunfeiVoiceCloneSynthesisHeader struct {
	AppID  string `json:"app_id"`
	Status int    `json:"status"`
	ResID  string `json:"res_id"`
}

type xunfeiVoiceCloneSynthesisParameter struct {
	TTS xunfeiVoiceCloneTTSParameter `json:"tts"`
}

type xunfeiVoiceCloneTTSParameter struct {
	VCN      string                         `json:"vcn"`
	Volume   int                            `json:"volume"`
	Speed    int                            `json:"speed"`
	Pitch    int                            `json:"pitch"`
	PyBuffer int                            `json:"pybuffer"`
	Audio    xunfeiVoiceCloneAudioParameter `json:"audio"`
}

type xunfeiVoiceCloneAudioParameter struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
}

type xunfeiVoiceCloneSynthesisPayload struct {
	Text xunfeiVoiceCloneTextPayload `json:"text"`
}

type xunfeiVoiceCloneTextPayload struct {
	Encoding string `json:"encoding"`
	Compress string `json:"compress"`
	Format   string `json:"format"`
	Status   int    `json:"status"`
	Seq      int    `json:"seq"`
	Text     string `json:"text"`
}

type xunfeiVoiceCloneSynthesisResponse struct {
	Header struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  int    `json:"status"`
	} `json:"header"`
	Payload struct {
		Audio struct {
			Audio  string `json:"audio"`
			Status int    `json:"status"`
		} `json:"audio"`
	} `json:"payload"`
}
