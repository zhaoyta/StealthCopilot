package main

// app_bindings.go 将各内部服务的方法通过 App 代理暴露给 Wails 前端。
// Wails 只识别 App 上的公开方法，服务的具体方法在此处转发。

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/config"
	"github.com/zhaoyta/stealthcopilot/internal/hearing"
	"github.com/zhaoyta/stealthcopilot/internal/rag"
	"github.com/zhaoyta/stealthcopilot/internal/resume"
	"github.com/zhaoyta/stealthcopilot/internal/system"
	"github.com/zhaoyta/stealthcopilot/internal/translation"
	"github.com/zhaoyta/stealthcopilot/internal/ui"
)

const (
	teleprompterWindowWidth  = 400
	teleprompterWindowHeight = 300
	mainWindowWidth          = 1024
	mainWindowHeight         = 768
)

// ===== Config 相关绑定 =====

// GetConfig 返回当前配置的前端视图。
func (a *App) GetConfig() config.FrontendConfig {
	return a.ConfigSvc.GetConfig()
}

// SaveAPIKey 将单个 API Key 写入 Keychain。
func (a *App) SaveAPIKey(req config.SaveAPIKeyRequest) string {
	return a.ConfigSvc.SaveAPIKey(req)
}

// SaveLocalConfig 保存非敏感配置（语言、设备、外观等）。
func (a *App) SaveLocalConfig(req config.SaveLocalConfigRequest) string {
	return a.ConfigSvc.SaveLocalConfig(req)
}

// MarkSetupComplete 标记初始化向导已完成。
func (a *App) MarkSetupComplete() string {
	return a.ConfigSvc.MarkSetupComplete()
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

// ===== Ghost Window 相关绑定 =====

// ShowTeleprompter 请求前端显示提词窗。
func (a *App) ShowTeleprompter() string {
	a.teleprompterMu.Lock()
	if !a.teleprompterVisible {
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
	a.teleprompterWindow = windowSnapshot{}
	a.teleprompterVisible = false
	a.teleprompterMu.Unlock()

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
	}
	return a.HearingChain.Start(a.ctx, chainCfg)
}

// StopHearingChain 停止听力链，等待所有 goroutine 退出后返回。
func (a *App) StopHearingChain() {
	a.HearingChain.Stop()
}
