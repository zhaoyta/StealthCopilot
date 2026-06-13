# Spec: stealth-hook

## Purpose

提词窗（Ghost Window）防录屏与鼠标穿透能力。确保提词窗在屏幕共享/截图场景下对面试官完全不可见，同时鼠标点击可穿透提词窗到达底层会议软件，交互控件仍可正常响应。

---

## Requirements

### Requirement: 防录屏 Hook（macOS）
macOS 平台 SHALL 在提词窗创建后立即调用 `NSWindowSharingNone` 使窗口从系统截屏/录屏/屏幕共享中完全消失，面试官在 Zoom/Teams 共享画面中看不到该窗口。

#### Scenario: 屏幕共享时不可见
- **WHEN** 用户在 Zoom 中开启屏幕共享
- **THEN** 提词窗在面试官端的共享画面中完全不渲染

#### Scenario: 系统截图时不可见
- **WHEN** 用户按 Cmd+Shift+3/4 截图
- **THEN** 截图结果中不包含提词窗内容

---

### Requirement: 鼠标点击穿透（macOS）
macOS 平台 SHALL 设置 `setIgnoresMouseEvents:YES withExceptions:YES`，使鼠标点击穿透提词窗到达底层会议软件，但提词窗内的交互控件（透明度滑块、字号按钮）仍可响应鼠标。

#### Scenario: 穿透到底层窗口
- **WHEN** 用户在提词窗非交互区域点击
- **THEN** 点击事件传递给底层的 Zoom/Teams 窗口

#### Scenario: 交互控件可点击
- **WHEN** 用户点击提词窗内的控制按钮
- **THEN** 按钮响应点击事件，不穿透

---

### Requirement: 防录屏 + 鼠标穿透（Windows）
Windows 平台 SHALL 调用 `SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)` 防录屏，并在 `GWL_EXSTYLE` 中注入 `WS_EX_TRANSPARENT` 实现鼠标穿透。

#### Scenario: Windows 屏幕共享不可见
- **WHEN** 用户在 Teams 中开启屏幕共享
- **THEN** 提词窗在对方共享画面中不渲染

#### Scenario: 低版本 Windows 降级处理
- **WHEN** 系统为 Windows 10 2004 之前版本
- **THEN** 应用显示警告"当前系统版本不支持防录屏功能"，提词窗正常显示但无防录屏保护
