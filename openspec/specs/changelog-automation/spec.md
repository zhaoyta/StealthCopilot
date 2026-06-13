## ADDED Requirements

### Requirement: git-cliff 自动生成 CHANGELOG
项目 SHALL 使用 `git-cliff` + Conventional Commits 规范，在每次打版本 tag 时自动生成/更新 `CHANGELOG.md`，无需手动维护。

#### Scenario: 打 patch tag
- **WHEN** 开发者执行 `make tag-patch`
- **THEN** git-cliff 解析从上次 tag 到 HEAD 的所有 `fix: / perf:` commit，生成该版本的 changelog section，追加到 `CHANGELOG.md` 顶部，创建 git tag（如 `v0.1.1`）

#### Scenario: 打 minor tag
- **WHEN** 开发者执行 `make tag-minor`
- **THEN** git-cliff 解析 `feat:` commit，生成该版本 changelog，追加到 `CHANGELOG.md` 顶部，创建 git tag（如 `v0.2.0`）

#### Scenario: 打 major tag
- **WHEN** 开发者执行 `make tag-major`
- **THEN** git-cliff 解析 `feat!: / BREAKING CHANGE:` commit，生成该版本 changelog，追加到 `CHANGELOG.md` 顶部，创建 git tag（如 `v1.0.0`）

#### Scenario: Conventional Commits 格式违规
- **WHEN** commit message 不符合 Conventional Commits 格式（如缺少 type 前缀）
- **THEN** commitlint pre-commit hook 拒绝提交，提示正确格式示例

### Requirement: commitizen 交互式提交
项目 SHALL 配置 commitizen + `cz-git` adapter，通过 `make commit` 提供交互式提交向导，生成符合 Conventional Commits 格式的 commit message。

#### Scenario: 使用 make commit 提交
- **WHEN** 开发者执行 `make commit`
- **THEN** 启动 cz-git 交互式向导，引导选择 type（feat/fix/docs/...）、scope、subject，最终生成规范 commit message
