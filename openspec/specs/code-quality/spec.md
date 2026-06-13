## ADDED Requirements

### Requirement: Go 代码质量门控
项目 SHALL 配置 `golangci-lint`，在 CI 和本地 pre-commit 中强制检查 Go 代码质量，失败时阻断 merge。

#### Scenario: Go Lint 通过
- **WHEN** CI 执行 `golangci-lint run`
- **THEN** 无 error 级别问题输出，退出码 0

#### Scenario: Go Lint 失败
- **WHEN** 检测到 unused variable / shadowed variable / errcheck 违规
- **THEN** CI 失败，PR 无法合并，输出具体违规行号

### Requirement: Vue/TS 代码质量门控
项目 SHALL 配置 ESLint（含 `eslint-plugin-vue-i18n`），强制所有 UI 文本走 `i18n.t()` 调用，禁止硬编码中文/英文字符串。

#### Scenario: 硬编码字符串检测
- **WHEN** Vue 模板或 script 中出现硬编码用户可见字符串
- **THEN** ESLint 报 error，CI 失败

#### Scenario: i18n key 缺失检测
- **WHEN** `i18n.t('some.key')` 中的 key 在 locale JSON 中不存在
- **THEN** ESLint 报 error，CI 失败

### Requirement: Pre-commit Hook
项目 SHALL 使用 `husky` + `lint-staged`，在 `git commit` 前自动运行 lint，阻止不合规代码进入历史。

#### Scenario: 提交前 lint 检查
- **WHEN** 开发者执行 `git commit`（或 `make commit`）
- **THEN** husky pre-commit hook 运行 lint-staged，只检查暂存文件，失败时拒绝提交

#### Scenario: 首次克隆初始化
- **WHEN** 开发者执行 `pnpm install`
- **THEN** husky 自动安装 git hooks（package.json `prepare` script），无需手动操作
