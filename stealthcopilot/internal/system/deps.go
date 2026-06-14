// Package system 实现系统级依赖检测和设备枚举，与平台相关的逻辑通过构建标签分离。
package system

import (
	"fmt"
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
// macOS: 检测 OBS 虚拟摄像头或 Continuity Camera 等 CoreMediaIO 设备。
// Windows: 检测 OBS-VirtualCam DirectShow 过滤器。
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
	if strings.Contains(strings.ToLower(string(camOut)), "obs") {
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
	dalOut, _ := exec.Command("ls", "/Library/CoreMediaIO/Plug-Ins/DAL/").Output()
	if strings.Contains(strings.ToLower(string(dalOut)), "obs") {
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
	if err != nil {
		return DepStatusMissing
	}
	if len(out) > 0 {
		return DepStatusInstalled
	}
	return DepStatusMissing
}

// DepInstallResult 表示依赖安装操作的结果。
type DepInstallResult struct {
	// AutoInstalled 为 true 表示已通过包管理器自动触发安装流程。
	// 安装在独立 Terminal 中运行，仍需用户点击「重新检测」确认完成。
	AutoInstalled bool `json:"auto_installed"`
	// Message 是展示给用户的操作提示或错误说明。
	Message string `json:"message"`
}

// InstallDep 根据依赖 key 尝试引导安装。
// macOS: 虚拟声卡优先走 Homebrew（在 Terminal 中运行），否则开浏览器下载页。
// Windows: 统一打开官方下载页。
func InstallDep(key string) DepInstallResult {
	switch runtime.GOOS {
	case "darwin":
		return installDepMac(key)
	case "windows":
		return installDepWin(key)
	default:
		return DepInstallResult{Message: "当前系统暂不支持自动引导，请参考文档手动安装"}
	}
}

func installDepMac(key string) DepInstallResult {
	switch key {
	case "ffmpeg":
		if brewPath, err := exec.LookPath("brew"); err == nil {
			script := fmt.Sprintf(
				`tell application "Terminal" to do script "%s install ffmpeg"`,
				brewPath,
			)
			if err := exec.Command("osascript", "-e", script).Start(); err == nil {
				return DepInstallResult{
					AutoInstalled: true,
					Message:       "已在 Terminal 中启动 Homebrew 安装，完成后点击「重新检测」",
				}
			}
		}
		_ = exec.Command("open", "https://ffmpeg.org/download.html").Start()
		return DepInstallResult{Message: "已打开 FFmpeg 官方下载页，安装完成后点击「重新检测」"}
	case "virtual_mic":
		// 优先使用 Homebrew 在独立 Terminal 中安装，用户可看到安装进度
		if brewPath, err := exec.LookPath("brew"); err == nil {
			script := fmt.Sprintf(
				`tell application "Terminal" to do script "%s install blackhole-2ch"`,
				brewPath,
			)
			// 使用 Start() 而非 Run()：osascript 启动 Terminal 后即可返回，无需等待 Terminal 退出
			if err := exec.Command("osascript", "-e", script).Start(); err == nil {
				return DepInstallResult{
					AutoInstalled: true,
					Message:       "已在 Terminal 中启动 Homebrew 安装，完成后点击「重新检测」",
				}
			}
		}
		// Homebrew 不可用，打开官方下载页
		_ = exec.Command("open", "https://existential.audio/blackhole/").Start()
		return DepInstallResult{
			Message: "已打开 BlackHole 官方下载页，安装完成后点击「重新检测」",
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
		// 优先尝试 winget（Windows 11 / 10 新版内置）
		if _, err := exec.LookPath("winget"); err == nil {
			if err := exec.Command("winget", "install", "Gyan.FFmpeg", "--silent").Start(); err == nil {
				return DepInstallResult{
					AutoInstalled: true,
					Message:       "已通过 winget 静默安装 FFmpeg，完成后点击「重新检测」",
				}
			}
		}
		_ = exec.Command("cmd", "/c", "start", "https://ffmpeg.org/download.html#build-windows").Start()
		return DepInstallResult{Message: "已打开 FFmpeg 下载页，将 ffmpeg.exe 放入系统 PATH 后点击「重新检测」"}
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
