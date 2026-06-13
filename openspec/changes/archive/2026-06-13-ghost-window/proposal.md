## Why

幽灵提词窗是 StealthCopilot 的核心差异功能。提词窗必须对屏幕共享和截图完全不可见，同时支持鼠标点击穿透，让用户可以正常操作底层会议软件。这需要在系统原生层面注入 Hook，无法用普通 Web 技术实现。

## What Changes

- macOS：通过 CGO 调用 NSWindow API 设置防录屏属性和鼠标穿透
- Windows：通过 Syscall 调用 user32.dll 设置 WDA_EXCLUDEFROMCAPTURE 和 WS_EX_TRANSPARENT
- 提词窗 Vue 组件：上区实时字幕、下区 AI 流式回答，支持最小化、透明度/字号调节
- 与 Wails 主窗体解耦：提词窗作为独立浮窗，可独立移动

## Capabilities

### New Capabilities

- `stealth-hook`: 平台级防录屏 + 鼠标穿透 Hook，在 Wails 窗体创建后立即应用
- `teleprompter-ui`: 幽灵提词窗 Vue 组件，显示字幕和 AI 回答，支持外观控制

### Modified Capabilities

## Impact

- 新增 `internal/ui/stealth_darwin.go`（CGO）和 `internal/ui/stealth_windows.go`（Syscall）
- 新增 `src/views/Teleprompter.vue`
- macOS 构建需 CGO，依赖 Cocoa 框架
- Windows 构建需 user32.dll Syscall
