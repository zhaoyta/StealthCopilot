package system

import (
	"runtime"
	"strings"
	"testing"
)

// TestCheckDeps_ReturnsSupportedStatuses 验证 CheckDeps 仅返回已定义的状态常量。
func TestCheckDeps_ReturnsSupportedStatuses(t *testing.T) {
	validStatuses := map[DepStatus]bool{
		DepStatusInstalled: true,
		DepStatusMissing:   true,
		DepStatusUnknown:   true,
	}

	report := CheckDeps()

	if !validStatuses[report.FFmpeg] {
		t.Errorf("FFmpeg 返回了未知状态: %q", report.FFmpeg)
	}
	if !validStatuses[report.VirtualMic] {
		t.Errorf("VirtualMic 返回了未知状态: %q", report.VirtualMic)
	}
	if !validStatuses[report.VirtualCam] {
		t.Errorf("VirtualCam 返回了未知状态: %q", report.VirtualCam)
	}
}

// TestInstallDep_UnknownKey 验证未知依赖 key 返回错误提示，不触发外部命令。
func TestInstallDep_UnknownKey(t *testing.T) {
	result := InstallDep("nonexistent_dep_key")

	if result.AutoInstalled {
		t.Error("未知依赖不应标记为 AutoInstalled")
	}
	if result.Message == "" {
		t.Error("未知依赖应返回非空错误提示")
	}
	// 支持的平台会返回 "未知依赖项"，不支持的平台返回 "当前系统暂不支持"
	if !strings.Contains(result.Message, "未知") && !strings.Contains(result.Message, "暂不支持") {
		t.Errorf("未知依赖的错误消息不符合预期: %q", result.Message)
	}
}

// TestInstallDep_KnownKey_HasMessage 验证已知依赖 key 在当前平台总能返回非空提示。
// 使用 -short 标志跳过（CI 环境无 GUI，会尝试打开浏览器或 Terminal）。
func TestInstallDep_KnownKey_HasMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过：-short 模式下不触发 GUI 外部命令")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("当前系统不支持依赖安装引导，跳过测试")
	}

	keys := []string{"ffmpeg", "virtual_mic", "virtual_cam"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			result := InstallDep(key)
			if result.Message == "" {
				t.Errorf("InstallDep(%q) 返回了空消息", key)
			}
			if result.AutoInstalled {
				t.Errorf("InstallDep(%q) 不应自动安装或拉起包管理器", key)
			}
		})
	}
}

// TestCheckFFmpeg_ReturnsValidStatus 验证 checkFFmpeg 只返回 installed 或 missing。
func TestCheckFFmpeg_ReturnsValidStatus(t *testing.T) {
	status := checkFFmpeg()
	if status != DepStatusInstalled && status != DepStatusMissing {
		t.Errorf("checkFFmpeg 返回了意外状态: %q", status)
	}
}

// TestDepInstallResult_Fields 验证 DepInstallResult 字段的零值语义。
func TestDepInstallResult_Fields(t *testing.T) {
	var r DepInstallResult
	if r.AutoInstalled {
		t.Error("DepInstallResult 零值 AutoInstalled 应为 false")
	}
	if r.Message != "" {
		t.Error("DepInstallResult 零值 Message 应为空字符串")
	}
}
