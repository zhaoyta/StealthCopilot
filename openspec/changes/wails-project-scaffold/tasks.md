## 1. Wails 项目初始化

- [x] 1.1 安装 Wails CLI（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`），验证 `wails doctor` 通过
- [x] 1.2 在 `stealthcopilot/` 目录下执行 `wails init -n stealthcopilot -t vue-ts`，生成初始项目结构
- [x] 1.3 验证 `make dev`（即 `wails dev`）可启动，默认窗口正常打开

## 2. Go 后端目录结构

- [x] 2.1 在 `stealthcopilot/` 下创建 `internal/` 目录及子包：audio、video、stt、tts、translation、lipsync、rag、ui、config，每个包含空 `doc.go`
- [x] 2.2 更新 `go.mod`，确保模块名为 `github.com/<owner>/stealthcopilot`

## 3. Provider 接口定义

- [x] 3.1 在 `internal/stt/` 定义 `STTProvider` interface（`Transcribe(ctx, audioStream) (<-chan STTResult, error)`）
- [x] 3.2 在 `internal/translation/` 定义 `TranslationProvider` interface（输入音频流，输出 `SrcText` + `DstText` 双 channel）
- [x] 3.3 在 `internal/tts/` 定义 `TTSProvider` interface（`Synthesize(ctx, text) (<-chan []byte, error)`，流式音频 chunk）
- [x] 3.4 在`internal/lipsync/` 定义 `LipSyncProvider` interface（输入视频帧 + 音频，输出处理后视频帧 channel）
- [x] 3.5 在 `internal/config/` 定义 `ProviderConfig` struct，含各 Provider 类型选择字段

## 4. 前端配置

- [x] 4.1 安装 Tailwind CSS（`npm install -D tailwindcss postcss autoprefixer`），初始化 `tailwind.config.js`，在 `style.css` 引入 Tailwind 指令
- [x] 4.2 安装 vue-i18n v9（`npm install vue-i18n@9`），在 `main.ts` 注册 i18n 插件
- [x] 4.3 创建 `src/locales/zh-CN.json` 和 `src/locales/en-US.json`，按模块分层（settings、setup、teleprompter、common）写入初始 key
- [x] 4.4 创建 `src/i18n.ts`，配置 `createI18n`（defaultLocale: zh-CN，fallbackLocale: zh-CN）
- [x] 4.5 安装 `eslint-plugin-vue-i18n`，在 `.eslintrc` 中启用 `@intlify/vue-i18n/no-raw-text` 规则

## 5. 工程工具链

- [x] 5.1 创建 `Makefile`，实现 dev、build、build-mac、build-win、commit、tag-patch、tag-minor、tag-major、release、docs、docs-build 目标
- [x] 5.2 安装 commitizen + cz-git，配置 `package.json` 的 `config.commitizen`
- [x] 5.3 安装 commitlint（`npm install -D @commitlint/cli @commitlint/config-conventional`），创建 `commitlint.config.cjs`
- [x] 5.4 安装 husky，手动创建 commit-msg hook（运行 commitlint）和 pre-commit hook（运行 lint-staged），git config core.hooksPath .husky
- [x] 5.5 安装 lint-staged，配置 `package.json`：`.go` 文件运行 `golangci-lint run`，`.vue`/`.ts` 文件运行 `eslint --fix`
- [x] 5.6 创建 `.golangci.yml`，启用 errcheck、govet、staticcheck、gofmt 等 linter
- [x] 5.7 安装 git-cliff（brew），创建 `cliff.toml`，配置 Conventional Commits 解析规则和 CHANGELOG 格式
- [ ] 5.8 验证 `make commit` 触发交互式引导，不规范提交被 commitlint 拒绝（需手动验证，依赖终端交互）

## 6. GitHub Actions CI

- [x] 6.1 创建 `.github/workflows/build.yml`：trigger on tag `v*`，定义 build-mac（macos-14）和 build-win（windows-latest）两个并行 job，各自安装依赖并执行 `wails build`
- [x] 6.2 创建 `.github/workflows/docs-check.yml`：trigger on PR，检测 PR 提交中是否含 `feat:` 或 `fix:` 但无 `docs/` 变更，若是则 fail
- [x] 6.3 创建 `.github/workflows/lint.yml`：trigger on PR，运行 golangci-lint 和 ESLint，失败则阻断

## 7. 文档站

- [x] 7.1 安装 VitePress（`npm install -D vitepress`），初始化 `docs/` 目录，创建 `.vitepress/config.ts`
- [x] 7.2 创建文档页面：`docs/guide/index.md`（快速开始）、`docs/guide/api-keys.md`（API Key 配置）、`docs/architecture.md`（三条管道架构图）
- [ ] 7.3 验证 `make docs` 启动 VitePress 开发服务器，`make docs-build` 输出静态站（需手动验证）

## 8. 开源合规文件

- [x] 8.1 在项目根目录创建 `LICENSE`，内容为完整 AGPL-3.0 文本
- [x] 8.2 创建 `CONTRIBUTING.md`，包含：开发环境搭建、Conventional Commits 规范、PR 流程、CLA 说明
- [x] 8.3 创建 `CODE_OF_CONDUCT.md`，使用 Contributor Covenant v2.1 标准文本
- [x] 8.4 创建 `SECURITY.md`，说明漏洞私下报告方式（邮件或 GitHub Private Vulnerability Reporting）
- [x] 8.5 创建 `THIRD_PARTY_LICENSES`，列出所有直接依赖及其 License
- [x] 8.6 创建 `.github/ISSUE_TEMPLATE/bug_report.md` 和 `feature_request.md`
- [x] 8.7 创建 `.github/pull_request_template.md`，含文档更新、测试通过、CHANGELOG 更新等 checklist
