# project-scaffold Spec

## Purpose

定义 StealthCopilot 桌面应用的 Wails v2 项目骨架规格，包括 Go 后端分层目录结构和 Vue 3 + TypeScript 前端配置。

---

## Requirements

### Requirement: Wails v2 项目初始化
项目 SHALL 使用 Wails v2 作为桌面框架，Go 后端与 Vue 3 + TypeScript 前端通过 Wails binding 通信。`wails dev` 启动热重载开发环境，`wails build` 生成平台原生二进制。

#### Scenario: 开发模式启动
- **WHEN** 执行 `make dev`
- **THEN** Wails 热重载服务器启动，应用窗口打开，Go 后端和 Vue 前端均可实时更新

#### Scenario: 生产构建
- **WHEN** 执行 `make build`
- **THEN** 生成当前平台的原生二进制制品，无外部运行时依赖

### Requirement: Go 后端分层目录结构
Go 后端 SHALL 按业务领域分包在 `internal/` 目录下，每个领域有独立包，包间通过接口依赖，禁止跨包直接调用具体实现。

#### Scenario: 目录结构验证
- **WHEN** 项目初始化完成
- **THEN** `internal/` 下存在 audio、video、stt、tts、translation、lipsync、rag、ui、config 子包

#### Scenario: 包间依赖
- **WHEN** 一个包需要调用另一个包的能力
- **THEN** 通过该包暴露的 interface 调用，不直接引用具体 struct

### Requirement: Vue 3 + TypeScript 前端配置
前端 SHALL 使用 Vue 3 Composition API + TypeScript，配置 Tailwind CSS 和 Vite 构建，所有 Wails backend binding 调用均有 TypeScript 类型声明。

#### Scenario: 类型安全的 Wails 调用
- **WHEN** 前端调用 Go 后端暴露的函数
- **THEN** 调用参数和返回值均有 TypeScript 类型，编译期即可发现类型错误

#### Scenario: Tailwind 样式生效
- **WHEN** 在 Vue 组件中使用 Tailwind utility class
- **THEN** 样式在 macOS WebKit 和 Windows WebView2 均正确渲染
