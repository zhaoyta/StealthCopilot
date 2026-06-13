## ADDED Requirements

### Requirement: vue-i18n 国际化配置
应用 SHALL 使用 vue-i18n v9（Composition API 模式）管理所有 UI 文案，初始支持 zh-CN 和 en-US，新增语种只需添加 locale JSON 文件，无需修改组件代码。

#### Scenario: 语言切换
- **WHEN** 用户在设置中切换语言
- **THEN** 所有 UI 文案立即切换为目标语言，无需重启应用

#### Scenario: 默认语言
- **WHEN** 首次启动应用且未配置语言偏好
- **THEN** 应用跟随操作系统语言；若系统语言不在支持列表中，则默认 zh-CN

### Requirement: 禁止 UI 硬编码字符串
所有用户可见的 UI 文案 SHALL 通过 `t('key')` 获取，不得在组件模板或脚本中硬编码中文或英文字符串。ESLint 规则 `eslint-plugin-vue-i18n` 在 CI 中强制检查。

#### Scenario: ESLint 检测硬编码
- **WHEN** 开发者在 Vue 组件中写入未经 `t()` 包装的文案字符串
- **THEN** ESLint 报错，pre-commit hook 阻止提交

#### Scenario: locale 文件完整性
- **WHEN** zh-CN locale 文件中存在某个 key
- **THEN** en-US locale 文件中 SHALL 存在相同 key，缺失时 CI 检查失败

### Requirement: locale 文件结构
locale 文件 SHALL 按功能模块分层组织（如 `settings.apiKey.label`），禁止所有 key 平铺在顶层。

#### Scenario: 新增 locale key
- **WHEN** 开发者新增一个 UI 文案
- **THEN** key 按所属模块放入对应命名空间，并同时在所有支持语言的 locale 文件中添加
