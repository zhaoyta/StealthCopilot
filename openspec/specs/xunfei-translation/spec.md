## ADDED Requirements

### Requirement: 讯飞实时语音翻译 WebSocket 接入
系统 SHALL 建立与讯飞实时语音翻译 API 的 WebSocket 长连接，持续发送虚拟声卡音频流，接收 src_text（源语言原文）和 dst_text（目标语言译文）双路输出。

#### Scenario: 建立连接
- **WHEN** 用户启动听力链
- **THEN** Go 后端在 2s 内与讯飞 WebSocket 建立连接，并开始发送音频数据

#### Scenario: 双路输出并行分发
- **WHEN** 讯飞返回包含 src_text 和 dst_text 的消息
- **THEN** Go 后端同时将 dst_text 推送至提词窗字幕区，将 src_text 发至意图识别模块，两路不互相阻塞

#### Scenario: 断连自动重连
- **WHEN** WebSocket 连接意外断开
- **THEN** Go 后端以指数退避（1s、2s、4s）最多重试 3 次；3 次失败后通过 Wails EventEmit 通知前端显示"连接中断"

#### Scenario: 语言配置动态生效
- **WHEN** 用户在设置中修改听力链语言对
- **THEN** 下次启动听力链时使用新语言配置，无需重启应用
