## Why

声音复刻流程已经从早期 ElevenLabs 方案切换为讯飞声音复刻，但当前主规格中仍混用 ElevenLabs、Voice ID、系统 TTS 降级等旧口径。用户在首次设置、设置面板、说话链启动前看到的准备条件也不够一致，容易误解“提交训练”和“可用于说话链输出”之间的差异。

## What Changes

- 将声音复刻统一为讯飞声音复刻：训练阶段使用 AppID/API Key/API Secret 获取训练文本、创建任务并上传录音；完成后保存 Asset ID。
- 将说话链 TTS 统一为“默认音色 / 个人复刻音色”两种模式：默认音色无需训练即可输出；个人复刻音色在 Asset ID 可用后使用讯飞声音复刻流式 TTS。
- 把 Setup 向导 Step 4 明确拆成“获取训练文本 → 录音 → 提交训练 → 查询状态 → 保存 Asset ID”。
- 将设置面板的用户凭证项从 ElevenLabs 迁移到讯飞声音复刻 AppID/API Key/API Secret；Task ID 和 Asset ID 仅作为声音复刻流程内部状态保存，不提供手填入口。
- 明确训练未完成时的行为：说话链应使用默认音色输出，且不得伪装成已完成个人复刻音色输出。

## Capabilities

### Modified Capabilities

- `setup-wizard`: 声音复刻录制从 ElevenLabs Voice ID 流程改为讯飞训练任务 + Asset ID 流程。
- `settings-panel`: 服务密钥从 ElevenLabs 项改为讯飞声音复刻凭证项；不展示 Task ID/Asset ID 输入框。
- `tts-output`: TTS Provider 初始实现从 ElevenLabs 改为默认音色输出 + 讯飞声音复刻个人音色输出。
- `provider-interfaces`: TTSProvider 保持流式 chunk 接口，但默认实现与术语统一为讯飞声音复刻。
- `prd` / `adr` / `docs`: 同步产品流程与帮助文档。

## Impact

- 规格和文档更新：`openspec/specs/**`、`docs/guide/**`、`README.md` 中的声音复刻流程统一为讯飞。
- 现有代码入口复用：`CloneVoice`、`GetXunfeiVoiceTrainText`、`QueryXunfeiVoiceCloneStatus`、`XunfeiVoiceCloneProvider`。
- 后续实现风险集中在训练状态体验和音色选择：默认音色 Provider、手动查询、失败重试、已提交任务恢复、Asset ID 保存后的配置刷新。
