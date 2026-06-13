## ADDED Requirements

### Requirement: Makefile 统一操作入口
项目 SHALL 提供根目录 `Makefile`，作为所有开发者操作的唯一入口，覆盖开发、构建、发布、文档、代码质量全流程，避免开发者直接记忆底层命令。

#### Scenario: 开发热重载
- **WHEN** 开发者执行 `make dev`
- **THEN** 启动 `wails dev`，触发 Go + Vue 热重载开发模式

#### Scenario: 生产构建
- **WHEN** 开发者执行 `make build`
- **THEN** 执行 `wails build -clean`，输出平台对应的可执行文件到 `build/bin/`

#### Scenario: 代码提交规范
- **WHEN** 开发者执行 `make commit`
- **THEN** 启动 `git-cz`（commitizen 交互式提交），强制 Conventional Commits 格式

#### Scenario: 版本打标
- **WHEN** 开发者执行 `make tag-patch` / `make tag-minor` / `make tag-major`
- **THEN** 使用 `git-cliff --bump` 自动计算下一个版本号，更新 CHANGELOG.md，创建 git tag，推送 tag

#### Scenario: 发布包
- **WHEN** 开发者执行 `make release`
- **THEN** 依次执行 build → 代码签名 → 打包（.dmg / .msi），输出到 `dist/`

#### Scenario: 文档本地预览
- **WHEN** 开发者执行 `make docs`
- **THEN** 启动 VitePress dev server（`docs/` 目录），浏览器打开文档预览

#### Scenario: 代码质量检查
- **WHEN** 开发者执行 `make lint`
- **THEN** 依次运行 `golangci-lint run` 和 `pnpm eslint`，任一失败则整体失败

#### Scenario: 测试
- **WHEN** 开发者执行 `make test`
- **THEN** 运行 `go test ./...`（含 race detector），输出覆盖率报告
