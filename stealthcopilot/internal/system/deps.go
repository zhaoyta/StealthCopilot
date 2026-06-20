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
	VirtualMic DepStatus `json:"virtual_mic"` // BlackHole (macOS) / VB-Cable (Windows)
	VirtualCam DepStatus `json:"virtual_cam"` // OBS 虚拟摄像头 / CoreMediaIO 插件
	FFmpeg     DepStatus `json:"ffmpeg"`      // 音视频采集必需；macOS brew / Windows 官网
}

// CheckDeps 检测运行所需的系统级依赖。
func CheckDeps() DepsReport {
	return DepsReport{
		VirtualMic: checkVirtualMic(),
		VirtualCam: checkVirtualCam(),
		FFmpeg:     checkFFmpeg(),
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

// checkVirtualCam 检测虚拟摄像头是否可用。
// macOS: 检测 StealthVirtualCam 或 OBS 虚拟摄像头。
// Windows: 检测 StealthVirtualCam 或 OBS-VirtualCam DirectShow 过滤器。
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
	if strings.Contains(cameraText, "stealthvirtualcam") || strings.Contains(cameraText, "obs") {
		return DepStatusInstalled
	}

	// 方法2：StealthCopilot CoreMediaIO DAL 插件目录
	dalOut, _ := exec.Command("ls", "/Library/CoreMediaIO/Plug-Ins/DAL/").Output()
	dalText := strings.ToLower(string(dalOut))
	if strings.Contains(dalText, "stealthvirtualcam") || strings.Contains(dalText, "stealthcam") {
		return DepStatusInstalled
	}

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
	// 检测 StealthVirtualCam / OBS VirtualCam DirectShow 过滤器注册情况
	out, err := exec.Command(
		"reg", "query",
		`HKLM\SOFTWARE\Classes\CLSID`,
		"/s", "/f", "StealthVirtualCam",
	).Output()
	if err == nil && len(out) > 0 {
		return DepStatusInstalled
	}
	out, err = exec.Command(
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
	default:
		return DepInstallResult{Message: "未知依赖项：" + key}
	}
}
