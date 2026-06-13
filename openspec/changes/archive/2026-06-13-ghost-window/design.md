## Context

Wails 在创建窗体后通过 `runtime.WindowGetPosition` 等 API 暴露窗体句柄。macOS 通过 CGO 可直接操作 NSWindow；Windows 通过 Syscall 调用 user32.dll。两个平台的实现完全不同，用 Go build tags 隔离。

## Goals / Non-Goals

**Goals:**
- 提词窗对 Zoom/Teams 屏幕共享完全不可见
- 鼠标点击穿透，用户可正常操作底层窗口
- 提词窗内容可交互（滚动、按钮）
- 最小化后以小胶囊形式悬浮

**Non-Goals:**
- 提词窗不实现业务数据（字幕、回答内容由听力链 change 实现）
- 不处理多显示器场景（后续版本）

## Decisions

### D1：macOS 用 CGO + NSWindow
```go
//go:build darwin
// NSWindowSharingNone (0) → 彻底从系统截屏/录屏中抹除
// setIgnoresMouseEvents:YES → 鼠标点击穿透
```
Wails 暴露 `unsafe.Pointer` 窗体句柄，CGO 桥接 Cocoa。

### D2：Windows 用 Syscall + user32.dll
```go
//go:build windows
// SetWindowDisplayAffinity(hwnd, WDA_EXCLUDEFROMCAPTURE=0x11) → 防截屏
// SetWindowLong(hwnd, GWL_EXSTYLE, WS_EX_TRANSPARENT=0x20) → 鼠标穿透
```
不需要 CGO，纯 Syscall。

### D3：提词窗交互区域例外
鼠标穿透（`setIgnoresMouseEvents` / `WS_EX_TRANSPARENT`）会让整个窗口不可点击。需要在控制栏（透明度滑块、字号按钮）等交互区域动态关闭穿透，鼠标悬停时临时恢复响应，离开后重新穿透。macOS 通过 `setIgnoresMouseEvents:YES withExceptions:YES` 实现；Windows 通过消息钩子实现。

### D4：提词窗作为 Wails 第二个窗体
使用 Wails v2 多窗口 API 创建独立的提词窗，与主窗口分离，可独立移动和最小化。

## Risks / Trade-offs

- [macOS Sequoia 权限变更] macOS 新版本可能调整屏幕录制权限策略 → Mitigation：在 Setup 向导中引导用户授予"屏幕录制"权限（NSWindow API 需要）
- [Windows WDA_EXCLUDEFROMCAPTURE 兼容性] 仅 Windows 10 2004+ 支持 → Mitigation：检测系统版本，低版本降级（显示警告但不崩溃）
- [鼠标穿透与交互冲突] 提词窗内按钮无法点击 → Mitigation：用 D3 的例外机制处理，或将控制区域放在单独的小悬浮条中
