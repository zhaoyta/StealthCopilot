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
}

// CheckDeps 检测运行所需的系统级依赖。
func CheckDeps() DepsReport {
	return DepsReport{
		VirtualMic: checkVirtualMic(),
		VirtualCam: checkVirtualCam(),
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
	// 检测 OBS 虚拟摄像头插件（com.obsproject.obs-mac-virtualcam）
	out, err := exec.Command(
		"pluginkit", "-m", "-i", "com.obsproject.obs-mac-virtualcam",
	).Output()
	if err == nil && strings.Contains(string(out), "com.obsproject") {
		return DepStatusInstalled
	}
	// 也可能通过 CoreMediaIO 插件目录安装
	out2, _ := exec.Command("ls", "/Library/CoreMediaIO/Plug-Ins/DAL/").Output()
	if strings.Contains(strings.ToLower(string(out2)), "obs") {
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
