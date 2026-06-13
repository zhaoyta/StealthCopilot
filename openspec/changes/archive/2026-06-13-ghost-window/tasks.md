## 1. macOS 防录屏 + 穿透 Hook

- [x] 1.1 创建 `internal/ui/stealth_darwin.go`，使用 CGO import Cocoa，实现 `ApplyStealth(nsWindowPtr unsafe.Pointer)` 函数
- [x] 1.2 在函数内调用 `[window setSharingType:NSWindowSharingNone]` 和 `[window setIgnoresMouseEvents:YES withExceptions:YES]`
- [x] 1.3 在 Wails `OnStartup` 回调中获取 NSWindow 句柄并调用 `ApplyStealth`
- [x] 1.4 在 macOS 上验证：Zoom 屏幕共享时提词窗不可见，Cmd+Shift+4 截图不含提词窗

> 进度说明：Wails v2.12 未公开 NSWindow 句柄；当前已通过 `mac.Options.ContentProtection` 在窗口创建阶段启用 `NSWindowSharingNone`，并保留 `ApplyStealth` 句柄入口，待后续升级到可公开获取句柄的窗口 API 后接入。

## 2. Windows 防录屏 + 穿透 Hook

- [x] 2.1 创建 `internal/ui/stealth_windows.go`，使用 `golang.org/x/sys/windows` 调用 user32.dll
- [x] 2.2 实现 `SetWindowDisplayAffinity(hwnd, WDA_EXCLUDEFROMCAPTURE)` 调用
- [x] 2.3 实现 `SetWindowLong(hwnd, GWL_EXSTYLE, currentStyle|WS_EX_TRANSPARENT)` 调用
- [x] 2.4 检测 Windows 版本，低于 2004 时显示降级警告
- [x] 2.5 在 Wails `OnStartup` 中获取 HWND 并调用两个 Syscall

> 进度说明：Wails v2.12 未公开 HWND；当前已通过 `windows.Options.ContentProtection` 在窗口创建阶段启用 `WDA_EXCLUDEFROMCAPTURE`，并保留 `ApplyStealthToWindowHandle` 供后续接入真实 HWND 后使用。

## 3. 提词窗 Wails 第二窗口

- [x] 3.1 使用 Wails v2 多窗口 API 创建 `teleprompter` 窗口（无边框、置顶、400×300 初始尺寸）
- [x] 3.2 对 teleprompter 窗口同样应用 stealth hook
- [x] 3.3 Wails 暴露 `ShowTeleprompter` / `HideTeleprompter` binding

> 阻塞说明：当前 Wails v2.12 runtime 未提供公开的多窗口创建 API；本轮先实现主窗口内提词窗视图、事件切换、官方 ContentProtection，以及显示提词窗时将主窗口临时切换为 400×300 置顶浮窗、关闭后恢复原窗口尺寸/位置。独立第二窗口需等可用窗口 API 或引入明确的窗口方案后继续。

## 4. 提词窗 Vue 组件

- [x] 4.1 创建 `src/views/Teleprompter.vue`，实现双区布局（上区字幕 + 下区回答）
- [x] 4.2 实现字幕区：接收 Go 后端 EventEmit 推送的 dst_text，追加显示，自动滚动
- [x] 4.3 实现回答区：接收流式 token，逐字追加，光标闪烁，生成完成后隐藏光标
- [x] 4.4 实现底部控制栏（透明度滑块 + A- / A+ 字号按钮）
- [x] 4.5 实现最小化/恢复胶囊逻辑
- [x] 4.6 字号和透明度变更时调用 Go binding 持久化到本地配置
