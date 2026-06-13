## ADDED Requirements

### Requirement: 开源合规文件
项目 SHALL 在根目录维护完整的开源合规文件集，包含 LICENSE、贡献规范、安全政策、第三方许可证声明。

#### Scenario: 访问 LICENSE
- **WHEN** 用户访问仓库根目录
- **THEN** 可见 `LICENSE` 文件，内容为 AGPL-3.0-only 全文，附带商业授权说明（dual license）

#### Scenario: 提交 PR 显示 CLA 提示
- **WHEN** 外部贡献者首次提交 PR
- **THEN** CLA Assistant bot 自动评论，要求贡献者在 PR 评论区签署 CLA，未签署者 PR 状态标记为 pending

#### Scenario: CLA 签署后解除 pending
- **WHEN** 贡献者在评论区回复"I have read the CLA Document and I hereby sign the CLA"
- **THEN** CLA Assistant 标记该贡献者已签署，PR pending 状态解除

#### Scenario: 第三方许可证声明
- **WHEN** 项目引入新的第三方依赖
- **THEN** `THIRD_PARTY_LICENSES` 文件中列出该依赖的名称、版本、许可证类型（CI 通过 `go-licenses` 自动生成/校验）

### Requirement: 社区规范文件
项目 SHALL 提供完整的社区规范文件，确保贡献流程规范、安全问题有处理渠道。

#### Scenario: 提交 Issue
- **WHEN** 用户点击 "New Issue"
- **THEN** GitHub 展示 Issue 模板选择：Bug Report / Feature Request / Question，引导填写必要信息

#### Scenario: 提交 PR
- **WHEN** 用户创建 PR
- **THEN** GitHub 自动填充 PR 模板，包含：改动描述、测试步骤、相关 Issue 引用、Checklist（lint/test/docs 是否通过）

#### Scenario: 安全漏洞报告
- **WHEN** 研究者发现安全漏洞
- **THEN** `SECURITY.md` 指引其通过 GitHub Security Advisories 私下报告，而非公开 Issue

### Requirement: 依赖安全扫描
项目 SHALL 启用 Dependabot（Go modules + pnpm）和 GitHub Secret Scanning，自动创建依赖升级 PR 并阻止意外密钥提交。
