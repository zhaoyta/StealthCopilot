## ADDED Requirements

### Requirement: 5 步向导流程
应用首次启动时 SHALL 显示 Setup 向导，完成后标记为已初始化，后续启动直接进主界面。向导分 5 步：欢迎、依赖检测、API Key 录入、声音克隆、完成。

#### Scenario: 首次启动显示向导
- **WHEN** 应用启动且本地无初始化完成标记
- **THEN** 显示 Setup 向导第 1 步（欢迎页），不显示主界面

#### Scenario: 再次启动跳过向导
- **WHEN** 应用启动且初始化已完成
- **THEN** 直接进入主界面，不显示向导

### Requirement: 依赖检测与一键安装
Step 2 SHALL 检测 BlackHole 虚拟声卡和虚拟摄像头驱动是否已安装，缺失时提供一键安装按钮，安装时显示进度条。

#### Scenario: 依赖已安装
- **WHEN** 检测到 BlackHole 和虚拟摄像头驱动均已安装
- **THEN** 两项均显示"已安装"绿色状态，可直接进入下一步

#### Scenario: 一键安装缺失依赖
- **WHEN** 用户点击缺失依赖的"一键安装"按钮
- **THEN** 触发系统 admin 授权弹窗，授权后显示安装进度条，完成后状态变为"已安装"

### Requirement: 核心 API Key 录入
Step 3 SHALL 仅要求填写讯飞和 DeepSeek 两项必填 Key，ElevenLabs 和 Simli AI 标记为可选，附说明"可稍后在设置中补充"。

#### Scenario: 填写必填 Key 后可继续
- **WHEN** 用户填写了讯飞和 DeepSeek 的 API Key
- **THEN** 下一步按钮可点击

#### Scenario: 跳过可选 Key
- **WHEN** 用户未填写 ElevenLabs 或 Simli AI Key
- **THEN** 仍可进入下一步，相关管道功能在设置补全前降级或禁用

### Requirement: 声音克隆录制
Step 4 SHALL 在 App 内提供录音界面，用户朗读示例文本约 15 秒，完成后自动上传至 ElevenLabs（使用用户自己的 API Key）并获取 Voice ID，存入 Keychain。

#### Scenario: 录制并上传成功
- **WHEN** 用户完成录音并点击提交
- **THEN** 显示上传进度，成功后显示"音色克隆完成"，Voice ID 自动存入 Keychain

#### Scenario: 跳过声音克隆
- **WHEN** 用户未填写 ElevenLabs Key 或主动跳过
- **THEN** Step 4 显示跳过选项，说话链降级为系统默认 TTS，可稍后在设置中补全
