## Context

工程规范 change 不涉及业务逻辑，覆盖开发体验、代码质量、文档、发版和开源合规。这些配置一旦建立后很少改动，但对项目长期健康影响深远。

## Goals / Non-Goals

**Goals:**
- 所有日常操作通过 Makefile 统一入口
- PR 合并和发版均有自动化质量门禁
- 文档与代码强制同步
- 满足 AGPL v3 + 商业双授权的开源合规要求

**Non-Goals:**
- 不实现自动化测试（业务测试随各功能 change 添加）
- 不配置 Homebrew/Scoop 发布（依赖代码签名，单独处理）

## Decisions

### D1：Makefile 作为唯一操作入口
所有工具（wails、golangci-lint、eslint、git-cz、git-cliff）通过 Makefile target 封装，新贡献者只需看 `make help` 输出。

### D2：docs-check 用 GitHub Actions 脚本检测
脚本获取 PR 的 commit messages，正则匹配 `^feat:|^fix:`，再检查 PR diff 是否包含 `docs/` 路径变更。两个条件同时满足才通过，缺一 fail。

### D3：CLA Assistant 通过 GitHub App 集成
使用 `cla-assistant.io` 或自托管 CLA Assistant GitHub App，在每个 PR 检测贡献者是否已签署 CLA，未签署则 CI fail 并添加评论引导签署。这是双授权模式的法律必要条件。

### D4：发版流程 make tag-* → CI 自动构建
`make tag-patch` = 运行 git-cliff 更新 CHANGELOG → git commit → git tag vX.Y.Z → `make release` = git push --tags → 触发 GitHub Actions build.yml → 构建双平台制品 → 创建 GitHub Release 并上传制品。

## Risks / Trade-offs

- [Windows runner CGO] GitHub Actions Windows runner 需要 mingw64 → workflow 中加 `choco install mingw` 步骤
- [CLA 回溯] 已有贡献者（仅自己）无需 CLA → CLA Assistant 配置豁免仓库所有者
- [golangci-lint 误报] 部分 linter 规则对 CGO 代码误报 → `.golangci.yml` 对 `internal/ui/` 目录针对性关闭相关规则
