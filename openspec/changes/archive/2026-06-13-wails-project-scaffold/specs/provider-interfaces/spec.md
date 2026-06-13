## ADDED Requirements

### Requirement: STT Provider 接口
系统 SHALL 定义 `STTProvider` Go interface，抽象语音转文字能力，当前实现为讯飞 ASR，未来可替换为其他供应商或自营服务。

#### Scenario: Provider 可替换
- **WHEN** 需要切换 STT 供应商
- **THEN** 只需实现 `STTProvider` interface 并在配置中切换，无需修改调用方代码

#### Scenario: 接口定义完整性
- **WHEN** 编译项目
- **THEN** 所有 Provider interface 编译通过，各领域包仅依赖 interface，不依赖具体实现

### Requirement: Translation Provider 接口
系统 SHALL 定义 `TranslationProvider` Go interface，抽象实时语音翻译能力（输入音频流，输出源文本和目标文本），初始实现为讯飞实时语音翻译 API。

#### Scenario: 双输出流
- **WHEN** TranslationProvider 收到音频流
- **THEN** 同时产出 `SrcText`（原文）和 `DstText`（译文）两路输出，通过 channel 或 callback 传递

### Requirement: TTS Provider 接口
系统 SHALL 定义 `TTSProvider` Go interface，抽象流式语音合成能力，初始实现为 ElevenLabs，接口支持流式音频输出（chunk by chunk）。

#### Scenario: 流式输出
- **WHEN** TTSProvider 收到文本输入
- **THEN** 以音频 chunk 流形式输出，首个 chunk 在完整文本生成前即可播放

### Requirement: LipSync Provider 接口
系统 SHALL 定义 `LipSyncProvider` Go interface，抽象实时口型同步能力，初始实现为 Simli AI API，预留 `StealthCloudProvider` 实现位置。

#### Scenario: Provider 切换
- **WHEN** 用户在设置中选择口型同步服务商（Simli / StealthCloud）
- **THEN** 系统在运行时切换 LipSyncProvider 实现，无需重启

### Requirement: 所有 Provider 通过配置注入
所有 Provider 实现 SHALL 通过依赖注入（DI）在应用启动时根据配置实例化，禁止在业务逻辑代码中直接 `new` 具体实现。

#### Scenario: 启动时 Provider 初始化
- **WHEN** 应用启动
- **THEN** 根据 config 中的 provider 选择，实例化对应的实现并注入到各 Pipeline
