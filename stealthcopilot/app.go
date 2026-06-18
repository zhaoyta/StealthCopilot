package main

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/config"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/hearing"
	"github.com/zhaoyta/stealthcopilot/internal/resume"
	"github.com/zhaoyta/stealthcopilot/internal/speaking"
	"github.com/zhaoyta/stealthcopilot/internal/system"
	"github.com/zhaoyta/stealthcopilot/internal/ui"
	"github.com/zhaoyta/stealthcopilot/internal/video"
)

//go:embed scripts/embed.py
var embeddedEmbedScript []byte

// App 是 Wails 应用主结构，负责生命周期管理和各服务的协调初始化。
type App struct {
	ctx                 context.Context
	ConfigSvc           *config.Service
	ResumeSvc           *resume.Service
	SystemSvc           *system.Service
	HearingChain        *hearing.Chain
	SpeakingChain       *speaking.Chain
	VideoChain          *video.Chain
	TeleprompterWindow  ui.TeleprompterWindow
	VoiceRecorder       *audio.VoiceTrainingRecorder
	teleprompterMu      sync.RWMutex
	teleprompterVisible bool
	teleprompterWindow  windowSnapshot
	stealthStatus       ui.StealthStatus
}

// windowSnapshot 记录进入提词窗模式前的主窗口状态，用于关闭提词窗时恢复。
type windowSnapshot struct {
	Width  int
	Height int
	X      int
	Y      int
	Saved  bool
}

// NewApp 创建 App 实例（服务在 startup 中初始化）。
func NewApp() *App {
	return &App{}
}

// startup 在 Wails 应用启动时调用，完成所有服务的初始化和配置预加载。
// 初始化失败时弹出错误对话框并退出。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.stealthStatus = ui.StealthStatusUnavailable

	dataDir := appDataDir()
	diag.Init(dataDir)
	diag.Infof("startup begin data_dir=%s", dataDir)
	scriptPath := filepath.Join(dataDir, "embed.py")
	if err := ensureEmbeddedFile(scriptPath, embeddedEmbedScript, 0o700); err != nil {
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "初始化失败",
			Message: "无法安装 embedding 脚本：" + err.Error(),
		})
		os.Exit(1)
	}

	// 1. 配置服务（含 Keychain 预读，应 2s 内完成）
	cfgSvc, err := config.NewService(dataDir)
	if err != nil {
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "初始化失败",
			Message: "无法加载配置：" + err.Error(),
		})
		os.Exit(1)
	}
	a.ConfigSvc = cfgSvc
	cfgSvc.Startup(ctx)
	diag.Infof("config loaded setup_completed=%t ui_locale=%s translation_provider=%s tts_provider=%s lipsync_provider=%s virtual_mic=%q physical_mic=%q monitor_enabled=%t monitor_output=%q physical_cam=%q virtual_cam=%q",
		cfgSvc.InternalManager().Config.SetupCompleted,
		cfgSvc.InternalManager().Config.UILocale,
		cfgSvc.InternalManager().Config.TranslationProvider,
		cfgSvc.InternalManager().Config.TTSProvider,
		cfgSvc.InternalManager().Config.LipSyncProvider,
		cfgSvc.InternalManager().Config.VirtualMicName,
		cfgSvc.InternalManager().Config.PhysicalMicName,
		cfgSvc.InternalManager().Config.HearingMonitorEnabled,
		cfgSvc.InternalManager().Config.MonitorOutputName,
		cfgSvc.InternalManager().Config.PhysicalCamName,
		cfgSvc.InternalManager().Config.VirtualCamName,
	)

	// 2. 简历服务（embedding：Python 桥接，不可用时降级为 NullProvider）
	var embedder resume.EmbeddingProvider
	if cfgSvc.InternalManager().Config.EmbeddingProvider == config.EmbeddingProviderPythonBridge {
		provider := resume.NewPythonBridgeProvider(scriptPath)
		if provider.Ready() {
			embedder = provider
		} else {
			embedder = &resume.NullProvider{}
		}
	} else {
		embedder = &resume.NullProvider{}
	}

	resumeSvc, err := resume.NewService(dataDir, embedder)
	if err != nil {
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "初始化失败",
			Message: "无法初始化简历服务：" + err.Error(),
		})
		os.Exit(1)
	}
	a.ResumeSvc = resumeSvc
	resumeSvc.Startup(ctx)

	// 3. 系统服务（设备枚举、依赖检测）
	a.SystemSvc = system.NewSystemService()

	// 4. 听力链协调器（各组件在 StartHearingChain binding 中按需实例化）
	a.HearingChain = &hearing.Chain{}

	// 5. 说话链协调器（各组件在 StartSpeakingChain binding 中按需实例化）
	a.SpeakingChain = &speaking.Chain{}

	// 6. 视频链协调器（各组件在 StartVideoChain binding 中按需实例化）
	a.VideoChain = &video.Chain{}

	// 7. 原生提词窗（平台不可用时内部为 no-op，ShowTeleprompter 会走 Wails fallback）
	a.TeleprompterWindow = ui.NewTeleprompterWindow()

	// 8. 声音复刻训练录音器（通过 Go/ffmpeg 访问麦克风，绕过 WKWebView getUserMedia 限制）
	a.VoiceRecorder = &audio.VoiceTrainingRecorder{}
	diag.Infof("startup complete")
}

// shutdown 在 Wails 应用关闭时调用，释放资源。
func (a *App) shutdown(_ context.Context) {
	diag.Infof("shutdown begin")
	if a.TeleprompterWindow != nil {
		_ = a.TeleprompterWindow.Close()
	}
	if a.ResumeSvc != nil {
		_ = a.ResumeSvc.InternalManager().Close()
	}
	diag.Infof("shutdown complete")
}

// appDataDir 返回平台相关的应用数据目录。
// macOS: ~/Library/Application Support/StealthCopilot
// Windows/其他: ~/.stealthcopilot
func appDataDir() string {
	homeDir, _ := os.UserHomeDir()
	switch {
	case isDir(filepath.Join(homeDir, "Library", "Application Support")):
		dir := filepath.Join(homeDir, "Library", "Application Support", "StealthCopilot")
		_ = os.MkdirAll(dir, 0o700)
		return dir
	default:
		dir := filepath.Join(homeDir, ".stealthcopilot")
		_ = os.MkdirAll(dir, 0o700)
		return dir
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func ensureEmbeddedFile(path string, data []byte, perm os.FileMode) error {
	current, err := os.ReadFile(path)
	if err == nil && string(current) == string(data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}
