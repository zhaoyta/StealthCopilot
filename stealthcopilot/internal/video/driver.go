// Package video — driver.go 封装虚拟摄像头驱动注册逻辑。
// macOS：引导用户安装 CoreMediaIO DAL 插件（需要管理员权限）。
// Windows：以 regsvr32 注册 DirectShow Filter DLL（触发 UAC）。
// Phase 1 为桩实现：检测驱动状态，提示用户手动安装，不执行自动 sudo 注册。
package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// DriverInstallResult 驱动注册结果
type DriverInstallResult struct {
	// Status 注册后最新状态
	Status DriverStatus
	// Message 给用户的提示信息
	Message string
}

// EnsureDriver 确保虚拟摄像头驱动已注册。
// 若已注册则直接返回；若未注册则尝试注册（需要用户授权）。
// Phase 1：仅检测状态，未注册时返回安装引导信息。
func EnsureDriver(bundledDriverPath string) DriverInstallResult {
	status := CheckDriverStatus()
	if status == DriverStatusRegistered {
		return DriverInstallResult{Status: status, Message: "虚拟摄像头驱动已注册"}
	}
	if status == DriverStatusUnsupported {
		return DriverInstallResult{Status: status, Message: "当前系统不支持虚拟摄像头"}
	}

	// 尝试注册
	switch runtime.GOOS {
	case "darwin":
		return installMacDriver(bundledDriverPath)
	case "windows":
		return installWinDriver(bundledDriverPath)
	default:
		return DriverInstallResult{Status: DriverStatusUnsupported, Message: "不支持的操作系统"}
	}
}

// installMacDriver 安装 macOS CoreMediaIO DAL 插件。
// 将 .plugin bundle 拷贝到 /Library/CoreMediaIO/Plug-Ins/DAL/（需要管理员权限）。
// 使用 osascript 弹出系统授权对话框，避免后台 sudo 进程。
func installMacDriver(pluginSrc string) DriverInstallResult {
	if pluginSrc == "" || !pathExists(pluginSrc) {
		return DriverInstallResult{
			Status:  DriverStatusNotRegistered,
			Message: "虚拟摄像头驱动文件不存在，请重新安装应用",
		}
	}

	destDir := "/Library/CoreMediaIO/Plug-Ins/DAL"
	destPath := filepath.Join(destDir, filepath.Base(pluginSrc))

	// 先创建目标目录（需要 sudo）
	script := fmt.Sprintf(
		`do shell script "mkdir -p '%s' && cp -R '%s' '%s'" with administrator privileges`,
		destDir, pluginSrc, destPath,
	)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return DriverInstallResult{
			Status:  DriverStatusNotRegistered,
			Message: "驱动安装失败（需要管理员授权）：" + string(out),
		}
	}

	// 重新检测
	newStatus := checkMacDriver()
	msg := "虚拟摄像头驱动安装成功，请重启应用使其生效"
	if newStatus != DriverStatusRegistered {
		msg = "驱动文件已拷贝，但检测尚未生效，请重启系统后重试"
	}
	return DriverInstallResult{Status: newStatus, Message: msg}
}

// installWinDriver 注册 Windows DirectShow Filter DLL。
// 通过 regsvr32 /s 注册（/s 静默模式）；需要 UAC 提权。
func installWinDriver(dllPath string) DriverInstallResult {
	if dllPath == "" || !pathExists(dllPath) {
		return DriverInstallResult{
			Status:  DriverStatusNotRegistered,
			Message: "虚拟摄像头 DLL 文件不存在，请重新安装应用",
		}
	}

	out, err := exec.Command("regsvr32", "/s", dllPath).CombinedOutput()
	if err != nil {
		return DriverInstallResult{
			Status:  DriverStatusNotRegistered,
			Message: "DLL 注册失败（需要管理员权限）：" + string(out),
		}
	}

	newStatus := checkWinDriver()
	return DriverInstallResult{
		Status:  newStatus,
		Message: "虚拟摄像头驱动注册成功",
	}
}

// pathExists 检查文件或目录是否存在。
func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
