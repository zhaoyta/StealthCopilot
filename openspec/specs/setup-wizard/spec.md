# Capability Spec: setup-wizard

## Purpose

提供首次启动的 5 步引导向导，帮助用户完成环境依赖检测、API Key 录入和声音复刻录制，完成后标记初始化状态，后续启动直接进入主界面。

---

## Requirements

### Requirement: 5 步向导流程
应用首次启动时 SHALL 显示 Setup 向导，完成后标记为已初始化，后续启动直接进主界面。向导分 5 步：欢迎、依赖检测、API Key 录入、声音复刻、完成。

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
Step 3 SHALL 要求填写讯飞 RTASR、讯飞机器翻译、讯飞声音复刻和 DeepSeek 的必填凭证，Simli AI 标记为可选，附说明"可稍后在设置中补充"。

#### Scenario: 填写必填 Key 后可继续
- **WHEN** 用户填写了讯飞 RTASR、讯飞机器翻译、讯飞声音复刻和 DeepSeek 的必填凭证
- **THEN** 下一步按钮可点击

#### Scenario: 跳过可选 Key
- **WHEN** 用户未填写 Simli AI Key
- **THEN** 仍可进入下一步，视频口型同步功能在设置补全前降级或禁用

### Requirement: 声音复刻录制
Step 4 SHALL 在 App 内提供讯飞声音复刻录音界面，用户按讯飞训练文本录音，提交后创建训练任务并保存 Task ID；训练完成并返回 Asset ID 后，Asset ID SHALL 存入 Keychain。

#### Scenario: 获取训练文本
- **WHEN** 用户进入声音复刻步骤且讯飞声音复刻凭证完整
- **THEN** 应用请求讯飞训练文本并展示给用户朗读

#### Scenario: 提交训练成功
- **WHEN** 用户完成录音并点击提交训练
- **THEN** 应用上传录音，保存返回的 Task ID，并显示"训练已提交"

#### Scenario: 查询训练完成
- **WHEN** 用户查询训练状态且讯飞返回成功 Asset ID
- **THEN** 应用保存 Asset ID，显示"声音复刻成功"，说话链可使用克隆音色

#### Scenario: 查询训练中
- **WHEN** 用户查询训练状态且训练尚未完成
- **THEN** 应用保持"训练已提交"状态，并提示稍后继续查询

#### Scenario: 跳过声音复刻
- **WHEN** 用户主动跳过
- **THEN** Step 4 显示跳过状态，说话链使用默认音色输出，可稍后在设置中补全个人复刻音色
