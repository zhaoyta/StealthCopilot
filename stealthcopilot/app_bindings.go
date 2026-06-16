package main

// app_bindings.go 将各内部服务的方法通过 App 代理暴露给 Wails 前端。
// Wails 只识别 App 上的公开方法，服务的具体方法在此处转发。

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/circuit"
	"github.com/zhaoyta/stealthcopilot/internal/config"
	"github.com/zhaoyta/stealthcopilot/internal/hearing"
	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
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
	apiConnectionTimeout     = 5 * time.Second
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
		modelsURL := strings.TrimRight(cfg.LLMBaseURL, "/") + "/models"
		req, _ := http.NewRequest(http.MethodGet, modelsURL, nil)
		req.Header.Set("Authorization", "Bearer "+cfg.DeepSeekKey)
		return probeHTTP(client, req)
	case "xunfei_tts":
		if cfg.XunfeiTTSAppID == "" || cfg.XunfeiTTSAPIKey == "" || cfg.XunfeiTTSAPISecret == "" {
			return APIConnectionResult{Message: "讯飞声音复刻 AppID/API Key/API Secret 未完整配置"}
		}
		ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
		defer cancel()
		_, err := tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).FetchTrainText(ctx)
		if err != nil {
			return APIConnectionResult{Message: "讯飞声音复刻训练接口测试失败：" + err.Error()}
		}
		if cfg.XunfeiTTSAssetID == "" {
			return APIConnectionResult{OK: true, Message: "讯飞声音复刻凭证可用，尚未完成音色训练"}
		}
		return APIConnectionResult{OK: true, Message: "讯飞声音复刻凭证可用，Asset ID 已配置"}
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
		if cfg.XunfeiRTASRAppID == "" || cfg.XunfeiRTASRAPIKey == "" {
			return APIConnectionResult{Message: "讯飞 RTASR AppID/API Key 未完整配置"}
		}
		if cfg.XunfeiMTAppID == "" || cfg.XunfeiMTAPIKey == "" || cfg.XunfeiMTAPISecret == "" {
			return probeXunfeiRTASR(cfg)
		}
		rtasrResult := probeXunfeiRTASR(cfg)
		if !rtasrResult.OK {
			return rtasrResult
		}
		mtResult := probeXunfeiMT(cfg)
		if !mtResult.OK {
			return mtResult
		}
		return APIConnectionResult{OK: true, Message: "讯飞 RTASR WebSocket 握手成功，机器翻译 v1/v2 HTTP 请求成功"}
	case "xunfei_rtasr":
		if cfg.XunfeiRTASRAppID == "" || cfg.XunfeiRTASRAPIKey == "" {
			return APIConnectionResult{Message: "讯飞 RTASR AppID/API Key 未完整配置"}
		}
		return probeXunfeiRTASR(cfg)
	case "xunfei_mt":
		if cfg.XunfeiMTAppID == "" || cfg.XunfeiMTAPIKey == "" || cfg.XunfeiMTAPISecret == "" {
			return APIConnectionResult{Message: "讯飞机器翻译 AppID/API Key/API Secret 未完整配置"}
		}
		return probeXunfeiMT(cfg)
	default:
		return APIConnectionResult{Message: "未知服务：" + service}
	}
}

func probeXunfeiRTASR(cfg *config.AppConfig) APIConnectionResult {
	ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
	defer cancel()
	err := translation.ProbeXunfeiRTASRConnection(ctx, translation.XunfeiConfig{
		AppID:      cfg.XunfeiRTASRAppID,
		APIKey:     cfg.XunfeiRTASRAPIKey,
		SourceLang: cfg.HearingSourceLang,
		TargetLang: cfg.HearingTargetLang,
	})
	if err != nil {
		return APIConnectionResult{Message: "讯飞 RTASR WebSocket 握手失败：" + err.Error()}
	}
	return APIConnectionResult{OK: true, Message: "讯飞 RTASR WebSocket 握手成功"}
}

func probeXunfeiMT(cfg *config.AppConfig) APIConnectionResult {
	ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
	defer cancel()
	err := translation.ProbeXunfeiMachineTranslationConnection(ctx, translation.XunfeiMachineTranslationConfig{
		AppID:     cfg.XunfeiMTAppID,
		APIKey:    cfg.XunfeiMTAPIKey,
		APISecret: cfg.XunfeiMTAPISecret,
	})
	if err != nil {
		if strings.Contains(err.Error(), "apikey not found") {
			detail := fmt.Sprintf("当前保存：API Key %s；API Secret %s", maskedSecretHint(cfg.XunfeiMTAPIKey), maskedSecretHint(cfg.XunfeiMTAPISecret))
			if looksLikeXunfeiSecret(cfg.XunfeiMTAPIKey) {
				return APIConnectionResult{Message: "讯飞机器翻译 API Key 未被接口识别。" + detail + "。API Key 当前值看起来更像 API Secret；请重新保存控制台 APIKey 行的值"}
			}
			return APIConnectionResult{Message: "讯飞机器翻译 API Key 未被接口识别。" + detail + "。请确认这是机器翻译服务页的 API Key，且保存后重新点击测试"}
		}
		if strings.Contains(err.Error(), "HMAC signature does not match") {
			return APIConnectionResult{Message: "讯飞机器翻译签名不匹配：请检查机器翻译 API Secret 是否和 API Key 属于同一个服务/应用"}
		}
		return APIConnectionResult{Message: "讯飞机器翻译测试请求失败：" + err.Error()}
	}
	return APIConnectionResult{OK: true, Message: "讯飞机器翻译测试翻译成功"}
}

func xunfeiVoiceCloneConfigFromApp(cfg *config.AppConfig) tts.XunfeiVoiceCloneConfig {
	return tts.XunfeiVoiceCloneConfig{
		AppID:     cfg.XunfeiTTSAppID,
		APIKey:    cfg.XunfeiTTSAPIKey,
		APISecret: cfg.XunfeiTTSAPISecret,
		AssetID:   cfg.XunfeiTTSAssetID,
		TaskID:    cfg.XunfeiTTSTaskID,
	}
}

func maskedSecretHint(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "未保存"
	}
	if len(key) <= 8 {
		return fmt.Sprintf("长度 %d", len(key))
	}
	return fmt.Sprintf("长度 %d，前4位 %s，后4位 %s", len(key), key[:4], key[len(key)-4:])
}

func looksLikeXunfeiSecret(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 24 {
		return false
	}
	if strings.HasPrefix(value, "NW") || strings.HasPrefix(value, "MG") || strings.HasPrefix(value, "ZW") {
		return true
	}
	return strings.ContainsAny(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") && strings.ContainsAny(value, "abcdefghijklmnopqrstuvwxyz") && !strings.ContainsAny(value, "-_")
}

// SaveLocalConfig 保存非敏感配置（语言、设备、外观等）。
func (a *App) SaveLocalConfig(req config.SaveLocalConfigRequest) string {
	return a.ConfigSvc.SaveLocalConfig(req)
}

// MarkSetupComplete 标记初始化向导已完成。
func (a *App) MarkSetupComplete() string {
	return a.ConfigSvc.MarkSetupComplete()
}

// CloneVoice submits a recorded WAV sample to iFlytek VoiceClone and stores the task ID.
func (a *App) CloneVoice(audioBytes []byte) string {
	cfg := a.ConfigSvc.InternalManager().Config
	if cfg.XunfeiTTSAppID == "" || cfg.XunfeiTTSAPIKey == "" || cfg.XunfeiTTSAPISecret == "" {
		return "讯飞声音复刻 AppID/API Key/API Secret 未完整配置"
	}
	if len(audioBytes) == 0 {
		return "录音为空，请重新录制"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	taskID, err := tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).SubmitTrainingAudio(ctx, audioBytes)
	if err != nil {
		return err.Error()
	}
	return a.ConfigSvc.SaveAPIKey(config.SaveAPIKeyRequest{
		Service: "xunfei_tts",
		Field:   "task_id",
		Value:   taskID,
	})
}

// GetXunfeiVoiceTrainText returns the required reading text for iFlytek voice training.
func (a *App) GetXunfeiVoiceTrainText() (tts.XunfeiVoiceTrainText, error) {
	cfg := a.ConfigSvc.InternalManager().Config
	ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
	defer cancel()
	return tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).FetchTrainText(ctx)
}

// QueryXunfeiVoiceCloneStatus refreshes a submitted iFlytek voice training task.
func (a *App) QueryXunfeiVoiceCloneStatus() APIConnectionResult {
	cfg := a.ConfigSvc.InternalManager().Config
	ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
	defer cancel()
	result, err := tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).QueryTrainingResult(ctx, cfg.XunfeiTTSTaskID)
	if err != nil {
		return APIConnectionResult{Message: "讯飞声音复刻训练状态查询失败：" + err.Error()}
	}
	if result.AssetID != "" {
		if errMsg := a.ConfigSvc.SaveAPIKey(config.SaveAPIKeyRequest{Service: "xunfei_tts", Field: "asset_id", Value: result.AssetID}); errMsg != "" {
			return APIConnectionResult{Message: errMsg}
		}
	}
	switch result.TrainStatus {
	case 1:
		return APIConnectionResult{OK: true, Message: "讯飞声音复刻训练成功，Asset ID 已保存"}
	case -1, 2:
		return APIConnectionResult{Message: "讯飞声音复刻训练中，请稍后再查询"}
	default:
		return APIConnectionResult{Message: "讯飞声音复刻训练失败：" + result.FailedDesc}
	}
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
	if key == "virtual_cam" {
		result := video.EnsureDriver("")
		return system.DepInstallResult{
			AutoInstalled: result.Status == video.DriverStatusRegistered,
			Message:       result.Message,
		}
	}
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
	a.teleprompterMu.Unlock()

	if useNative {
		cfg := a.ConfigSvc.InternalManager().Config
		a.TeleprompterWindow.SetAppearance(cfg.GhostFontSize, cfg.GhostOpacity)
		if err := a.TeleprompterWindow.Show(); err != nil {
			return err.Error()
		}
		a.teleprompterMu.Lock()
		a.teleprompterVisible = true
		a.teleprompterMu.Unlock()
		return ""
	}

	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetSize(a.ctx, teleprompterWindowWidth, teleprompterWindowHeight)
	runtime.WindowCenter(a.ctx)
	runtime.WindowSetBackgroundColour(a.ctx, 0, 0, 0, 0)
	runtime.EventsEmit(a.ctx, "teleprompter:show")
	a.teleprompterMu.Lock()
	a.teleprompterVisible = true
	a.teleprompterMu.Unlock()
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
	var translator translation.Provider
	if cfg.TranslationProvider == config.TranslationProviderNull {
		return "听力链需要真实翻译 Provider，请在高级设置选择讯飞"
	}
	if cfg.HearingSourceLang != cfg.HearingTargetLang &&
		(cfg.XunfeiMTAppID == "" || cfg.XunfeiMTAPIKey == "" || cfg.XunfeiMTAPISecret == "") {
		return "听力链跨语言字幕需要讯飞机器翻译 AppID、API Key 和 API Secret；RTASR 只转写不需要机器翻译凭证"
	}
	llmCfg := llm.Config{
		Provider: string(cfg.LLMProvider),
		APIKey:   cfg.DeepSeekKey,
		Model:    cfg.DeepSeekModel,
		BaseURL:  cfg.LLMBaseURL,
	}
	textTranslator := translation.NewXunfeiTextTranslator(translation.XunfeiMachineTranslationConfig{
		AppID:     cfg.XunfeiMTAppID,
		APIKey:    cfg.XunfeiMTAPIKey,
		APISecret: cfg.XunfeiMTAPISecret,
	})

	chainCfg := hearing.ChainConfig{
		Xunfei: translation.XunfeiConfig{
			AppID:      cfg.XunfeiRTASRAppID,
			APIKey:     cfg.XunfeiRTASRAPIKey,
			SourceLang: cfg.HearingSourceLang,
			TargetLang: cfg.HearingTargetLang,
		},
		TextTranslator:      textTranslator,
		TranslationProvider: translator,
		LLMConfig:           llmCfg,
		DeepSeekKey:         cfg.DeepSeekKey,
		DeepSeekModel:       cfg.DeepSeekModel,
		RAGPrompt:           cfg.RAGPrompt,
		VirtualMicDevice:    cfg.VirtualMicName,
		MonitorConfig: audio.MonitorConfig{
			Enabled:      cfg.HearingMonitorEnabled,
			OutputDevice: cfg.MonitorOutputName,
			Rate:         cfg.HearingMonitorRate,
			Volume:       cfg.HearingMonitorVolume,
		},
		Retriever: retriever,
		EventSink: a.emitTeleprompterEvent,
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
	case hearing.EventError:
		if len(data) == 0 {
			return
		}
		if msg, ok := data[0].(string); ok {
			a.TeleprompterWindow.SetError(msg)
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
	case circuit.EventCircuitOpen:
		a.TeleprompterWindow.SetCircuitOpen(true)
	case circuit.EventCircuitClosed:
		a.TeleprompterWindow.SetCircuitOpen(false)
	}
}

// StopHearingChain 停止听力链，等待所有 goroutine 退出后返回。
func (a *App) StopHearingChain() {
	a.HearingChain.Stop()
}

// ===== Speaking Chain 相关绑定 =====

// StartSpeakingChain 启动说话链管道：麦克风捕获 → VAD → 讯飞翻译 → 讯飞声音复刻 TTS → 虚拟麦克风。
// 配置从当前 ConfigSvc 读取；已在运行时先停止再重新启动。
// 返回空字符串表示成功，否则返回错误描述。
func (a *App) StartSpeakingChain() string {
	cfg := a.ConfigSvc.InternalManager().Config
	var translator translation.SpeakProvider
	if cfg.TranslationProvider == config.TranslationProviderNull {
		return "说话链需要真实翻译 Provider，请在高级设置选择讯飞"
	}
	if cfg.SpeakingInputLang != cfg.SpeakingOutputLang &&
		(cfg.XunfeiMTAppID == "" || cfg.XunfeiMTAPIKey == "" || cfg.XunfeiMTAPISecret == "") {
		return "说话链跨语言输出需要讯飞机器翻译 AppID、API Key 和 API Secret；RTASR 只转写不需要机器翻译凭证"
	}
	var ttsProvider tts.Provider
	switch cfg.TTSProvider {
	case config.TTSProviderNull, config.TTSProviderSystem:
		return "说话链需要真实 TTS Provider，请在高级设置选择讯飞声音复刻"
	case config.TTSProviderXunfeiVoiceClone:
		voiceCfg := xunfeiVoiceCloneConfigFromApp(cfg)
		if !tts.XunfeiVoiceCloneConfigReady(voiceCfg) {
			return "说话链需要讯飞声音复刻 AppID、API Key、API Secret 和 Asset ID；请先在服务密钥里完成声音训练"
		}
		ttsProvider = tts.NewXunfeiVoiceCloneProvider(voiceCfg)
	}
	llmCfg := llm.Config{
		Provider: string(cfg.LLMProvider),
		APIKey:   cfg.DeepSeekKey,
		Model:    cfg.DeepSeekModel,
		BaseURL:  cfg.LLMBaseURL,
	}
	textTranslator := translation.NewXunfeiTextTranslator(translation.XunfeiMachineTranslationConfig{
		AppID:     cfg.XunfeiMTAppID,
		APIKey:    cfg.XunfeiMTAPIKey,
		APISecret: cfg.XunfeiMTAPISecret,
	})
	chainCfg := speaking.ChainConfig{
		Xunfei: translation.XunfeiSpeakConfig{
			AppID:      cfg.XunfeiRTASRAppID,
			APIKey:     cfg.XunfeiRTASRAPIKey,
			SourceLang: cfg.SpeakingInputLang,
			TargetLang: cfg.SpeakingOutputLang,
		},
		TextTranslator:     textTranslator,
		XunfeiVoiceClone:   xunfeiVoiceCloneConfigFromApp(cfg),
		Translator:         translator,
		TTSProvider:        ttsProvider,
		PhysicalMicDevice:  cfg.PhysicalMicName,
		VirtualMicDevice:   cfg.VirtualMicName,
		SilenceThresholdMs: 800,
		AudioSink:          a.VideoChain.SendAudioChunk,
		DeepSeekKey:        cfg.DeepSeekKey,
		DeepSeekModel:      cfg.DeepSeekModel,
		LLMConfig:          llmCfg,
		PolishPrompt:       cfg.SpeakPolishPrompt,
		PolishEnabled:      cfg.PolishEnabled,
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
	var lipSyncProvider lipsync.Provider
	lipSyncCloudMode := false
	if cfg.LipSyncProvider == config.LipSyncProviderNull {
		lipSyncProvider = lipsync.NewNullLipSyncProvider()
	} else if cfg.LipSyncProvider == config.LipSyncProviderSimli {
		lipSyncCloudMode = cfg.SimliKey != "" && cfg.SimliFaceID != ""
	}
	chainCfg := video.ChainConfig{
		SimliAPIKey: cfg.SimliKey,
		SilmiFaceID: cfg.SimliFaceID,
		// Simli 无公开 UDP 端点；改为 HTTP HEAD 探活。
		// 心跳判断：任何 2xx/3xx/4xx 响应 = 网络连通；5xx 或超时 = 触发熔断。
		SimliHeartbeatAddr: "https://api.simli.ai",
		PhysicalCamDevice:  cfg.PhysicalCamName,
		VirtualCamDevice:   cfg.VirtualCamName,
		LipSyncProvider:    lipSyncProvider,
		LipSyncCloudMode:   lipSyncCloudMode,
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
