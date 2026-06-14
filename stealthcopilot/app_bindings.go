package main

// app_bindings.go 将各内部服务的方法通过 App 代理暴露给 Wails 前端。
// Wails 只识别 App 上的公开方法，服务的具体方法在此处转发。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/config"
	"github.com/zhaoyta/stealthcopilot/internal/hearing"
	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/rag"
	"github.com/zhaoyta/stealthcopilot/internal/resume"
	"github.com/zhaoyta/stealthcopilot/internal/speaking"
	"github.com/zhaoyta/stealthcopilot/internal/system"
	"github.com/zhaoyta/stealthcopilot/internal/translation"
	"github.com/zhaoyta/stealthcopilot/internal/tts"
	"github.com/zhaoyta/stealthcopilot/internal/ui"
	"github.com/zhaoyta/stealthcopilot/internal/video"
)

const (
	teleprompterWindowWidth  = 400
	teleprompterWindowHeight = 300
	mainWindowWidth          = 1024
	mainWindowHeight         = 768
	apiConnectionTimeout     = 2 * time.Second
)

// APIConnectionResult is returned to the settings panel after probing a provider.
type APIConnectionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ===== Config 相关绑定 =====

// GetConfig 返回当前配置的前端视图。
func (a *App) GetConfig() config.FrontendConfig {
	return a.ConfigSvc.GetConfig()
}

// SaveAPIKey 将单个 API Key 写入 Keychain。
func (a *App) SaveAPIKey(req config.SaveAPIKeyRequest) string {
	return a.ConfigSvc.SaveAPIKey(req)
}

// TestAPIConnection probes the configured credentials for a provider.
func (a *App) TestAPIConnection(service string) APIConnectionResult {
	cfg := a.ConfigSvc.InternalManager().Config
	client := &http.Client{Timeout: apiConnectionTimeout}

	switch service {
	case "deepseek":
		if cfg.DeepSeekKey == "" {
			return APIConnectionResult{Message: "DeepSeek API Key 未配置"}
		}
		req, _ := http.NewRequest(http.MethodGet, "https://api.deepseek.com/models", nil)
		req.Header.Set("Authorization", "Bearer "+cfg.DeepSeekKey)
		return probeHTTP(client, req)
	case "elevenlabs":
		if cfg.ElevenLabsKey == "" {
			return APIConnectionResult{Message: "ElevenLabs API Key 未配置"}
		}
		req, _ := http.NewRequest(http.MethodGet, "https://api.elevenlabs.io/v1/user", nil)
		req.Header.Set("xi-api-key", cfg.ElevenLabsKey)
		return probeHTTP(client, req)
	case "simli":
		if cfg.SimliKey == "" {
			return APIConnectionResult{Message: "Simli API Key 未配置"}
		}
		req, _ := http.NewRequest(http.MethodGet, "https://api.simli.ai", nil)
		req.Header.Set("Authorization", "Bearer "+cfg.SimliKey)
		result := probeHTTP(client, req)
		if result.OK || result.Message == "HTTP 404" {
			return APIConnectionResult{OK: true, Message: "Simli API Key 已配置"}
		}
		return result
	case "xunfei":
		if cfg.XunfeiAppID == "" || cfg.XunfeiAPIKey == "" || cfg.XunfeiAPISecret == "" {
			return APIConnectionResult{Message: "讯飞 AppID/API Key/API Secret 未完整配置"}
		}
		return APIConnectionResult{OK: true, Message: "讯飞凭证已配置，启动管道时会进行 WebSocket 鉴权"}
	default:
		return APIConnectionResult{Message: "未知服务：" + service}
	}
}

// SaveLocalConfig 保存非敏感配置（语言、设备、外观等）。
func (a *App) SaveLocalConfig(req config.SaveLocalConfigRequest) string {
	return a.ConfigSvc.SaveLocalConfig(req)
}

// MarkSetupComplete 标记初始化向导已完成。
func (a *App) MarkSetupComplete() string {
	return a.ConfigSvc.MarkSetupComplete()
}

// CloneVoice uploads a recorded sample to ElevenLabs and stores the returned Voice ID.
func (a *App) CloneVoice(audioBytes []byte) string {
	cfg := a.ConfigSvc.InternalManager().Config
	if cfg.ElevenLabsKey == "" {
		return "ElevenLabs API Key 未配置"
	}
	if len(audioBytes) == 0 {
		return "录音为空，请重新录制"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", fmt.Sprintf("StealthCopilot-%d", time.Now().Unix()))
	part, err := writer.CreateFormFile("files", "voice.webm")
	if err != nil {
		return err.Error()
	}
	if _, err := part.Write(audioBytes); err != nil {
		return err.Error()
	}
	if err := writer.Close(); err != nil {
		return err.Error()
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.elevenlabs.io/v1/voices/add", &body)
	if err != nil {
		return err.Error()
	}
	req.Header.Set("xi-api-key", cfg.ElevenLabsKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Sprintf("ElevenLabs 克隆失败：HTTP %d %s", resp.StatusCode, string(respBody))
	}

	var payload struct {
		VoiceID string `json:"voice_id"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return "ElevenLabs 返回解析失败：" + err.Error()
	}
	if payload.VoiceID == "" {
		return "ElevenLabs 未返回 Voice ID"
	}

	return a.ConfigSvc.SaveAPIKey(config.SaveAPIKeyRequest{
		Service: "elevenlabs",
		Field:   "voice_id",
		Value:   payload.VoiceID,
	})
}

// GetDefaultPrompts 返回 Go 后端硬编码的默认 Prompt 值。
func (a *App) GetDefaultPrompts() config.DefaultPromptsResponse {
	return a.ConfigSvc.GetDefaultPrompts()
}

// ===== Resume 相关绑定 =====

// ListResumes 返回所有简历列表。
func (a *App) ListResumes() []resume.FrontendResume {
	return a.ResumeSvc.ListResumes()
}

// UploadResume 从文件路径上传简历（Wails 文件对话框返回的路径）。
func (a *App) UploadResume(path string) string {
	return a.ResumeSvc.UploadResume(path)
}

// DeleteResume 删除指定简历。
func (a *App) DeleteResume(id string) string {
	return a.ResumeSvc.DeleteResume(id)
}

// SetActiveResume 将指定简历设为激活。
func (a *App) SetActiveResume(id string) string {
	return a.ResumeSvc.SetActiveResume(id)
}

// IsEmbeddingReady 检查 embedding 服务是否就绪（用于 Setup Wizard 依赖检测）。
func (a *App) IsEmbeddingReady() bool {
	return a.ResumeSvc.IsEmbeddingReady()
}

// ===== System 相关绑定 =====

// CheckDeps 检测系统依赖（虚拟声卡、虚拟摄像头）。
func (a *App) CheckDeps() system.DepsReport {
	return a.SystemSvc.CheckDeps()
}

// EnumerateDevices 实时枚举系统音视频设备。
func (a *App) EnumerateDevices() system.DeviceList {
	return a.SystemSvc.EnumerateDevices()
}

// PickResumeFile 弹出系统文件选择对话框，返回用户选择的文件路径。
// 限制格式为 PDF 和 Word（.docx）。未选择时返回空字符串。
func (a *App) PickResumeFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择简历文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "简历（PDF / Word）", Pattern: "*.pdf;*.docx"},
		},
	})
}

// InstallDep 引导安装指定系统依赖（虚拟声卡或虚拟摄像头）。
// macOS: 虚拟声卡优先通过 Homebrew 在 Terminal 中安装；其余依赖打开官方下载页。
// Windows: 在浏览器中打开对应的官方下载页。
func (a *App) InstallDep(key string) system.DepInstallResult {
	return a.SystemSvc.InstallDep(key)
}

// ===== Ghost Window 相关绑定 =====

// ShowTeleprompter 请求前端显示提词窗。
func (a *App) ShowTeleprompter() string {
	a.teleprompterMu.Lock()
	useNative := a.TeleprompterWindow != nil && a.TeleprompterWindow.Available()
	if !a.teleprompterVisible && !useNative {
		width, height := runtime.WindowGetSize(a.ctx)
		x, y := runtime.WindowGetPosition(a.ctx)
		a.teleprompterWindow = windowSnapshot{
			Width:  width,
			Height: height,
			X:      x,
			Y:      y,
			Saved:  width > 0 && height > 0,
		}
	}
	a.teleprompterVisible = true
	a.teleprompterMu.Unlock()

	if useNative {
		if err := a.TeleprompterWindow.Show(); err != nil {
			return err.Error()
		}
		return ""
	}

	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetSize(a.ctx, teleprompterWindowWidth, teleprompterWindowHeight)
	runtime.WindowCenter(a.ctx)
	runtime.WindowSetBackgroundColour(a.ctx, 0, 0, 0, 0)
	runtime.EventsEmit(a.ctx, "teleprompter:show")
	return ""
}

// HideTeleprompter 请求前端隐藏提词窗。
func (a *App) HideTeleprompter() string {
	a.teleprompterMu.Lock()
	snapshot := a.teleprompterWindow
	useNative := a.TeleprompterWindow != nil && a.TeleprompterWindow.Available()
	a.teleprompterWindow = windowSnapshot{}
	a.teleprompterVisible = false
	a.teleprompterMu.Unlock()

	if useNative {
		if err := a.TeleprompterWindow.Hide(); err != nil {
			return err.Error()
		}
		runtime.EventsEmit(a.ctx, "teleprompter:hide")
		return ""
	}

	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	if snapshot.Saved {
		runtime.WindowSetSize(a.ctx, snapshot.Width, snapshot.Height)
		runtime.WindowSetPosition(a.ctx, snapshot.X, snapshot.Y)
	} else {
		runtime.WindowSetSize(a.ctx, mainWindowWidth, mainWindowHeight)
		runtime.WindowCenter(a.ctx)
	}
	runtime.WindowSetBackgroundColour(a.ctx, 27, 38, 54, 1)
	runtime.EventsEmit(a.ctx, "teleprompter:hide")
	return ""
}

// IsTeleprompterVisible 返回提词窗当前显示状态。
func (a *App) IsTeleprompterVisible() bool {
	a.teleprompterMu.RLock()
	defer a.teleprompterMu.RUnlock()
	return a.teleprompterVisible
}

// ApplyStealthToWindowHandle 为后续接入 Wails 原生句柄预留 hook 入口。
func (a *App) ApplyStealthToWindowHandle(handle uintptr) string {
	status, err := ui.ApplyStealthToHandle(ui.WindowHandle(handle))
	a.stealthStatus = status
	if err != nil {
		return err.Error()
	}
	if status == ui.StealthStatusUnsupported {
		return "当前系统版本不支持防录屏功能"
	}
	if status == ui.StealthStatusUnavailable {
		return "当前 Wails 运行时尚未提供原生窗口句柄"
	}
	return ""
}

// GetStealthStatus 返回最近一次原生 stealth hook 的应用状态。
func (a *App) GetStealthStatus() ui.StealthStatus {
	return a.stealthStatus
}

// ===== Hearing Chain 相关绑定 =====

// StartHearingChain 启动听力链管道：音频捕获 → 讯飞翻译 → 意图识别 → RAG → 回答生成。
// 配置从当前 ConfigSvc 读取；已在运行时先停止再重新启动。
// 返回空字符串表示成功，否则返回错误描述。
func (a *App) StartHearingChain() string {
	cfg := a.ConfigSvc.InternalManager().Config
	retriever := rag.NewRetriever(a.ResumeSvc.InternalManager())

	chainCfg := hearing.ChainConfig{
		Xunfei: translation.XunfeiConfig{
			AppID:      cfg.XunfeiAppID,
			APIKey:     cfg.XunfeiAPIKey,
			APISecret:  cfg.XunfeiAPISecret,
			SourceLang: cfg.HearingSourceLang,
			TargetLang: cfg.HearingTargetLang,
		},
		DeepSeekKey:      cfg.DeepSeekKey,
		DeepSeekModel:    cfg.DeepSeekModel,
		RAGPrompt:        cfg.RAGPrompt,
		VirtualMicDevice: cfg.VirtualMicName,
		Retriever:        retriever,
		EventSink:        a.emitTeleprompterEvent,
	}
	return a.HearingChain.Start(a.ctx, chainCfg)
}

func (a *App) emitTeleprompterEvent(eventName string, data ...any) {
	if a.TeleprompterWindow == nil || !a.TeleprompterWindow.Available() {
		return
	}
	switch eventName {
	case hearing.EventSubtitle:
		if len(data) == 0 {
			return
		}
		switch v := data[0].(type) {
		case hearing.SubtitleEvent:
			a.TeleprompterWindow.AppendSubtitle(v.Text)
		case string:
			a.TeleprompterWindow.AppendSubtitle(v)
		}
	case llm.EventAnswerToken:
		if len(data) == 0 {
			return
		}
		if token, ok := data[0].(string); ok {
			a.TeleprompterWindow.AppendAnswerToken(token)
		}
	case llm.EventAnswerDone:
		a.TeleprompterWindow.FinishAnswer()
	}
}

// StopHearingChain 停止听力链，等待所有 goroutine 退出后返回。
func (a *App) StopHearingChain() {
	a.HearingChain.Stop()
}

// ===== Speaking Chain 相关绑定 =====

// StartSpeakingChain 启动说话链管道：麦克风捕获 → VAD → 讯飞翻译 → ElevenLabs TTS → 虚拟麦克风。
// 配置从当前 ConfigSvc 读取；已在运行时先停止再重新启动。
// 返回空字符串表示成功，否则返回错误描述。
func (a *App) StartSpeakingChain() string {
	cfg := a.ConfigSvc.InternalManager().Config
	chainCfg := speaking.ChainConfig{
		Xunfei: translation.XunfeiSpeakConfig{
			AppID:      cfg.XunfeiAppID,
			APIKey:     cfg.XunfeiAPIKey,
			APISecret:  cfg.XunfeiAPISecret,
			SourceLang: cfg.SpeakingInputLang,
			TargetLang: cfg.SpeakingOutputLang,
		},
		ElevenLabs: tts.ElevenLabsConfig{
			APIKey:  cfg.ElevenLabsKey,
			VoiceID: cfg.ElevenLabsVoiceID,
		},
		PhysicalMicDevice:  cfg.PhysicalMicName,
		VirtualMicDevice:   cfg.VirtualMicName,
		SilenceThresholdMs: 800,
		AudioSink:          a.VideoChain.SendAudioChunk,
	}
	return a.SpeakingChain.Start(a.ctx, chainCfg)
}

// StopSpeakingChain 停止说话链，等待所有 goroutine 退出后返回。
func (a *App) StopSpeakingChain() {
	a.SpeakingChain.Stop()
}

// SetVADSilenceThreshold 运行时更新 VAD 静音阈值（毫秒），即时生效，无需重启说话链。
func (a *App) SetVADSilenceThreshold(ms int) {
	a.SpeakingChain.SetSilenceThreshold(ms)
}

// ===== Video Chain 相关绑定 =====

// StartVideoChain 启动视频链管道：摄像头捕获 → Simli 口型同步 → A/V 对齐 → 虚拟摄像头。
// 配置从当前 ConfigSvc 读取；已在运行时先停止再重新启动。
// 返回空字符串表示成功，否则返回错误描述。
func (a *App) StartVideoChain() string {
	cfg := a.ConfigSvc.InternalManager().Config
	chainCfg := video.ChainConfig{
		SimliAPIKey:        cfg.SimliKey,
		SilmiFaceID:        cfg.SimliFaceID,
		SimliHeartbeatAddr: "", // Phase 1 暂不配置 UDP 端点，由 Simli 文档确认后填入
		PhysicalCamDevice:  cfg.PhysicalCamName,
		VirtualCamDevice:   cfg.VirtualCamName,
	}
	return a.VideoChain.Start(a.ctx, chainCfg)
}

// StopVideoChain 停止视频链，等待所有 goroutine 退出后返回。
func (a *App) StopVideoChain() {
	a.VideoChain.Stop()
}

// IsCircuitOpen 返回熔断器当前是否处于 Open（直通）状态，供前端显示警告条。
func (a *App) IsCircuitOpen() bool {
	return a.VideoChain.IsCircuitOpen()
}

// EnsureVirtualCameraDriver 检测并尝试安装虚拟摄像头驱动。
// bundledDriverPath 为 App bundle 内的驱动文件路径（由前端传入）。
// 返回空字符串表示成功或已安装，否则返回提示信息。
func (a *App) EnsureVirtualCameraDriver(bundledDriverPath string) string {
	result := video.EnsureDriver(bundledDriverPath)
	return result.Message
}

// TripCircuit 手动触发熔断（用户点击幽灵提词窗"紧急降级"按钮时调用）。
func (a *App) TripCircuit() {
	a.VideoChain.TripCircuit()
}

// CheckVirtualCameraDriver 检测虚拟摄像头驱动注册状态（不执行安装）。
func (a *App) CheckVirtualCameraDriver() string {
	status := video.CheckDriverStatus()
	switch status {
	case video.DriverStatusRegistered:
		return "registered"
	case video.DriverStatusNotRegistered:
		return "not_registered"
	case video.DriverStatusUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// probeHTTP 先用 HEAD 请求探测（不消耗响应体/API 配额），
// 服务端返回 405 Method Not Allowed 时降级为原始方法（GET/POST）。
func probeHTTP(client *http.Client, req *http.Request) APIConnectionResult {
	headReq, _ := http.NewRequestWithContext(req.Context(), http.MethodHead, req.URL.String(), nil)
	headReq.Header = req.Header.Clone()
	if resp, err := client.Do(headReq); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				return APIConnectionResult{OK: true, Message: "连接成功"}
			}
			return APIConnectionResult{Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
		}
	}
	// HEAD 不支持或请求失败，回退到原始方法
	resp, err := client.Do(req)
	if err != nil {
		return APIConnectionResult{Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return APIConnectionResult{OK: true, Message: "连接成功"}
	}
	return APIConnectionResult{Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}
