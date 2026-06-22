// Package system 实现系统级依赖检测和设备枚举，与平台相关的逻辑通过构建标签分离。
package system

import (
	"os/exec"
	"runtime"
	"strings"
)

// DepStatus 表示单项系统依赖的检测状态。
type DepStatus string

const (
	// DepStatusInstalled 依赖已安装并可用。
	DepStatusInstalled DepStatus = "installed"
	// DepStatusMissing 依赖未安装。
	DepStatusMissing DepStatus = "missing"
	// DepStatusUnknown 无法确定安装状态。
	DepStatusUnknown DepStatus = "unknown"
)

// DepsReport 包含所有依赖项的检测结果。
type DepsReport struct {
	VirtualMic     DepStatus `json:"virtual_mic"`     // BlackHole (macOS) / VB-Cable (Windows)
	VirtualCam     DepStatus `json:"virtual_cam"`     // OBS 虚拟摄像头 / CoreMediaIO 插件
	FFmpeg         DepStatus `json:"ffmpeg"`          // 音视频采集必需；macOS brew / Windows 官网
	EmbeddingModel DepStatus `json:"embedding_model"` // 简历本地 embedding：Python + sentence-transformers
}

// CheckDeps 检测运行所需的系统级依赖。
func CheckDeps() DepsReport {
	return DepsReport{
		VirtualMic:     checkVirtualMic(),
		VirtualCam:     checkVirtualCam(),
		FFmpeg:         checkFFmpeg(),
		EmbeddingModel: checkEmbeddingModel(),
	}
}

// checkFFmpeg 检测 ffmpeg 是否在 PATH 中可用。
// ffmpeg 用于音视频采集（avfoundation/dshow）和设备枚举，缺失时两条链路均无法工作。
func checkFFmpeg() DepStatus {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return DepStatusInstalled
	}
	return DepStatusMissing
}

func checkEmbeddingModel() DepStatus {
	for _, python := range embeddingPythonCandidates() {
		if exec.Command(python, "-c", `import importlib.util as u; raise SystemExit(0 if u.find_spec("sentence_transformers") and u.find_spec("torch") else 1)`).Run() == nil {
			return DepStatusInstalled
		}
	}
	return DepStatusMissing
}

func embeddingPythonCandidates() []string {
	return []string{
		"python3",
		"python3.13",
		"python3.12",
		"python3.11",
		"python3.10",
		"/opt/homebrew/bin/python3.13",
		"/opt/homebrew/bin/python3.12",
		"/opt/homebrew/bin/python3.11",
		"/opt/homebrew/bin/python3.10",
		"/usr/local/bin/python3.13",
		"/usr/local/bin/python3.12",
		"/usr/local/bin/python3.11",
		"/usr/local/bin/python3.10",
	}
}

// checkVirtualMic 检测虚拟声卡是否已安装。
// macOS: 检测 BlackHole 内核扩展。
// Windows: 检测 VB-Cable 驱动。
func checkVirtualMic() DepStatus {
	switch runtime.GOOS {
	case "darwin":
		return checkMacBlackHole()
	case "windows":
		return checkWinVBCable()
	default:
		return DepStatusUnknown
	}
}

// checkVirtualCam 检测 OBS 虚拟摄像头是否可用。
func checkVirtualCam() DepStatus {
	switch runtime.GOOS {
	case "darwin":
		return checkMacVirtualCam()
	case "windows":
		return checkWinVirtualCam()
	default:
		return DepStatusUnknown
	}
}

func checkMacBlackHole() DepStatus {
	// BlackHole 安装后在 /Library/Audio/Plug-Ins/HAL 下有对应的 .driver bundle
	out, err := exec.Command("ls", "/Library/Audio/Plug-Ins/HAL").Output()
	if err != nil {
		return DepStatusMissing
	}
	if strings.Contains(strings.ToLower(string(out)), "blackhole") {
		return DepStatusInstalled
	}
	return DepStatusMissing
}

func checkWinVBCable() DepStatus {
	// 在注册表中查找 VB-Audio Virtual Cable
	out, err := exec.Command(
		"reg", "query",
		`HKLM\SYSTEM\CurrentControlSet\Enum\HDAUDIO`,
		"/s", "/f", "VBCable",
	).Output()
	if err != nil {
		return DepStatusMissing
	}
	if len(out) > 0 {
		return DepStatusInstalled
	}
	return DepStatusMissing
}

func checkMacVirtualCam() DepStatus {
	// 方法1：system_profiler 直接列出已注册摄像头，是最可靠的地面实况
	// OBS v28+ 注册为 "OBS Virtual Camera"（系统扩展方式）
	camOut, _ := exec.Command("system_profiler", "SPCameraDataType").Output()
	cameraText := strings.ToLower(string(camOut))
	if strings.Contains(cameraText, "obs") {
		return DepStatusInstalled
	}

	dalOut, _ := exec.Command("ls", "/Library/CoreMediaIO/Plug-Ins/DAL/").Output()
	dalText := strings.ToLower(string(dalOut))
	// 方法2：检测 OBS v28+ 系统扩展（mac-camera-extension）
	extOut, _ := exec.Command("systemextensionsctl", "list").Output()
	if strings.Contains(string(extOut), "com.obsproject.obs-studio.mac-camera-extension") {
		return DepStatusInstalled
	}

	// 方法3：OBS app bundle 内置插件（v28+ 打包在 .app 中）
	if _, err := exec.Command("ls",
		"/Applications/OBS.app/Contents/PlugIns/mac-virtualcam.plugin",
	).Output(); err == nil {
		return DepStatusInstalled
	}

	// 方法4：旧版 OBS v27- CoreMediaIO DAL 插件目录
	if strings.Contains(dalText, "obs") {
		return DepStatusInstalled
	}

	return DepStatusMissing
}

func checkWinVirtualCam() DepStatus {
	// 检测 OBS VirtualCam DirectShow 过滤器注册情况
	out, err := exec.Command(
		"reg", "query",
		`HKLM\SOFTWARE\Classes\CLSID`,
		"/s", "/f", "OBS Virtual Camera",
	).Output()
	if err == nil && len(out) > 0 {
		return DepStatusInstalled
	}
	return DepStatusMissing
}

// DepInstallResult 表示依赖安装操作的结果。
type DepInstallResult struct {
	// AutoInstalled is kept for frontend compatibility. The app no longer
	// launches package managers automatically; dependencies are installed by the user.
	AutoInstalled bool `json:"auto_installed"`
	// Message 是展示给用户的操作提示或错误说明。
	Message string `json:"message"`
}

// InstallDep returns manual installation guidance for the requested dependency.
// It must not open Terminal or launch package managers automatically.
func InstallDep(key string) DepInstallResult {
	switch runtime.GOOS {
	case "darwin":
		return installDepMac(key)
	case "windows":
		return installDepWin(key)
	default:
		return DepInstallResult{Message: "当前系统暂不支持安装引导，请参考文档手动安装"}
	}
}

func installDepMac(key string) DepInstallResult {
	switch key {
	case "ffmpeg":
		if brewPath, err := exec.LookPath("brew"); err == nil {
			return DepInstallResult{Message: "请在 Terminal 手动执行：" + brewPath + " install ffmpeg。安装完成后点击「重新检测」。"}
		}
		_ = exec.Command("open", "https://ffmpeg.org/download.html").Start()
		return DepInstallResult{Message: "已打开 FFmpeg 官方下载页，请手动安装并放入 PATH，完成后点击「重新检测」。"}
	case "virtual_mic":
		if brewPath, err := exec.LookPath("brew"); err == nil {
			return DepInstallResult{Message: "请在 Terminal 手动执行：" + brewPath + " install blackhole-2ch。安装完成后重启会议软件并点击「重新检测」。"}
		}
		_ = exec.Command("open", "https://existential.audio/blackhole/").Start()
		return DepInstallResult{
			Message: "已打开 BlackHole 官方下载页，请手动安装 BlackHole 2ch，完成后重启会议软件并点击「重新检测」。",
		}
	case "virtual_cam":
		_ = exec.Command("open", "https://obsproject.com/").Start()
		return DepInstallResult{
			Message: "已打开 OBS 官方下载页，安装后在 OBS 工具栏启用「虚拟摄像头」，再点击「重新检测」",
		}
	case "embedding_model":
		python := preferredEmbeddingPython()
		return DepInstallResult{
			Message: "请在 Terminal 手动执行：" + python + " -m pip install -U sentence-transformers torch。安装完成后点击「重新检测」，首次上传简历会自动下载本地模型。",
		}
	default:
		return DepInstallResult{Message: "未知依赖项：" + key}
	}
}

func installDepWin(key string) DepInstallResult {
	switch key {
	case "ffmpeg":
		if wingetPath, err := exec.LookPath("winget"); err == nil {
			return DepInstallResult{Message: "请在终端手动执行：" + wingetPath + " install Gyan.FFmpeg。安装完成后点击「重新检测」。"}
		}
		_ = exec.Command("cmd", "/c", "start", "https://ffmpeg.org/download.html#build-windows").Start()
		return DepInstallResult{Message: "已打开 FFmpeg 下载页，请手动安装并将 ffmpeg.exe 放入系统 PATH，完成后点击「重新检测」。"}
	case "virtual_mic":
		_ = exec.Command("cmd", "/c", "start", "https://vb-audio.com/Cable/").Start()
		return DepInstallResult{
			Message: "已打开 VB-Cable 下载页，安装完成后重启应用并点击「重新检测」",
		}
	case "virtual_cam":
		_ = exec.Command("cmd", "/c", "start", "https://obsproject.com/").Start()
		return DepInstallResult{
			Message: "已打开 OBS 下载页，安装后在 OBS 工具栏启用「虚拟摄像头」，再点击「重新检测」",
		}
	case "embedding_model":
		return DepInstallResult{
			Message: "请在终端手动执行：py -3.11 -m pip install -U sentence-transformers torch。若未安装 Python 3.11，请先从 python.org 安装；完成后点击「重新检测」。",
		}
	default:
		return DepInstallResult{Message: "未知依赖项：" + key}
	}
}

func preferredEmbeddingPython() string {
	for _, python := range []string{"python3.11", "/opt/homebrew/bin/python3.11", "/usr/local/bin/python3.11", "python3"} {
		if _, err := exec.LookPath(python); err == nil {
			return python
		}
	}
	return "python3"
}
