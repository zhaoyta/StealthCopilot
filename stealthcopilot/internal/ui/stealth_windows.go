//go:build windows

package ui

import (
	"fmt"

	"golang.org/x/sys/windows"
)

const (
	wdaExcludeFromCapture = 0x00000011
	gwlExStyle            = ^uintptr(19) // -20 as a pointer-sized Win32 index.
	wsExTransparent       = 0x00000020
	windows10Major        = 10
	windows10Minor        = 0
	windows10Build2004    = 19041
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
	procGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
)

// applyStealthToHandle 对 Windows HWND 应用防录屏和鼠标穿透。
func applyStealthToHandle(handle WindowHandle) (StealthStatus, error) {
	if !isCaptureExclusionSupported() {
		return StealthStatusUnsupported, nil
	}

	hwnd := uintptr(handle)
	if err := setWindowDisplayAffinity(hwnd); err != nil {
		return StealthStatusUnsupported, err
	}
	if err := enableClickThrough(hwnd); err != nil {
		return StealthStatusUnsupported, err
	}
	return StealthStatusApplied, nil
}

func setWindowDisplayAffinity(hwnd uintptr) error {
	ret, _, callErr := procSetWindowDisplayAffinity.Call(hwnd, uintptr(wdaExcludeFromCapture))
	if ret == 0 {
		return fmt.Errorf("SetWindowDisplayAffinity: %w", callErr)
	}
	return nil
}

func enableClickThrough(hwnd uintptr) error {
	style, _, callErr := procGetWindowLongPtr.Call(hwnd, gwlExStyle)
	if style == 0 && callErr != windows.ERROR_SUCCESS {
		return fmt.Errorf("GetWindowLongPtr: %w", callErr)
	}

	ret, _, setErr := procSetWindowLongPtr.Call(hwnd, gwlExStyle, style|uintptr(wsExTransparent))
	if ret == 0 && setErr != windows.ERROR_SUCCESS {
		return fmt.Errorf("SetWindowLongPtr: %w", setErr)
	}
	return nil
}

func isCaptureExclusionSupported() bool {
	major, minor, build := windows.RtlGetNtVersionNumbers()
	return major > windows10Major ||
		(major == windows10Major && minor > windows10Minor) ||
		(major == windows10Major && minor == windows10Minor && build >= windows10Build2004)
}
