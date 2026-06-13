## ADDED Requirements

### Requirement: Makefile 统一操作入口
项目 SHALL 提供 Makefile 作为所有日常操作的唯一入口，开发者无需记忆底层命令。

#### Scenario: 开发模式
- **WHEN** 执行 `make dev`
- **THEN** 启动 Wails 热重载开发服务器

#### Scenario: 构建
- **WHEN** 执行 `make build`
- **THEN** 构建当前平台制品

#### Scenario: 规范提交
- **WHEN** 执行 `make commit`
- **THEN** 启动 git-cz 交互式引导，强制 Conventional Commits 格式

#### Scenario: 版本发布
- **WHEN** 执行 `make tag-patch`（或 tag-minor / tag-major）
- **THEN** 自动运行 git-cliff 更新 CHANGELOG，创建对应版本 tag

#### Scenario: 发布推送
- **WHEN** 执行 `make release`
- **THEN** push tag 到远端，触发 GitHub Actions 构建发布流程

### Requirement: Pre-commit Hooks
项目 SHALL 配置 husky + lint-staged，在每次 `git commit` 前自动运行 lint 检查，不通过则阻止提交。

#### Scenario: Go 代码 lint
- **WHEN** 开发者执行 git commit，staged 文件包含 `.go` 文件
- **THEN** golangci-lint 对 staged Go 文件运行检查，发现问题则提交失败

#### Scenario: Vue/TS 代码 lint
- **WHEN** 开发者执行 git commit，staged 文件包含 `.vue` 或 `.ts` 文件
- **THEN** ESLint 运行检查，包含 vue-i18n 硬编码检测，发现问题则提交失败

### Requirement: Conventional Commits 规范
所有 git commit message SHALL 符合 Conventional Commits 规范（`feat:` / `fix:` / `docs:` / `chore:` 等前缀），commitlint 在 commit-msg hook 中强制验证。

#### Scenario: 不规范提交被拒绝
- **WHEN** 开发者提交不含规范前缀的 commit message
- **THEN** commitlint 报错，提交被阻止

### Requirement: CHANGELOG 自动生成
项目 SHALL 配置 git-cliff，从 Conventional Commits 历史自动生成 CHANGELOG.md，`make tag-*` 命令执行时自动触发更新。

#### Scenario: 发版时自动更新
- **WHEN** 执行 `make tag-patch`
- **THEN** git-cliff 基于自上次 tag 以来的提交生成新版本 CHANGELOG 条目，写入 CHANGELOG.md，自动 stage 并包含在版本 commit 中
