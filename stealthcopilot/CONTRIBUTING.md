# Contributing to StealthCopilot

感谢你对 StealthCopilot 的贡献！请先阅读以下说明。

## 贡献者许可协议（CLA）

本项目采用 AGPL-3.0 + 商业双授权模式。**所有贡献者必须签署 CLA**，首次提交 PR 时 CLA Assistant bot 会自动提示。

## 开发环境

```bash
# 前置：Go 1.23+, Node 20+, Wails CLI v2
git clone https://github.com/zhaoyta/stealthcopilot.git
cd stealthcopilot
npm install --prefix frontend
make dev
```

## 提交规范

使用 `make commit` 触发交互式提交向导（Conventional Commits 格式）。

格式：`<type>(<scope>): <subject>`

- type：`feat` / `fix` / `docs` / `refactor` / `perf` / `test` / `chore` / `ci`
- scope：`audio` / `video` / `ghost` / `hearing` / `speaking` / `rag` / `ui` / `ci` / `docs` / `config`

## PR 流程

1. Fork → 创建分支 → 提交代码和测试
2. 运行 `make lint` 和 `make test`，确保全部通过
3. 新功能必须同步更新 `docs/` 目录
4. 提交 PR，签署 CLA

详细说明见[文档站贡献指南](https://zhaoyta.github.io/stealthcopilot/contributing)。
