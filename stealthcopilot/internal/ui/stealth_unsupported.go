//go:build !darwin && !windows

package ui

// applyStealthToHandle 在非 macOS/Windows 平台返回不支持。
func applyStealthToHandle(_ WindowHandle) (StealthStatus, error) {
	return StealthStatusUnsupported, nil
}
