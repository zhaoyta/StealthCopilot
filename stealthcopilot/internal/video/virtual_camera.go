// Package video — virtual_camera.go 实现虚拟摄像头驱动注册检测和帧写入接口。
// 生产实现依赖平台驱动（macOS CoreMediaIO DAL / Windows DirectShow）；
// 未注册时降级为 NullVirtualCameraWriter（丢弃所有帧）。
package video

import (
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
)

// DriverStatus 虚拟摄像头驱动注册状态
type DriverStatus int32

const (
	// DriverStatusUnknown 未检测
	DriverStatusUnknown DriverStatus = iota
	// DriverStatusRegistered 驱动已注册，虚拟摄像头可用
	DriverStatusRegistered
	// DriverStatusNotRegistered 驱动未注册
	DriverStatusNotRegistered
	// DriverStatusUnsupported 当前系统不支持虚拟摄像头
	DriverStatusUnsupported
)

// virtualCamDeviceName 是在系统设备列表中标识虚拟摄像头的名称。
const virtualCamDeviceName = "StealthVirtualCam"

// CheckDriverStatus 检测虚拟摄像头驱动是否已注册（不执行注册动作）。
// macOS：检查 CoreMediaIO DAL 插件目录；Windows：尝试 ffmpeg 枚举设备列表。
func CheckDriverStatus() DriverStatus {
	switch runtime.GOOS {
	case "darwin":
		return checkMacDriver()
	case "windows":
		return checkWinDriver()
	default:
		return DriverStatusUnsupported
	}
}

// checkMacDriver 检查 macOS CoreMediaIO DAL 插件是否已安装。
func checkMacDriver() DriverStatus {
	// CoreMediaIO DAL 插件通常安装到 /Library/CoreMediaIO/Plug-Ins/DAL/
	out, err := exec.Command(
		"ls", "/Library/CoreMediaIO/Plug-Ins/DAL/",
	).Output()
	if err != nil {
		return DriverStatusNotRegistered
	}
	for _, line := range splitLines(string(out)) {
		if containsCI(line, "StealthVirtualCam") || containsCI(line, "stealthcam") {
			return DriverStatusRegistered
		}
	}
	return DriverStatusNotRegistered
}

// checkWinDriver 检查 Windows DirectShow Filter 是否已注册（通过 ffmpeg dshow 枚举）。
func checkWinDriver() DriverStatus {
	out, err := exec.Command(
		"ffmpeg", "-f", "dshow", "-list_devices", "true", "-i", "dummy",
	).CombinedOutput()
	if err != nil && len(out) == 0 {
		return DriverStatusNotRegistered
	}
	if containsCI(string(out), virtualCamDeviceName) {
		return DriverStatusRegistered
	}
	return DriverStatusNotRegistered
}

// splitLines 将字符串按换行符分割。
func splitLines(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// containsCI 大小写不敏感字符串包含检测。
func containsCI(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	sl, subl := toLower(s), toLower(substr)
	return containsStr(sl, subl)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func containsStr(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// VirtualCameraWriter 向虚拟摄像头驱动写入视频帧。
// 生产实现通过共享内存或命名管道将 BGRA 帧推送给驱动进程。
type VirtualCameraWriter interface {
	// WriteFrame 写入一帧 BGRA 视频数据。调用方需保证帧率不超过 TargetFPS。
	WriteFrame(frame lipsync.VideoFrame) error
	// Close 释放资源。
	Close() error
}

// vcState 虚拟摄像头写入器内部状态
type vcState int32

const (
	vcStateIdle    vcState = iota
	vcStateRunning         // 正在写帧
)

// NullVirtualCameraWriter 是虚拟摄像头不可用时的空实现，丢弃所有帧。
// 用于驱动未注册或单元测试场景的降级运行。
type NullVirtualCameraWriter struct {
	state atomic.Int32
	once  sync.Once
}

// WriteFrame 丢弃视频帧（NullWriter 无实际输出）。
func (w *NullVirtualCameraWriter) WriteFrame(_ lipsync.VideoFrame) error {
	w.state.Store(int32(vcStateRunning))
	return nil
}

// Close 标记状态为 Idle，释放资源。
func (w *NullVirtualCameraWriter) Close() error {
	w.once.Do(func() {
		w.state.Store(int32(vcStateIdle))
	})
	return nil
}
