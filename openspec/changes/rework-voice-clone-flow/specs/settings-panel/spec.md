## MODIFIED Requirements

### Requirement: API 凭证配置
设置面板 SHALL 为讯飞 RTASR、讯飞机器翻译、讯飞声音复刻、DeepSeek、Simli AI 提供凭证输入和连接测试。讯飞声音复刻的用户可编辑凭证 SHALL 只有 App ID、API Key、API Secret。Task ID 和 Asset ID 是声音复刻流程产物，SHALL NOT 在服务密钥页提供手填入口。

#### Scenario: 测试讯飞声音复刻凭证
- **WHEN** 用户点击讯飞声音复刻连接测试
- **THEN** 应用验证训练接口是否可用，并在缺少 Asset ID 时提示"凭证可用，尚未完成音色训练"

#### Scenario: 保存训练状态字段
- **WHEN** 训练提交返回 Task ID 或训练完成返回 Asset ID
- **THEN** 应用通过 Keychain 保存对应字段，声音复刻流程页显示训练状态，服务密钥页不显示原始 ID 输入框
