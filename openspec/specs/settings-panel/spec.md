# Capability Spec: settings-panel

## Purpose

提供 6 Tab 的设置面板 UI，涵盖 API 凭证、语言配置、设备绑定、简历管理、提词窗外观和高级配置，所有改动实时写入 Keychain 或本地配置，无需手动保存。

---

## Requirements

### Requirement: 6 Tab 设置面板
设置面板 SHALL 包含 6 个 Tab：API 凭证、语言配置、设备绑定、简历管理、提词窗外观、高级。Tab 切换无需保存操作，各 Tab 内改动实时写入 Keychain / 本地配置。

#### Scenario: Tab 切换保留状态
- **WHEN** 用户在 Tab 间切换
- **THEN** 各 Tab 的输入状态保留，不重置

### Requirement: API 凭证 Tab
每个服务（讯飞 / DeepSeek / ElevenLabs / Simli AI）SHALL 提供密码输入框（可切换显示/隐藏）和"连接测试"按钮，测试结果显示"已连接"或"失败"状态徽章。

#### Scenario: 连接测试
- **WHEN** 用户点击某服务的"连接测试"按钮
- **THEN** 调用对应服务的健康检查接口，2s 内显示结果（已连接 / 失败）

#### Scenario: Key 更新后自动失效测试状态
- **WHEN** 用户修改某服务的 API Key 内容
- **THEN** 该服务的连接状态徽章重置为未测试状态

### Requirement: 语言配置 Tab
听力链和说话链的语言方向 SHALL 独立配置，各提供"源语言"和"目标语言"下拉，选项为讯飞支持的语言列表。

#### Scenario: 独立配置两条链的语言
- **WHEN** 用户修改听力链语言
- **THEN** 说话链语言不受影响，反之亦然

### Requirement: 设备绑定 Tab
系统音视频设备 SHALL 动态枚举，提供"重新枚举"按钮，每类设备（虚拟声卡 / 物理麦克风 / 物理摄像头 / 虚拟摄像头）各一个下拉选择框。

#### Scenario: 重新枚举设备
- **WHEN** 用户点击"重新枚举"按钮
- **THEN** 调用 Go 后端重新扫描系统设备，下拉列表刷新

### Requirement: 高级 Tab Prompt 配置
高级 Tab SHALL 以折叠面板展示 RAG 回答生成 Prompt 和说话链润色 Prompt，各提供多行文本框和"恢复默认"按钮，默认值由 Go 后端常量提供。

#### Scenario: 恢复默认 Prompt
- **WHEN** 用户点击"恢复默认"按钮
- **THEN** 对应文本框内容恢复为 Go 后端硬编码的默认值
