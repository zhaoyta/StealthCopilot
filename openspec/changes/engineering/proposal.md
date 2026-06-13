## Why

开源项目需要完整的工程基础设施：一致的代码规范、自动化 CI/CD、强制文档同步、规范的发版流程，以及满足 AGPL v3 双授权模式的开源合规文件。这些是项目长期健康运转的基础，也是吸引外部贡献者的前提。

## What Changes

- Makefile：统一所有日常操作入口
- 代码质量工具链：golangci-lint + ESLint + husky + commitlint
- git-cliff：Conventional Commits → CHANGELOG 自动生成
- GitHub Actions：双平台构建 + docs-check 强制卡关 + lint workflow
- VitePress 文档站
- 开源合规文件（LICENSE / CONTRIBUTING / CODE_OF_CONDUCT / SECURITY / CLA）
- 代码签名配置（macOS Notarize + Windows）

## Capabilities

### New Capabilities

- `makefile`: 统一操作入口，覆盖 dev / build / commit / tag / release / docs
- `code-quality`: golangci-lint + ESLint（含 vue-i18n 检测）+ husky hooks + commitlint
- `changelog-automation`: git-cliff 配置，发版时自动生成 CHANGELOG
- `ci-workflows`: GitHub Actions 双平台构建 + docs-check + lint 三个 workflow
- `vitepress-docs`: 文档站，含架构说明、API Key 配置、贡献指南
- `open-source-compliance`: LICENSE + 社区文件 + CLA + 代码签名配置

### Modified Capabilities

## Impact

- 根目录新增：Makefile、.golangci.yml、commitlint.config.js、cliff.toml
- 新增 `.github/workflows/`：build.yml、docs-check.yml、lint.yml
- 新增 `docs/` 目录（VitePress）
- 新增开源合规文件：LICENSE、CONTRIBUTING.md、CODE_OF_CONDUCT.md、SECURITY.md、THIRD_PARTY_LICENSES
- 新增 `.github/ISSUE_TEMPLATE/` 和 `pull_request_template.md`
- 依赖（开发）：husky、lint-staged、commitizen、git-cliff、VitePress
