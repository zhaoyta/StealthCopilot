## Context

StealthCopilot 是 greenfield 项目，尚无任何代码。本 change 建立整个项目的工程骨架，是后续所有功能模块的基础。项目面向 macOS 和 Windows 双平台，核心是 Wails v2（Go 后端 + WebView 前端），通过 CGO/Syscall 实现系统级能力，前端用 Vue 3。

## Goals / Non-Goals

**Goals:**
- 建立可运行的 Wails v2 项目（`wails dev` 和 `wails build` 均可工作）
- 前端 Vue 3 + TypeScript + Tailwind CSS + vue-i18n，支持语言切换
- Go 后端按业务分层，预留各 Pipeline 的 Provider 接口
- 完整的工程工具链（lint、hooks、Makefile、CHANGELOG）
- GitHub Actions 双平台 CI 骨架和文档同步卡关
- 开源合规文件

**Non-Goals:**
- 实现任何音视频、LLM、API 业务逻辑
- 配置代码签名（单独 change 处理）
- 实现设置面板 UI（单独 change 处理）

## Decisions

### D1：前端框架选 Vue 3 + TypeScript（不用 React/Svelte）
Wails 官方支持三种框架，选 Vue 3 原因：Vue DevTools 调试响应式数据更直观，适合这类状态复杂的桌面应用。TypeScript 提供类型安全，在 Wails 前后端 binding 层尤为重要。

### D2：样式用 Tailwind CSS（不用 component library）
桌面应用 UI 高度定制，现有 component library（Element Plus 等）的设计语言不适合原生桌面感。Tailwind 原子类灵活，bundle 小，适合 Wails WebView 环境。

### D3：i18n 用 vue-i18n v9（Composition API 模式）
vue-i18n v9 支持 `useI18n()` composable，与 Vue 3 Composition API 一致，类型安全。locale 文件用 JSON，简单且 CI 友好。

### D4：Go 后端目录结构
```
internal/
  audio/          # 音频路由、BlackHole/VB-Cable
  video/          # 摄像头捕获、虚拟摄像头
  stt/            # STT Provider 接口
  tts/            # TTS Provider 接口
  translation/    # 翻译 Provider 接口
  lipsync/        # LipSync Provider 接口
  rag/            # RAG 管道
  ui/             # 幽灵窗 CGO/Syscall
  config/         # 设置存储
```
所有外部服务抽象为 Provider interface，不在骨架里放具体实现。

### D5：Provider 接口定义
每个外部服务定义 Go interface，便于：
- 切换供应商（讯飞 → 其他 STT）
- 接入自营云服务（StealthCloudProvider）
- 测试时 mock

### D6：Makefile 作为唯一操作入口
所有常用操作（开发、构建、提交、发版）都通过 `make <target>` 统一，降低新贡献者上手成本。

### D7：文档同步强制卡关
GitHub Actions `docs-check` workflow：PR 中含 `feat:` 或 `fix:` 提交但未修改 `docs/` 目录时，CI fail 阻断合并。这是硬约束，不可绕过。

## Risks / Trade-offs

- [Wails v2 CGO 编译] macOS 需 Xcode Command Line Tools，Windows 需 MSYS2/mingw64 → Mitigation：README 明确写出环境要求，GitHub Actions 预装好依赖
- [Windows runner CGO] GitHub Actions Windows runner 默认没有 mingw64 → Mitigation：workflow 中加 `chocolatey install mingw` 步骤
- [Tailwind + WebView] 不同平台 WebView 渲染差异（WebKit vs WebView2）→ Mitigation：早期在两平台各跑一次 UI 验证
- [vue-i18n 覆盖率] 开发者新增 UI 时忘记用 i18n key → Mitigation：ESLint 插件 `eslint-plugin-vue-i18n` 检测硬编码字符串
