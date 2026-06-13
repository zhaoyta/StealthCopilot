## Why

StealthCopilot 是一个从零开始的项目，需要建立完整的桌面应用基础骨架。在编写任何业务逻辑之前，必须先有可运行的 Wails 项目结构、Vue 3 前端框架、i18n 国际化支持和标准的工程规范，才能支撑后续所有模块的并行开发。

## What Changes

- 初始化 Wails v2 项目（Go 后端 + Vue 3 + TypeScript 前端）
- 配置 vue-i18n，建立 zh-CN / en-US 两套 locale 文件，所有 UI 文案走 i18n key，禁止硬编码字符串
- 配置 Tailwind CSS 作为样式框架
- 建立 Go 后端目录结构（`internal/` 分层，预留各 Pipeline 的 Provider 接口骨架）
- 配置 Conventional Commits 规范（commitizen / git-cz）
- 配置 golangci-lint（Go）+ ESLint（Vue/TS）+ husky pre-commit hooks
- 编写 Makefile（dev / build / commit / tag-patch / tag-minor / tag-major / release / docs）
- 配置 git-cliff 生成 CHANGELOG
- 搭建 GitHub Actions：双平台构建（macOS + Windows）、docs-check 卡关
- 初始化 VitePress 文档站骨架
- 添加开源合规文件（LICENSE、CONTRIBUTING.md、CODE_OF_CONDUCT.md、SECURITY.md、Issue/PR 模板）

## Capabilities

### New Capabilities

- `project-scaffold`: Wails v2 项目骨架，含 Go 后端分层结构和 Vue 3 + TypeScript 前端
- `i18n`: vue-i18n 国际化配置，zh-CN / en-US locale 文件，语言切换机制
- `provider-interfaces`: Go 后端各外部服务的 Provider 接口定义骨架（STT、TTS、LipSync、Translation），为后续模块预留可插拔扩展点
- `engineering-toolchain`: Makefile、golangci-lint、ESLint、husky hooks、git-cliff、Conventional Commits
- `ci-cd-scaffold`: GitHub Actions 双平台构建 workflow 骨架 + docs-check 强制卡关
- `docs-scaffold`: VitePress 文档站骨架 + 开源合规文件

### Modified Capabilities

## Impact

- 新建整个项目目录结构（Wails、Go modules、Vue package.json）
- 依赖：Wails v2、Go 1.21+、Node.js 18+、Vue 3、TypeScript、Tailwind CSS、vue-i18n、golangci-lint、ESLint、husky、git-cliff、commitizen
- GitHub Actions 需配置 macOS runner 和 Windows runner
- 不影响任何现有代码（greenfield 项目）
