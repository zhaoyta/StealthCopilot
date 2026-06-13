//go:build darwin && !cgo

package ui

import (
	"errors"
	"unsafe"
)

var errCGODisabled = errors.New("CGO 未启用，无法直接调用 macOS NSWindow stealth hook")

// ApplyStealth 在 CGO 关闭时返回明确错误；Wails ContentProtection 仍会在窗口创建时提供防录屏保护。
func ApplyStealth(_ unsafe.Pointer) error {
	return errCGODisabled
}

func applyStealthToHandle(_ WindowHandle) (StealthStatus, error) {
	return StealthStatusUnavailable, errCGODisabled
}
