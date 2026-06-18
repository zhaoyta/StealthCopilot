## MODIFIED Requirements

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
