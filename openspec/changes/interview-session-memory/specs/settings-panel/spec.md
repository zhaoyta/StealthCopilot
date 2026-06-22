## MODIFIED Requirements

### Requirement: 6 Tab 设置面板
设置面板 SHALL 包含 7 个 Tab：API 凭证、语言配置、设备绑定、简历管理、提词窗外观、历史会话、高级。Tab 切换无需保存操作，各 Tab 内改动实时写入 Keychain / 本地配置。

#### Scenario: Tab 切换保留状态
- **WHEN** 用户在多个 Tab 之间切换
- **THEN** 各 Tab 的表单内容保持不变，不触发保存或重置

## ADDED Requirements

### Requirement: 历史轮数高级配置
设置面板「高级」Tab SHALL 提供「历史轮数」数字输入（范围 1–20，默认 5），用于控制回答生成时注入 DeepSeek 的最大历史 Q&A 轮数。

#### Scenario: 设置历史轮数
- **WHEN** 用户在「历史轮数」输入框输入数值并离开焦点
- **THEN** 新值写入本地配置，后续回答生成最多注入对应数量的历史问答

#### Scenario: 历史轮数超出范围
- **WHEN** 用户输入的历史轮数不在 1–20 范围内
- **THEN** 系统自动夹紧到最近的边界值（1 或 20），不显示错误
