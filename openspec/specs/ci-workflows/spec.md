## ADDED Requirements

### Requirement: 双平台 CI 构建
项目 SHALL 使用 GitHub Actions，在 macOS runner 和 Windows runner 上分别构建平台对应制品，不进行跨平台交叉编译（因 CGO 限制）。

#### Scenario: macOS 构建成功
- **WHEN** push 到 `main` 或 PR 合并后触发 CI
- **THEN** macOS runner 执行：安装 Go + Node + Wails CLI + CGO 依赖（Xcode CLT）→ `wails build -platform darwin/amd64` → 输出 `.app` bundle → 代码签名 + Notarize → 打包为 `.dmg` → 上传 artifact

#### Scenario: Windows 构建成功
- **WHEN** 同上
- **THEN** Windows runner 执行：安装 Go + Node + Wails CLI + MSYS2/mingw64（CGO）→ `wails build -platform windows/amd64` → 输出 `.exe` → 代码签名（EV cert）→ 打包为 `.msi` → 上传 artifact

#### Scenario: 构建失败
- **WHEN** 任一 runner 构建失败
- **THEN** GitHub Actions 标记 PR 为 failed，阻止合并，构建日志可查

### Requirement: Release 自动发布 workflow
项目 SHALL 在 push `v*` tag 时自动触发 Release workflow，将 macOS + Windows 制品上传到 GitHub Release，并附带 CHANGELOG 段落作为 release notes。

#### Scenario: 推送版本 tag 触发 Release
- **WHEN** 开发者执行 `make tag-patch/minor/major`（自动推送 tag）
- **THEN** Release workflow 等待双平台构建完成，创建 GitHub Release，附加 `.dmg` + `.msi` + CHANGELOG 内容

### Requirement: 文档更新强制门控
项目 SHALL 在 GitHub Actions 中检查：含 `feat:` 或 `fix:` commit 的 PR，必须同时包含 `docs/` 目录的修改，否则 CI 失败。

#### Scenario: PR 含新功能但未更新文档
- **WHEN** PR 包含 `feat:` commit 且未修改 `docs/` 目录任何文件
- **THEN** `docs-check` job 失败，PR 无法合并，提示"请更新 docs/ 目录中的相关文档"

#### Scenario: 仅 fix 类提交且有文档更新
- **WHEN** PR 包含 `fix:` commit 且已修改 `docs/`
- **THEN** `docs-check` job 通过

### Requirement: Lint CI workflow
项目 SHALL 在每个 PR 上运行 lint workflow，包含 Go lint（golangci-lint）和 Vue/TS lint（ESLint），任一失败则 PR 无法合并。
