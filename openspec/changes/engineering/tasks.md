## 1. Makefile

- [ ] 1.1 在项目根目录创建 `Makefile`，实现 `dev` / `build` / `build-mac` / `build-win` 目标
- [ ] 1.2 实现 `make commit`（调用 `git cz`）、`make lint`（golangci-lint + ESLint）、`make test`
- [ ] 1.3 实现 `make tag-patch` / `make tag-minor` / `make tag-major`（调用 git-cliff 生成 CHANGELOG 并打 tag）
- [ ] 1.4 实现 `make release`（build → 签名 → 打包 → 输出到 `dist/`）
- [ ] 1.5 实现 `make docs`（启动 VitePress dev server）和 `make docs-build`（生成静态文件）

## 2. 代码质量工具链

- [ ] 2.1 创建 `.golangci.yml`，启用 errcheck / unused / shadow / goimports 等 linter
- [ ] 2.2 安装 ESLint + `eslint-plugin-vue` + `eslint-plugin-vue-i18n`，创建 `.eslintrc.cjs`
- [ ] 2.3 配置 `eslint-plugin-vue-i18n` 规则：`vue-i18n/no-raw-text` error，`vue-i18n/no-missing-keys` error
- [ ] 2.4 安装 `husky` + `lint-staged`，在 `package.json` `prepare` 中自动安装 git hooks
- [ ] 2.5 配置 `.husky/pre-commit`：运行 `lint-staged`（只检查暂存文件）
- [ ] 2.6 安装 `commitlint` + `@commitlint/config-conventional`，配置 `.commitlintrc.cjs`
- [ ] 2.7 配置 `.husky/commit-msg`：运行 `commitlint --edit`

## 3. CHANGELOG 自动化

- [ ] 3.1 安装 `git-cliff`，创建 `cliff.toml`（按 feat/fix/perf/docs/chore 分组，排除 chore/docs from notes）
- [ ] 3.2 安装 `commitizen` + `cz-git`，配置 `.czrc` 使用 cz-git adapter
- [ ] 3.3 配置 `cz-git` scopes：audio / video / ghost / hearing / speaking / rag / ui / ci / docs
- [ ] 3.4 测试验证：执行 `make commit` → 完整提交向导 → `make tag-patch` → CHANGELOG.md 正确生成

## 4. GitHub Actions CI

- [ ] 4.1 创建 `.github/workflows/build.yml`：macOS + Windows 双 job，CGO 构建配置（Xcode CLT / MSYS2）
- [ ] 4.2 创建 `.github/workflows/release.yml`：监听 `v*` tag，等待 build job 完成后发布 GitHub Release
- [ ] 4.3 创建 `.github/workflows/lint.yml`：PR 触发，运行 golangci-lint + ESLint
- [ ] 4.4 创建 `.github/workflows/docs-check.yml`：检测 feat/fix commit 是否包含 docs/ 修改
- [ ] 4.5 配置 GitHub Secrets：`APPLE_SIGNING_CERT` / `APPLE_NOTARIZE_PASSWD` / `WIN_SIGNING_CERT`

## 5. VitePress 文档站

- [ ] 5.1 初始化 `docs/` 目录（`pnpm create vitepress`），配置 `docs/.vitepress/config.ts`
- [ ] 5.2 创建文档页面：首页、架构说明、API Key 配置指南、隐私说明、贡献指南
- [ ] 5.3 配置 `docs/.vitepress/config.ts` sidebar 导航结构
- [ ] 5.4 创建 `.github/workflows/docs-deploy.yml`：push main 时自动部署到 GitHub Pages

## 6. 开源合规文件

- [ ] 6.1 创建 `LICENSE`（AGPL-3.0-only 全文 + 商业授权说明段落）
- [ ] 6.2 创建 `CONTRIBUTING.md`（开发环境搭建、提交规范、CLA 说明、PR 流程）
- [ ] 6.3 创建 `CODE_OF_CONDUCT.md`（Contributor Covenant 1.4）
- [ ] 6.4 创建 `SECURITY.md`（通过 GitHub Security Advisories 私下报告漏洞的指引）
- [ ] 6.5 配置 CLA Assistant（`.github/cla-assistant.yml`），指向 CLA 文档 URL
- [ ] 6.6 创建 `.github/ISSUE_TEMPLATE/`：bug_report.yml + feature_request.yml
- [ ] 6.7 创建 `.github/PULL_REQUEST_TEMPLATE.md`（含 Checklist）
- [ ] 6.8 创建 `.github/dependabot.yml`（Go modules + pnpm 依赖自动升级，每周检查）
- [ ] 6.9 在仓库 Settings 开启 GitHub Secret Scanning + Push Protection
- [ ] 6.10 配置 go-licenses CI 检查，输出 `THIRD_PARTY_LICENSES` 文件并在 diff 中可见
