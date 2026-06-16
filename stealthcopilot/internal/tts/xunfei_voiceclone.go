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
	text = strings.TrimSpace(text)
	if text == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	if !XunfeiVoiceCloneConfigReady(p.cfg) {
		return nil, fmt.Errorf("xunfei_voiceclone: AppID/API Key/API Secret/AssetID 未完整配置")
	}
	authURL, err := buildXunfeiVoiceCloneAuthURL(p.cfg)
	if err != nil {
		return nil, err
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, authURL, nil)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("xunfei_voiceclone: websocket status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("xunfei_voiceclone: websocket: %w", err)
	}
	if err := conn.WriteJSON(buildXunfeiVoiceCloneSynthesisRequest(p.cfg.AppID, p.cfg.AssetID, text)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("xunfei_voiceclone: send synthesis request: %w", err)
	}

	ch := make(chan []byte, 16)
	go func() {
		defer close(ch)
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg xunfeiVoiceCloneSynthesisResponse
			if err := json.Unmarshal(data, &msg); err != nil {
				return
			}
			if msg.Header.Code != 0 {
				return
			}
			if msg.Payload.Audio.Audio != "" {
				audio, err := base64.StdEncoding.DecodeString(msg.Payload.Audio.Audio)
				if err == nil && len(audio) > 0 {
					select {
					case ch <- audio:
					case <-ctx.Done():
						return
					}
				}
			}
			if msg.Header.Status == 2 || msg.Payload.Audio.Status == 2 {
				return
			}
		}
	}()
	return ch, nil
}

func (p *XunfeiVoiceCloneProvider) VoiceID() string { return p.cfg.AssetID }

func (p *XunfeiVoiceCloneProvider) Close() error { return nil }

type XunfeiVoiceCloneClient struct {
	cfg    XunfeiVoiceCloneConfig
	client *http.Client
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
				TextSegID: stringOr(seg.SegID, xunfeiVoiceDefaultSegID),
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
	body := fmt.Sprintf(`{"base":{"appid":"%s","version":"v1","timestamp":"%s"},"model":"remote"}`, c.cfg.AppID, timestamp)
	keySign := md5Hex([]byte(c.cfg.APIKey + timestamp))
	sign := md5Hex([]byte(keySign + body))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xunfeiVoiceTokenURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", sign)
	respBody, err := c.do(req)
	if err != nil {
		return "", err
	}
	var resp struct {
		RetCode     string `json:"retcode"`
		AccessToken string `json:"accesstoken"`
		ExpiresIn   string `json:"expiresin"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("xunfei_voiceclone: parse token response: %w", err)
	}
	if resp.RetCode != "000000" || resp.AccessToken == "" {
		return "", fmt.Errorf("xunfei_voiceclone: token retcode %s", resp.RetCode)
	}
	return resp.AccessToken, nil
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
	sign := md5Hex([]byte(c.cfg.APIKey + timestamp + md5Hex(body)))
	req.Header.Set("X-Sign", sign)
	req.Header.Set("X-Token", token)
	req.Header.Set("X-AppId", c.cfg.AppID)
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
			SegID   string `json:"segId"`
			SegText string `json:"segText"`
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
