## ADDED Requirements

### Requirement: VitePress 文档站
项目 SHALL 在 `docs/` 目录维护 VitePress 文档站，涵盖架构说明、配置指南、贡献指南，并通过 GitHub Pages 自动发布。

#### Scenario: 本地文档预览
- **WHEN** 开发者执行 `make docs`
- **THEN** 启动 VitePress dev server（默认 http://localhost:5173），实时热重载

#### Scenario: 文档构建
- **WHEN** CI 执行 `docs-deploy` job
- **THEN** `vitepress build docs/` 生成静态文件到 `docs/.vitepress/dist/`，部署到 GitHub Pages

#### Scenario: 文档结构
- **WHEN** 用户访问文档站
- **THEN** 可访问以下页面：
  - 首页（产品简介、快速开始）
  - 架构说明（三管道流程图、技术选型理由）
  - API Key 配置指南（讯飞 RTASR/讯飞机器翻译/讯飞声音复刻/DeepSeek/Simli 各服务申请步骤）
  - 隐私说明（数据不上传到第三方云服务）
  - 贡献指南（开发环境搭建、Conventional Commits 规范、CLA 说明）
  - CHANGELOG（从 CHANGELOG.md 自动引用）

#### Scenario: 文档与代码同步强制要求
- **WHEN** PR 中含 `feat:` 或 `fix:` commit
- **THEN** CI `docs-check` job 检查 `docs/` 目录是否有对应修改，未修改则 CI 失败（见 ci-workflows spec）
