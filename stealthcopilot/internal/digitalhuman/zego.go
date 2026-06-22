// Package digitalhuman drives cloud digital-human video output from synthesized PCM audio.
package digitalhuman

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/video"
)

const (
	defaultZegoAPIEndpoint = "https://aigc-digitalhuman-api.zegotech.cn/"
	signatureVer           = "2.0"
)

type Config struct {
	AppID          string
	ServerSecret   string
	DigitalHumanID string
	RoomID         string
	StreamID       string
	Endpoint       string
	HTTPClient     *http.Client
	PullClient     PullClient
	// RTMPPullURL 是 ZEGO CDN 混流转推拉流地址（非空时自动创建 FFmpegRTMPPullClient）。
	RTMPPullURL string
	// VirtualCameraWriter 接收拉取到的数字人视频帧并写入本机虚拟摄像头；nil 时跳过视频输出。
	VirtualCameraWriter video.VirtualCameraWriter
}

type Driver interface {
	// Start 启动数字人输出管道。audioSink 接收从数字人云端拉取到的 PCM 块（仅 ZEGO 使用）。
	Start(ctx context.Context, audioSink func([]byte)) error
	SendAudio(chunk []byte) error
	Close() error
	// SuppressDirectAudio 返回 true 时，说话链不将 TTS 音频直接写入虚拟麦克风，
	// 由数字人云端负责生成并回传音频（如 ZEGO）；
	// 返回 false 时，TTS 音频直接写入虚拟麦克风，驱动仅处理视频（如 Simli）。
	SuppressDirectAudio() bool
}

type PullClient interface {
	Start(ctx context.Context, cfg PullConfig, audioSink func([]byte), videoSink func(VideoFrame)) error
	Close() error
}

type PullConfig struct {
	AppID    string
	RoomID   string
	StreamID string
}

type VideoFrame struct {
	Data []byte
	PTS  int64
}

type ZegoDriver struct {
	cfg    Config
	client *Client

	mu     sync.Mutex
	conn   *websocket.Conn
	taskID string
	cancel context.CancelFunc
	pull   PullClient
}

func NewZegoDriver(cfg Config) *ZegoDriver {
	return &ZegoDriver{cfg: cfg, client: NewClient(cfg)}
}

// ConfigReady 检查启动数字人输出所需的最少配置是否已填写。
// RoomID 和 StreamID 不在此检查范围内——两者会在 Start() 中自动生成。
func ConfigReady(cfg Config) bool {
	return strings.TrimSpace(cfg.AppID) != "" &&
		strings.TrimSpace(cfg.ServerSecret) != "" &&
		strings.TrimSpace(cfg.DigitalHumanID) != ""
}

// Start 启动即构数字人输出管道，包括创建云端任务、建立 WebSocket 驱动连接、启动本地拉流。
// audioSink 由说话链传入，接收从 ZEGO RTC 拉取的数字人音频 PCM；可为 nil（跳过音频输出）。
func (d *ZegoDriver) Start(ctx context.Context, audioSink func([]byte)) error {
	if !ConfigReady(d.cfg) {
		return errors.New("即构数字人配置不完整：请配置 AppID、ServerSecret、数字人 ID、Room ID 和 Stream ID")
	}
	ctx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	// RoomID / StreamID 未配置时自动生成，保证每次会话唯一。
	roomID := strings.TrimSpace(d.cfg.RoomID)
	streamID := strings.TrimSpace(d.cfg.StreamID)
	if roomID == "" {
		nonce, _ := randomNonce()
		roomID = "sc_room_" + nonce
	}
	if streamID == "" {
		nonce, _ := randomNonce()
		streamID = "sc_stream_" + nonce
	}
	diag.Infof("digitalhuman session room_id=%s stream_id=%s", roomID, streamID)

	taskID, err := d.client.CreateStreamTask(ctx, CreateStreamTaskRequest{
		DigitalHumanConfig: DigitalHumanConfig{DigitalHumanID: d.cfg.DigitalHumanID},
		RTCConfig:          RTCConfig{RoomID: roomID, StreamID: streamID},
	})
	if err != nil {
		cancel()
		return err
	}
	drive, err := d.client.DriveByWSStream(ctx, taskID)
	if err != nil {
		cancel()
		_ = d.client.StopStreamTask(context.Background(), taskID)
		return err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, drive.WssAddress, nil)
	if err != nil {
		cancel()
		_ = d.client.StopStreamTask(context.Background(), taskID)
		return fmt.Errorf("连接即构数字人 WebSocket 失败：%w", err)
	}

	// 选择拉流客户端：优先使用注入的 PullClient，否则按配置自动创建 FFmpegRTMPPullClient。
	pull := d.cfg.PullClient
	if pull == nil && strings.TrimSpace(d.cfg.RTMPPullURL) != "" {
		pull = &FFmpegRTMPPullClient{Address: d.cfg.RTMPPullURL}
	}
	if pull == nil {
		pull = UnsupportedPullClient{}
	}

	// 构建视频 sink：将拉取到的 BGRA 帧写入虚拟摄像头（未配置时跳过）。
	var videoSink func(VideoFrame)
	if d.cfg.VirtualCameraWriter != nil {
		cw := d.cfg.VirtualCameraWriter
		videoSink = func(frame VideoFrame) {
			_ = cw.WriteFrame(video.Frame{Data: frame.Data, PTS: frame.PTS})
		}
	}

	pullCfg := PullConfig{AppID: d.cfg.AppID, RoomID: d.cfg.RoomID, StreamID: d.cfg.StreamID}
	if err := pull.Start(ctx, pullCfg, audioSink, videoSink); err != nil {
		_ = conn.Close()
		cancel()
		_ = d.client.StopStreamTask(context.Background(), taskID)
		return fmt.Errorf("启动即构数字人 RTC 拉流失败：%w", err)
	}

	d.mu.Lock()
	d.taskID = taskID
	d.conn = conn
	d.pull = pull
	d.mu.Unlock()
	return nil
}

func (d *ZegoDriver) SendAudio(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return errors.New("即构数字人 WebSocket 未连接")
	}
	return conn.WriteMessage(websocket.BinaryMessage, chunk)
}

func (d *ZegoDriver) Close() error {
	if d.cancel != nil {
		d.cancel()
	}
	d.mu.Lock()
	conn := d.conn
	taskID := d.taskID
	pull := d.pull
	d.conn = nil
	d.taskID = ""
	d.pull = nil
	d.mu.Unlock()
	if pull != nil {
		_ = pull.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if d.cfg.VirtualCameraWriter != nil {
		_ = d.cfg.VirtualCameraWriter.Close()
	}
	if taskID != "" {
		return d.client.StopStreamTask(context.Background(), taskID)
	}
	return nil
}

type Client struct {
	appID        string
	serverSecret string
	endpoint     string
	httpClient   *http.Client
}

func NewClient(cfg Config) *Client {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		appID:        strings.TrimSpace(cfg.AppID),
		serverSecret: strings.TrimSpace(cfg.ServerSecret),
		endpoint:     strings.TrimSpace(cfg.Endpoint),
		httpClient:   client,
	}
}

type DigitalHumanConfig struct {
	DigitalHumanID  string `json:"DigitalHumanId"`
	BackgroundColor string `json:"BackgroundColor,omitempty"`
}

type RTCConfig struct {
	RoomID   string `json:"RoomId"`
	StreamID string `json:"StreamId"`
}

type CreateStreamTaskRequest struct {
	DigitalHumanConfig DigitalHumanConfig `json:"DigitalHumanConfig"`
	RTCConfig          RTCConfig          `json:"RTCConfig"`
	TTL                int                `json:"TTL,omitempty"`
	MaxIdleTime        int                `json:"MaxIdleTime,omitempty"`
}

type CreateStreamTaskData struct {
	TaskID string `json:"TaskId"`
}

type DriveByWSStreamData struct {
	DriveID    string `json:"DriveId"`
	WssAddress string `json:"WssAddress"`
}

type apiResponse[T any] struct {
	Code      int    `json:"Code"`
	Message   string `json:"Message"`
	RequestID string `json:"RequestId"`
	Data      T      `json:"Data"`
}

func (c *Client) CreateStreamTask(ctx context.Context, body CreateStreamTaskRequest) (string, error) {
	var resp apiResponse[CreateStreamTaskData]
	if err := c.doPOST(ctx, "CreateDigitalHumanStreamTask", body, &resp); err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.Data.TaskID) == "" {
		return "", errors.New("即构未返回数字人视频流任务 ID")
	}
	return resp.Data.TaskID, nil
}

func (c *Client) DriveByWSStream(ctx context.Context, taskID string) (DriveByWSStreamData, error) {
	var resp apiResponse[DriveByWSStreamData]
	if err := c.doPOST(ctx, "DriveByWsStream", map[string]string{"TaskId": taskID}, &resp); err != nil {
		return DriveByWSStreamData{}, err
	}
	if strings.TrimSpace(resp.Data.WssAddress) == "" {
		return DriveByWSStreamData{}, errors.New("即构未返回数字人 WebSocket 地址")
	}
	return resp.Data, nil
}

func (c *Client) StopStreamTask(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	var resp apiResponse[map[string]any]
	return c.doPOST(ctx, "StopDigitalHumanStreamTask", map[string]string{"TaskId": taskID}, &resp)
}

func (c *Client) Probe(ctx context.Context) error {
	var resp apiResponse[map[string]any]
	err := c.doPOST(ctx, "DescribeDigitalHuman", map[string]string{}, &resp)
	if err == nil {
		return nil
	}
	return err
}

func (c *Client) doPOST(ctx context.Context, action string, body any, out any) error {
	if strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.serverSecret) == "" {
		return errors.New("即构 AppID 或 ServerSecret 未配置")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	reqURL, err := c.signedURL(action)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("即构 API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	if apiErr := responseError(out); apiErr != nil {
		return apiErr
	}
	return nil
}

func (c *Client) signedURL(action string) (string, error) {
	appID, err := strconv.ParseUint(c.appID, 10, 32)
	if err != nil {
		return "", errors.New("即构 AppID 必须是数字")
	}
	nonce, err := randomNonce()
	if err != nil {
		return "", err
	}
	timestamp := time.Now().Unix()
	values := url.Values{}
	values.Set("Action", action)
	values.Set("AppId", strconv.FormatUint(appID, 10))
	values.Set("SignatureNonce", nonce)
	values.Set("Timestamp", strconv.FormatInt(timestamp, 10))
	values.Set("SignatureVersion", signatureVer)
	values.Set("Signature", GenerateSignature(uint32(appID), nonce, c.serverSecret, timestamp))
	endpoint := c.endpoint
	if endpoint == "" {
		endpoint = defaultZegoAPIEndpoint
	}
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	return endpoint + "?" + values.Encode(), nil
}

func GenerateSignature(appID uint32, nonce string, serverSecret string, timestamp int64) string {
	raw := fmt.Sprintf("%d%s%s%d", appID, nonce, serverSecret, timestamp)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomNonce() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func responseError(out any) error {
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	var envelope struct {
		Code      int    `json:"Code"`
		Message   string `json:"Message"`
		RequestID string `json:"RequestId"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Code == 0 {
		return nil
	}
	msg := strings.TrimSpace(envelope.Message)
	if msg == "" {
		msg = "unknown"
	}
	return fmt.Errorf("即构 API 返回错误 code=%d request_id=%s message=%s", envelope.Code, envelope.RequestID, msg)
}

// SuppressDirectAudio 返回 true：ZEGO 自己生成并返回数字人音频，不需要本地 TTS 直接输出。
func (d *ZegoDriver) SuppressDirectAudio() bool { return true }

type NullDriver struct{}

func (NullDriver) Start(context.Context, func([]byte)) error { return nil }
func (NullDriver) SendAudio([]byte) error                    { return nil }
func (NullDriver) Close() error                              { return nil }
func (NullDriver) SuppressDirectAudio() bool                 { return false }

type UnsupportedPullClient struct{}

func (UnsupportedPullClient) Start(context.Context, PullConfig, func([]byte), func(VideoFrame)) error {
	return errors.New("当前版本尚未集成 ZEGO RTC 本机拉流 SDK，无法把数字人音视频写入本机虚拟设备")
}

func (UnsupportedPullClient) Close() error { return nil }

type NullPullClient struct{}

func (NullPullClient) Start(context.Context, PullConfig, func([]byte), func(VideoFrame)) error {
	return nil
}

func (NullPullClient) Close() error { return nil }
