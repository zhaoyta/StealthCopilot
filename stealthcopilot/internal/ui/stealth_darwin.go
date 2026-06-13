//go:build darwin && cgo

package ui

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa
#include <Cocoa/Cocoa.h>

static void applyStealthToNSWindow(void *windowPtr) {
	NSWindow *window = (__bridge NSWindow *)windowPtr;
	if (window == nil) {
		return;
	}

	[window setSharingType:NSWindowSharingNone];
	[window setIgnoresMouseEvents:YES];
	[window setLevel:NSFloatingWindowLevel];
	[window setOpaque:NO];
	[window setHasShadow:NO];
}
*/
import "C"

import "unsafe"

// ApplyStealth 对 macOS NSWindow 应用防录屏、置顶和鼠标穿透。
// nsWindowPtr 必须是 Wails/WebView 创建后的 NSWindow 指针。
func ApplyStealth(nsWindowPtr unsafe.Pointer) error {
	C.applyStealthToNSWindow(nsWindowPtr)
	return nil
}

// applyStealthToHandle 将通用 WindowHandle 转为 NSWindow 指针后应用 macOS hook。
func applyStealthToHandle(handle WindowHandle) (StealthStatus, error) {
	if err := ApplyStealth(unsafe.Pointer(uintptr(handle))); err != nil {
		return StealthStatusUnsupported, err
	}
	return StealthStatusApplied, nil
}
