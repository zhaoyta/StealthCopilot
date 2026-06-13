## 1. macOS 防录屏 + 穿透 Hook

- [ ] 1.1 创建 `internal/ui/stealth_darwin.go`，使用 CGO import Cocoa，实现 `ApplyStealth(nsWindowPtr unsafe.Pointer)` 函数
- [ ] 1.2 在函数内调用 `[window setSharingType:NSWindowSharingNone]` 和 `[window setIgnoresMouseEvents:YES withExceptions:YES]`
- [ ] 1.3 在 Wails `OnStartup` 回调中获取 NSWindow 句柄并调用 `ApplyStealth`
- [ ] 1.4 在 macOS 上验证：Zoom 屏幕共享时提词窗不可见，Cmd+Shift+4 截图不含提词窗

## 2. Windows 防录屏 + 穿透 Hook

- [ ] 2.1 创建 `internal/ui/stealth_windows.go`，使用 `golang.org/x/sys/windows` 调用 user32.dll
- [ ] 2.2 实现 `SetWindowDisplayAffinity(hwnd, WDA_EXCLUDEFROMCAPTURE)` 调用
- [ ] 2.3 实现 `SetWindowLong(hwnd, GWL_EXSTYLE, currentStyle|WS_EX_TRANSPARENT)` 调用
- [ ] 2.4 检测 Windows 版本，低于 2004 时显示降级警告
- [ ] 2.5 在 Wails `OnStartup` 中获取 HWND 并调用两个 Syscall

## 3. 提词窗 Wails 第二窗口

- [ ] 3.1 使用 Wails v2 多窗口 API 创建 `teleprompter` 窗口（无边框、置顶、400×300 初始尺寸）
- [ ] 3.2 对 teleprompter 窗口同样应用 stealth hook
- [ ] 3.3 Wails 暴露 `ShowTeleprompter` / `HideTeleprompter` binding

## 4. 提词窗 Vue 组件

- [ ] 4.1 创建 `src/views/Teleprompter.vue`，实现双区布局（上区字幕 + 下区回答）
- [ ] 4.2 实现字幕区：接收 Go 后端 EventEmit 推送的 dst_text，追加显示，自动滚动
- [ ] 4.3 实现回答区：接收流式 token，逐字追加，光标闪烁，生成完成后隐藏光标
- [ ] 4.4 实现底部控制栏（透明度滑块 + A- / A+ 字号按钮）
- [ ] 4.5 实现最小化/恢复胶囊逻辑
- [ ] 4.6 字号和透明度变更时调用 Go binding 持久化到本地配置
