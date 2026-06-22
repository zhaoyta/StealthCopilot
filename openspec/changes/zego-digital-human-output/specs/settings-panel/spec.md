## MODIFIED Requirements

### Requirement: API 凭证 Tab
每个服务（讯飞 RTASR / 讯飞机器翻译 / 讯飞声音复刻 / DeepSeek / Simli AI）SHALL 提供密码输入框（可切换显示/隐藏）和"连接测试"按钮，测试结果显示"已连接"或"失败"状态徽章。讯飞声音复刻的用户可编辑凭证 SHALL 只有 App ID、API Key、API Secret。Task ID 和 Asset ID 是声音复刻流程产物，SHALL NOT 在服务密钥页提供手填入口。

#### Scenario: 连接测试
- **WHEN** 用户点击某服务的"连接测试"按钮
- **THEN** 调用对应服务的健康检查接口，2s 内显示结果（已连接 / 失败）

#### Scenario: Key 更新后自动失效测试状态
- **WHEN** 用户修改某服务的 API Key 内容
- **THEN** 该服务的连接状态徽章重置为未测试状态

#### Scenario: Simli 数字人凭证保存
- **WHEN** 用户保存 Simli API Key
- **THEN** 系统将凭证写入 Keychain，并在设置页只显示已配置状态，不回显明文

## ADDED Requirements

### Requirement: 数字人配置设置
设置面板 SHALL 提供 Simli 数字人非敏感配置入口，包括 Face ID、OBS Browser Source URL、会议虚拟摄像头名称和说话链数字人输出默认开关。

#### Scenario: 保存数字人流配置
- **WHEN** 用户填写 Simli Face ID 和会议虚拟摄像头名称并保存
- **THEN** 系统将这些非敏感配置写入本地配置文件，后续说话链启动数字人模式时使用这些值

#### Scenario: 展示 OBS 输出地址
- **WHEN** 用户查看数字人配置
- **THEN** 页面显示 OBS Browser Source URL `http://127.0.0.1:18765/`，并提示在 OBS 中添加浏览器源后启动 OBS Virtual Camera

#### Scenario: 数字人默认开关
- **WHEN** 用户在设置中启用或关闭数字人输出默认开关
- **THEN** 首页说话链数字人开关默认状态与该设置保持一致
