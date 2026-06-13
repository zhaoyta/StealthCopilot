# Makefile — StealthCopilot 统一操作入口
# 所有开发、构建、发布操作均通过 make 命令执行，避免记忆底层命令。
#
# 常用命令：
#   make dev         — 热重载开发模式
#   make build       — 当前平台生产构建
#   make commit      — 规范化 git 提交
#   make tag-patch   — 发布 patch 版本
#   make test        — 运行所有单元测试
#   make lint        — 代码质量检查

.PHONY: dev build build-mac build-win commit lint test \
        tag-patch tag-minor tag-major release docs docs-build

# ─── 开发 ────────────────────────────────────────────────────────────────────

## dev：启动 Wails 热重载开发服务器
dev:
	wails dev

# ─── 构建 ────────────────────────────────────────────────────────────────────

## build：构建当前平台制品（自动检测 macOS/Windows）
build:
	wails build -clean

## build-mac：构建 macOS arm64 制品
build-mac:
	wails build -clean -platform darwin/arm64

## build-win：构建 Windows amd64 制品
build-win:
	wails build -clean -platform windows/amd64

# ─── 代码质量 ────────────────────────────────────────────────────────────────

## lint：运行 Go lint + Vue/TS lint（任一失败则整体失败）
lint:
	golangci-lint run ./...
	cd frontend && npm run lint

## test：运行所有 Go 单元测试（含竞态检测）
test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# ─── 提交 & 版本 ─────────────────────────────────────────────────────────────

## commit：交互式规范化提交（Conventional Commits 格式）
commit:
	cd frontend && npx git-cz

## tag-patch：发布 patch 版本（如 v0.1.0 → v0.1.1），自动更新 CHANGELOG
tag-patch:
	git-cliff --bump --unreleased --prepend CHANGELOG.md
	@VERSION=$$(git-cliff --bumped-version); \
	git add CHANGELOG.md; \
	git commit -m "chore(release): $$VERSION"; \
	git tag -a "$$VERSION" -m "Release $$VERSION"; \
	echo "Tagged $$VERSION — run 'git push && git push --tags' to publish"

## tag-minor：发布 minor 版本（如 v0.1.0 → v0.2.0）
tag-minor:
	git-cliff --bump --bump-minor --unreleased --prepend CHANGELOG.md
	@VERSION=$$(git-cliff --bumped-version --bump-minor); \
	git add CHANGELOG.md; \
	git commit -m "chore(release): $$VERSION"; \
	git tag -a "$$VERSION" -m "Release $$VERSION"; \
	echo "Tagged $$VERSION — run 'git push && git push --tags' to publish"

## tag-major：发布 major 版本（如 v0.1.0 → v1.0.0）
tag-major:
	git-cliff --bump --bump-major --unreleased --prepend CHANGELOG.md
	@VERSION=$$(git-cliff --bumped-version --bump-major); \
	git add CHANGELOG.md; \
	git commit -m "chore(release): $$VERSION"; \
	git tag -a "$$VERSION" -m "Release $$VERSION"; \
	echo "Tagged $$VERSION — run 'git push && git push --tags' to publish"

# ─── 发布 ────────────────────────────────────────────────────────────────────

## release：构建 + 签名 + 打包，输出到 dist/
release: build
	mkdir -p dist
	@echo "TODO: 添加平台签名脚本"
	@echo "构建产物已输出到 build/bin/"

# ─── 文档 ────────────────────────────────────────────────────────────────────

## docs：启动 VitePress 文档开发服务器
docs:
	cd ../docs && npm run docs:dev

## docs-build：构建文档静态站
docs-build:
	cd ../docs && npm run docs:build
