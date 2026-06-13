// Package ui 提供幽灵提词窗的原生窗口能力入口。
package ui

// WindowHandle 表示平台原生窗口句柄。
// macOS 对应 NSWindow 指针，Windows 对应 HWND。
type WindowHandle uintptr

// StealthStatus 表示防录屏和鼠标穿透能力的应用结果。
type StealthStatus string

const (
	// StealthStatusApplied 表示原生 hook 已成功应用。
	StealthStatusApplied StealthStatus = "applied"
	// StealthStatusUnsupported 表示当前平台或系统版本不支持该能力。
	StealthStatusUnsupported StealthStatus = "unsupported"
	// StealthStatusUnavailable 表示当前 Wails 运行时尚未提供可用窗口句柄。
	StealthStatusUnavailable StealthStatus = "unavailable"
)

// ApplyStealthToHandle 对原生窗口句柄应用防录屏和鼠标穿透。
// handle 为 0 时返回 StealthStatusUnavailable，调用方应在获取真实句柄后重试。
func ApplyStealthToHandle(handle WindowHandle) (StealthStatus, error) {
	if handle == 0 {
		return StealthStatusUnavailable, nil
	}
	return applyStealthToHandle(handle)
}
