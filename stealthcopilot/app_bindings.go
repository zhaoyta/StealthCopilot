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

	"github.com/zhaoyta/stealthcopilot/internal/asr"
	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/circuit"
	"github.com/zhaoyta/stealthcopilot/internal/config"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/hearing"
	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/rag"
	"github.com/zhaoyta/stealthcopilot/internal/resume"
	"github.com/zhaoyta/stealthcopilot/internal/speaking"
	"github.com/zhaoyta/stealthcopilot/internal/system"
	"github.com/zhaoyta/stealthcopilot/internal/trans"
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

// VoiceCloneStatusResult returns stable voice training state to the frontend.
type VoiceCloneStatusResult struct {
	OK         bool   `json:"ok"`
	State      string `json:"state"`
	Message    string `json:"message"`
	CanRetry   bool   `json:"can_retry"`
	HasTaskID  bool   `json:"has_task_id"`
	HasAssetID bool   `json:"has_asset_id"`
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
		if err := tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).ProbeToken(ctx); err != nil {
			detail := fmt.Sprintf("当前保存：App ID %s；API Key %s；API Secret %s",
				secretLengthHint(cfg.XunfeiTTSAppID),
				secretLengthHint(cfg.XunfeiTTSAPIKey),
				secretLengthHint(cfg.XunfeiTTSAPISecret),
			)
			return APIConnectionResult{Message: "讯飞声音复刻鉴权失败：" + err.Error() + "。" + detail}
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
	case "xunfei_simult":
		if cfg.XunfeiSimultAppID == "" || cfg.XunfeiSimultAPIKey == "" || cfg.XunfeiSimultAPISecret == "" {
			return APIConnectionResult{Message: "讯飞同声传译 AppID/API Key/API Secret 未完整配置"}
		}
		return probeXunfeiSimult(cfg)
	default:
		return APIConnectionResult{Message: "未知服务：" + service}
	}
}

func probeXunfeiSimult(cfg *config.AppConfig) APIConnectionResult {
	ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
	defer cancel()
	err := asr.ProbeXunfeiSimultConnection(ctx, asr.XunfeiSimultConfig{
		AppID:      cfg.XunfeiSimultAppID,
		APIKey:     cfg.XunfeiSimultAPIKey,
		APISecret:  cfg.XunfeiSimultAPISecret,
		SourceLang: cfg.HearingSourceLang,
		TargetLang: cfg.HearingTargetLang,
	})
	if err != nil {
		return APIConnectionResult{Message: "讯飞同声传译 WebSocket 握手失败：" + err.Error()}
	}
	return APIConnectionResult{OK: true, Message: "讯飞同声传译 WebSocket 握手成功"}
}

func xunfeiSimultLangPairMessage(sourceLang, targetLang string) string {
	if asr.XunfeiSimultLangPairSupported(sourceLang, targetLang) {
		return ""
	}
	return "讯飞同声传译当前只支持中文普通话 → 英文；当前语言方向为 " + sourceLang + " → " + targetLang + "。英文 → 中文需要改用 ASR + 机器翻译 + TTS 方案。"
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

func hearingASRExtensionFromApp(cfg *config.AppConfig, override asr.StreamingExtension) asr.StreamingExtension {
	if override != nil {
		return override
	}
	switch cfg.HearingASRProvider {
	case config.TranslationProviderXunfeiSimult:
		return asr.NewXunfeiRTASRLLMExtension(asr.XunfeiRTASRLLMConfig{
			AppID:      cfg.XunfeiSimultAppID,
			APIKey:     cfg.XunfeiSimultAPIKey,
			APISecret:  cfg.XunfeiSimultAPISecret,
			SourceLang: cfg.HearingSourceLang,
		})
	default:
		return override
	}
}

func speakingASRExtensionFromApp(
	cfg *config.AppConfig,
	override asr.SegmentExtension,
) asr.SegmentExtension {
	if override != nil {
		return override
	}
	switch cfg.SpeakingASRProvider {
	case config.TranslationProviderXunfeiSimult:
		return asr.NewXunfeiSimultSegmentExtension(asr.XunfeiSimultConfig{
			AppID:      cfg.XunfeiSimultAppID,
			APIKey:     cfg.XunfeiSimultAPIKey,
			APISecret:  cfg.XunfeiSimultAPISecret,
			SourceLang: cfg.SpeakingInputLang,
			TargetLang: cfg.SpeakingOutputLang,
		})
	default:
		return override
	}
}

func speechExtensionUsesXunfei(extension config.TranslationProviderType) bool {
	return extension == config.TranslationProviderXunfeiSimult
}

func hearingTransExtensionFromApp(cfg *config.AppConfig) trans.Extension {
	switch cfg.HearingTransProvider {
	case config.TranslationProviderXunfeiSimult:
		return trans.NewXunfeiTextExtension(trans.XunfeiTextTransConfig{
			AppID:      cfg.XunfeiSimultAppID,
			APIKey:     cfg.XunfeiSimultAPIKey,
			APISecret:  cfg.XunfeiSimultAPISecret,
			SourceLang: cfg.HearingSourceLang,
			TargetLang: cfg.HearingTargetLang,
		})
	default:
		return trans.SourceOnlyExtension{}
	}
}

func speakingTransExtensionFromApp(cfg *config.AppConfig) trans.Extension {
	if cfg.SpeakingTransProvider == config.TranslationProviderNull {
		return trans.SourceOnlyExtension{}
	}
	return trans.NoopExtension{}
}

func secretLengthHint(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "未保存"
	}
	return fmt.Sprintf("长度 %d", len(key))
}

// SaveLocalConfig 保存非敏感配置（语言、设备、外观等）。
func (a *App) SaveLocalConfig(req config.SaveLocalConfigRequest) string {
	return a.ConfigSvc.SaveLocalConfig(req)
}

// MarkSetupComplete 标记初始化向导已完成。
func (a *App) MarkSetupComplete() string {
	return a.ConfigSvc.MarkSetupComplete()
}

// StartVoiceTrainingRecording 通过 Go/ffmpeg 开始从物理麦克风录制音频。
// deviceName 为空时使用默认设备（avfoundation 索引 0）。
// 返回空字符串表示启动成功，否则为错误描述。
func (a *App) StartVoiceTrainingRecording(deviceName string) string {
	if err := a.VoiceRecorder.Start(deviceName); err != nil {
		return err.Error()
	}
	return ""
}

// StopVoiceTrainingResult 停止录音的结果，通过 JSON 结构体传给前端避免多返回值歧义。
type StopVoiceTrainingResult struct {
	WAV    []byte `json:"wav"`
	ErrMsg string `json:"err_msg"`
}

// StopVoiceTrainingRecording 停止录音，返回 WAV 字节和错误描述。
// err_msg 非空时表示录音失败（含 ffmpeg stderr 诊断信息）。
func (a *App) StopVoiceTrainingRecording() StopVoiceTrainingResult {
	wav, err := a.VoiceRecorder.Stop()
	if err != nil {
		return StopVoiceTrainingResult{ErrMsg: err.Error()}
	}
	return StopVoiceTrainingResult{WAV: wav}
}

// CloneVoice submits a recorded WAV sample to iFlytek VoiceClone and stores the task ID.
func (a *App) CloneVoice(audioBytes []byte) string {
	cfg := a.ConfigSvc.InternalManager().Config
	if cfg.XunfeiTTSAppID == "" || cfg.XunfeiTTSAPIKey == "" || cfg.XunfeiTTSAPISecret == "" {
		return "讯飞声音复刻 AppID/API Key/API Secret 未完整配置"
	}
	if err := tts.ValidateTrainingWAV(audioBytes); err != nil {
		return err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	taskID, err := tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).SubmitTrainingAudio(ctx, audioBytes)
	if err != nil {
		return err.Error()
	}
	if err := a.ConfigSvc.InternalManager().SaveXunfeiTTSTaskID(taskID); err != nil {
		return err.Error()
	}
	return ""
}

// GetXunfeiVoiceTrainText returns the required reading text for iFlytek voice training.
func (a *App) GetXunfeiVoiceTrainText() (tts.XunfeiVoiceTrainText, error) {
	cfg := a.ConfigSvc.InternalManager().Config
	ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
	defer cancel()
	return tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).FetchTrainText(ctx)
}

// QueryXunfeiVoiceCloneStatus refreshes a submitted iFlytek voice training task.
func (a *App) QueryXunfeiVoiceCloneStatus() VoiceCloneStatusResult {
	cfg := a.ConfigSvc.InternalManager().Config
	base := VoiceCloneStatusResult{
		State:      tts.TrainStateSubmitted,
		HasTaskID:  cfg.XunfeiTTSTaskID != "",
		HasAssetID: cfg.XunfeiTTSAssetID != "",
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiConnectionTimeout)
	defer cancel()
	result, err := tts.NewXunfeiVoiceCloneClient(xunfeiVoiceCloneConfigFromApp(cfg)).QueryTrainingResult(ctx, cfg.XunfeiTTSTaskID)
	if err != nil {
		base.State = tts.TrainStateFailed
		base.CanRetry = true
		base.Message = "讯飞声音复刻训练状态查询失败：" + err.Error()
		return base
	}
	if result.AssetID != "" {
		if err := a.ConfigSvc.InternalManager().SaveXunfeiTTSAssetID(result.AssetID); err != nil {
			base.State = tts.TrainStateFailed
			base.CanRetry = true
			base.Message = err.Error()
			return base
		}
		base.HasAssetID = true
	}
	state, canRetry := tts.XunfeiVoiceTrainState(result)
	base.State = state
	base.CanRetry = canRetry
	base.OK = state == tts.TrainStateDone
	switch state {
	case tts.TrainStateDone:
		base.Message = "讯飞声音复刻训练成功，个人复刻音色已可用"
	case tts.TrainStateSubmitted:
		base.Message = "讯飞声音复刻训练中，请稍后再查询"
	default:
		base.Message = "讯飞声音复刻训练失败：" + result.FailedDesc
	}
	return base
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
	report := a.SystemSvc.CheckDeps()
	diag.Infof("deps ffmpeg=%s virtual_mic=%s virtual_cam=%s", report.FFmpeg, report.VirtualMic, report.VirtualCam)
	return report
}

// EnumerateDevices 实时枚举系统音视频设备。
func (a *App) EnumerateDevices() system.DeviceList {
	dl := a.SystemSvc.EnumerateDevices()
	diag.Infof("devices audio_inputs=%d [%s] audio_outputs=%d [%s] video_inputs=%d [%s]",
		len(dl.AudioInputs), deviceNames(dl.AudioInputs),
		len(dl.AudioOutputs), deviceNames(dl.AudioOutputs),
		len(dl.VideoInputs), deviceNames(dl.VideoInputs),
	)
	return dl
}

// GetDiagnosticLogPath 返回本地诊断日志路径，方便用户排障时定位。
func (a *App) GetDiagnosticLogPath() string {
	return diag.Path()
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
	diag.Infof("hearing start requested virtual_mic=%q source_lang=%s target_lang=%s monitor_enabled=%t monitor_output=%q hearing_asr_provider=%s hearing_trans_provider=%s hearing_tts_provider=%s llm_provider=%s",
		cfg.VirtualMicName, cfg.HearingSourceLang, cfg.HearingTargetLang, cfg.HearingMonitorEnabled, cfg.MonitorOutputName, cfg.HearingASRProvider, cfg.HearingTransProvider, cfg.HearingTTSProvider, cfg.LLMProvider)
	retriever := rag.NewRetriever(a.ResumeSvc.InternalManager())
	var speechExtension asr.StreamingExtension
	if cfg.HearingASRProvider == config.TranslationProviderNull {
		return "听力链需要 ASR Extension，请在高级设置选择讯飞 RTASR"
	}
	if (speechExtensionUsesXunfei(cfg.HearingASRProvider) || speechExtensionUsesXunfei(cfg.HearingTransProvider)) &&
		(cfg.XunfeiSimultAppID == "" || cfg.XunfeiSimultAPIKey == "" || cfg.XunfeiSimultAPISecret == "") {
		return "讯飞同声传译配置不完整：请配置 AppID、API Key 和 API Secret"
	}
	if cfg.HearingTTSProvider != config.TTSProviderSystem && cfg.HearingTTSProvider != config.TTSProviderNull {
		return "听力链 TTS 当前支持系统语音播报或禁用"
	}
	asrCfg := asr.XunfeiRTASRLLMConfig{
		AppID:      cfg.XunfeiSimultAppID,
		APIKey:     cfg.XunfeiSimultAPIKey,
		APISecret:  cfg.XunfeiSimultAPISecret,
		SourceLang: cfg.HearingSourceLang,
	}
	llmCfg := llm.Config{
		Provider: string(cfg.LLMProvider),
		APIKey:   cfg.DeepSeekKey,
		Model:    cfg.DeepSeekModel,
		BaseURL:  cfg.LLMBaseURL,
	}
	chainCfg := hearing.ChainConfig{
		ASRConfig:        asrCfg,
		ASRExtension:     hearingASRExtensionFromApp(cfg, speechExtension),
		TransExtension:   hearingTransExtensionFromApp(cfg),
		LLMConfig:        llmCfg,
		DeepSeekKey:      cfg.DeepSeekKey,
		DeepSeekModel:    cfg.DeepSeekModel,
		RAGPrompt:        cfg.RAGPrompt,
		VirtualMicDevice: cfg.VirtualMicName,
		MonitorConfig: audio.MonitorConfig{
			Enabled:      cfg.HearingMonitorEnabled && cfg.HearingTTSProvider != config.TTSProviderNull,
			OutputDevice: cfg.MonitorOutputName,
			Rate:         cfg.HearingMonitorRate,
			Volume:       cfg.HearingMonitorVolume,
		},
		MonitorPrefersExtensionAudio: false,
		Retriever:                    retriever,
		EventSink:                    a.emitTeleprompterEvent,
	}
	if err := a.HearingChain.Start(a.ctx, chainCfg); err != "" {
		diag.Errorf("hearing start failed err=%q", err)
		return err
	}
	diag.Infof("hearing start ok")
	return ""
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
	diag.Infof("hearing stop requested")
	a.HearingChain.Stop()
	diag.Infof("hearing stop complete")
}

// ===== Speaking Chain 相关绑定 =====

// StartSpeakingChain 启动说话链管道：麦克风捕获 → VAD → 讯飞翻译 → TTS → 虚拟麦克风。
// 配置从当前 ConfigSvc 读取；已在运行时先停止再重新启动。
// 返回空字符串表示成功，否则返回错误描述。
func (a *App) StartSpeakingChain() string {
	cfg := a.ConfigSvc.InternalManager().Config
	diag.Infof("speaking start requested physical_mic=%q virtual_mic=%q input_lang=%s output_lang=%s speaking_asr_provider=%s speaking_trans_provider=%s speaking_tts_provider=%s polish_enabled=%t",
		cfg.PhysicalMicName, cfg.VirtualMicName, cfg.SpeakingInputLang, cfg.SpeakingOutputLang, cfg.SpeakingASRProvider, cfg.SpeakingTransProvider, cfg.SpeakingTTSProvider, cfg.PolishEnabled)
	var speechExtension asr.SegmentExtension
	if cfg.SpeakingASRProvider == config.TranslationProviderNull {
		return "说话链需要 ASR Extension，请在高级设置选择讯飞同声传译"
	}
	if (speechExtensionUsesXunfei(cfg.SpeakingASRProvider) || speechExtensionUsesXunfei(cfg.SpeakingTransProvider)) &&
		(cfg.XunfeiSimultAppID == "" || cfg.XunfeiSimultAPIKey == "" || cfg.XunfeiSimultAPISecret == "") {
		return "讯飞同声传译配置不完整：请配置 AppID、API Key 和 API Secret"
	}
	if msg := xunfeiSimultLangPairMessage(cfg.SpeakingInputLang, cfg.SpeakingOutputLang); msg != "" {
		diag.Errorf("speaking start rejected err=%q", msg)
		return msg
	}
	var ttsExtension tts.Extension
	resolvedTTSExtension := string(cfg.SpeakingTTSProvider)
	switch cfg.SpeakingTTSProvider {
	case config.TTSProviderNull:
		return "说话链需要 TTS Extension，请在高级设置选择默认音色或讯飞声音复刻"
	case config.TTSProviderSystem:
		ttsExtension = tts.NewSystemExtension()
	case config.TTSProviderXunfeiVoiceClone:
		voiceCfg := xunfeiVoiceCloneConfigFromApp(cfg)
		if !tts.XunfeiVoiceCloneConfigReady(voiceCfg) {
			return "个人复刻音色需要完成声音复刻训练；也可以在高级设置切换为默认音色"
		}
		ttsExtension = tts.NewXunfeiVoiceCloneExtension(voiceCfg)
	default:
		resolvedTTSExtension = string(config.TTSProviderSystem)
		ttsExtension = tts.NewSystemExtension()
	}
	diag.Infof("speaking tts extension resolved requested=%s resolved=%s voiceclone_asset_set=%t", cfg.SpeakingTTSProvider, resolvedTTSExtension, strings.TrimSpace(cfg.XunfeiTTSAssetID) != "")
	llmCfg := llm.Config{
		Provider: string(cfg.LLMProvider),
		APIKey:   cfg.DeepSeekKey,
		Model:    cfg.DeepSeekModel,
		BaseURL:  cfg.LLMBaseURL,
	}
	chainCfg := speaking.ChainConfig{
		Simult: asr.XunfeiSimultConfig{
			AppID:      cfg.XunfeiSimultAppID,
			APIKey:     cfg.XunfeiSimultAPIKey,
			APISecret:  cfg.XunfeiSimultAPISecret,
			SourceLang: cfg.SpeakingInputLang,
			TargetLang: cfg.SpeakingOutputLang,
		},
		XunfeiVoiceClone:   xunfeiVoiceCloneConfigFromApp(cfg),
		ASRExtension:       speakingASRExtensionFromApp(cfg, speechExtension),
		TransExtension:     speakingTransExtensionFromApp(cfg),
		TTSExtension:       ttsExtension,
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
	if err := a.SpeakingChain.Start(a.ctx, chainCfg); err != "" {
		diag.Errorf("speaking start failed err=%q", err)
		return err
	}
	diag.Infof("speaking start ok")
	return ""
}

// StopSpeakingChain 停止说话链，等待所有 goroutine 退出后返回。
func (a *App) StopSpeakingChain() {
	diag.Infof("speaking stop requested")
	a.SpeakingChain.Stop()
	diag.Infof("speaking stop complete")
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
	diag.Infof("video start requested physical_cam=%q virtual_cam=%q lipsync_provider=%s simli_key_set=%t simli_face_set=%t",
		cfg.PhysicalCamName, cfg.VirtualCamName, cfg.LipSyncProvider, cfg.SimliKey != "", cfg.SimliFaceID != "")
	if strings.EqualFold(strings.TrimSpace(cfg.PhysicalCamName), strings.TrimSpace(cfg.VirtualCamName)) && strings.TrimSpace(cfg.PhysicalCamName) != "" {
		err := "真实摄像头和会议虚拟摄像头不能选择同一个设备：" + cfg.PhysicalCamName
		diag.Errorf("video start rejected err=%q", err)
		return err
	}
	var lipSyncProvider lipsync.Provider
	lipSyncCloudMode := false
	if cfg.LipSyncProvider == config.LipSyncProviderNull {
		lipSyncProvider = lipsync.NewNullLipSyncProvider()
	} else if cfg.LipSyncProvider == config.LipSyncProviderSimli {
		lipSyncCloudMode = cfg.SimliKey != "" && cfg.SimliFaceID != ""
	}
	simliHeartbeatAddr := ""
	if lipSyncCloudMode {
		// Simli 无公开 UDP 端点；改为 HTTP HEAD 探活。
		// 心跳判断：任何 2xx/3xx/4xx 响应 = 网络连通；5xx 或超时 = 触发直连模式。
		// 未配置云端口型同步时不启用心跳，避免一启动视频直通就误显示直连模式。
		simliHeartbeatAddr = "https://api.simli.ai"
	}
	chainCfg := video.ChainConfig{
		SimliAPIKey:        cfg.SimliKey,
		SilmiFaceID:        cfg.SimliFaceID,
		SimliHeartbeatAddr: simliHeartbeatAddr,
		PhysicalCamDevice:  cfg.PhysicalCamName,
		VirtualCamDevice:   cfg.VirtualCamName,
		LipSyncProvider:    lipSyncProvider,
		LipSyncCloudMode:   lipSyncCloudMode,
	}
	if err := a.VideoChain.Start(a.ctx, chainCfg); err != "" {
		diag.Errorf("video start failed err=%q", err)
		return err
	}
	diag.Infof("video start ok cloud_mode=%t heartbeat=%q", lipSyncCloudMode, simliHeartbeatAddr)
	return ""
}

// StopVideoChain 停止视频链，等待所有 goroutine 退出后返回。
func (a *App) StopVideoChain() {
	diag.Infof("video stop requested")
	a.VideoChain.Stop()
	diag.Infof("video stop complete")
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

func deviceNames(devices []system.Device) string {
	if len(devices) == 0 {
		return ""
	}
	names := make([]string, 0, len(devices))
	for _, d := range devices {
		names = append(names, d.Name)
	}
	return strings.Join(names, ", ")
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
