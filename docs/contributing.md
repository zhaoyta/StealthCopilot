# 贡献指南

感谢你对 StealthCopilot 的兴趣！

## 许可证说明

本项目采用 **AGPL-3.0 + 商业双授权**模式：
- 个人学习和非商业用途：免费使用 AGPL-3.0 版本
- 商业用途（转售、SaaS 集成等）：需要购买商业授权

**所有贡献者必须签署贡献者许可协议（CLA）**，首次提交 PR 时会自动提示。

## 开发环境搭建

### 前置条件

- Go 1.23+
- Node.js 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2

### 克隆并运行

```bash
git clone https://github.com/zhaoyta/stealthcopilot.git
cd stealthcopilot
npm install --prefix frontend  # 安装前端依赖
make dev                        # 启动热重载开发模式
```

## 提交规范

本项目使用 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/) 格式：

```
<type>(<scope>): <subject>

# 示例
feat(hearing): 实现讯飞实时语音翻译 WebSocket 接入
fix(audio): 修复虚拟麦克风切换时的音频爆音问题
docs(guide): 更新 API Key 配置说明
```

允许的 type：`feat` / `fix` / `docs` / `refactor` / `perf` / `test` / `chore` / `ci`

允许的 scope：`audio` / `video` / `ghost` / `hearing` / `speaking` / `rag` / `ui` / `ci` / `docs` / `config`

使用 `make commit` 启动交互式提交向导，自动生成符合规范的 commit message。

## PR 流程

1. Fork 仓库，创建功能分支（`git checkout -b feat/xxx`）
2. 编写代码和对应的单元测试
3. 确保 `make lint` 和 `make test` 全部通过
4. 如有新功能，同步更新 `docs/` 目录的相关文档
5. 提交 PR，填写 PR 模板中的各项内容
6. 签署 CLA（首次贡献时）

## 代码规范

- Go 文件：遵循 golangci-lint 配置的规则
- Vue/TS 文件：遵循 ESLint 配置的规则，所有 UI 文本走 i18n
- 单个文件不超过 500 行
- 状态值使用具名常量，禁止魔法字符串
