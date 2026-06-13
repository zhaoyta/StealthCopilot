## ADDED Requirements

### Requirement: VitePress 文档站
项目 SHALL 使用 VitePress 搭建文档站，`make docs` 本地预览，内容包含架构说明、API Key 配置指南、开发环境搭建指南。

#### Scenario: 本地文档预览
- **WHEN** 执行 `make docs`
- **THEN** VitePress 开发服务器启动，文档可在浏览器中访问

#### Scenario: 文档构建
- **WHEN** 执行 `make docs-build`
- **THEN** 生成静态文档站，可部署到 GitHub Pages 或其他静态托管

### Requirement: 开源合规文件
项目根目录 SHALL 包含所有开源必要文件：LICENSE（AGPL-3.0）、CONTRIBUTING.md、CODE_OF_CONDUCT.md、SECURITY.md、THIRD_PARTY_LICENSES。

#### Scenario: LICENSE 文件存在
- **WHEN** 查看项目根目录
- **THEN** 存在 LICENSE 文件，内容为完整 AGPL-3.0 文本

#### Scenario: 安全漏洞报告流程
- **WHEN** 安全研究员发现漏洞
- **THEN** SECURITY.md 中有明确的私下报告流程（邮箱或 GitHub Private Vulnerability Reporting），不要求公开 issue

### Requirement: Issue 和 PR 模板
项目 SHALL 配置 GitHub Issue 模板（Bug Report、Feature Request）和 PR 模板，引导贡献者提供必要信息。

#### Scenario: Bug Report 模板
- **WHEN** 用户新建 Issue 并选择 Bug Report
- **THEN** 模板自动填充，包含：复现步骤、期望行为、实际行为、平台信息（macOS/Windows 版本）字段

#### Scenario: PR 模板 Checklist
- **WHEN** 开发者新建 PR
- **THEN** PR 描述自动填充 checklist，包含：`[ ] 文档已更新`、`[ ] 测试已通过`、`[ ] CHANGELOG 已更新` 等项
