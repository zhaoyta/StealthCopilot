# ci-cd-scaffold Spec

## Purpose

定义 StealthCopilot 的 GitHub Actions CI/CD 流水线规格，包括双平台构建 Workflow、文档同步检查和分支保护规则，确保所有合并到主分支的代码均通过质量门禁。

---

## Requirements

### Requirement: 双平台构建 Workflow
GitHub Actions SHALL 配置独立的 macOS job 和 Windows job，分别在原生 runner 上编译 CGO 代码，生成平台制品并上传至 Release。

#### Scenario: macOS 构建
- **WHEN** 推送版本 tag（`v*`）
- **THEN** macOS runner 安装依赖（Xcode CLT、Wails）并执行 `wails build`，产出 `.app` 制品

#### Scenario: Windows 构建
- **WHEN** 推送版本 tag（`v*`）
- **THEN** Windows runner 安装 mingw64 + Wails 并执行 `wails build`，产出 `.exe` 制品

#### Scenario: 两平台并行构建
- **WHEN** Release workflow 触发
- **THEN** macOS job 和 Windows job 并行执行，互不阻塞

### Requirement: docs-check 强制卡关
GitHub Actions SHALL 配置 `docs-check` workflow，检测 PR 中是否存在 `feat:` 或 `fix:` 类型的提交但未修改 `docs/` 目录，若是则 CI fail 阻断合并。

#### Scenario: 功能提交未更新文档
- **WHEN** PR 包含 `feat:` 或 `fix:` 类型提交，但 diff 中无 `docs/` 路径的文件变更
- **THEN** docs-check job 失败，PR 无法合并

#### Scenario: 文档已同步更新
- **WHEN** PR 包含 `feat:` 提交且同时修改了 `docs/` 目录下的文件
- **THEN** docs-check job 通过

#### Scenario: 非功能提交豁免
- **WHEN** PR 仅包含 `chore:` 或 `refactor:` 类型提交
- **THEN** docs-check job 通过，不要求文档更新

### Requirement: Branch Protection Rules
主分支（main）SHALL 配置 Branch Protection，docs-check、lint、test 三个 check 均设为 Required Status Check，任何人（含仓库所有者）不可绕过。

#### Scenario: 未通过检查的 PR 不可合并
- **WHEN** PR 中任一 Required check 失败
- **THEN** GitHub 阻止 merge 按钮，无论权限级别
