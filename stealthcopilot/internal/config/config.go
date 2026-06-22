package config

import "strings"

// keyring key 常量 —— 严禁在代码中硬编码字符串
const (
	keyXunfeiSimultAppID     = "xunfei_simult_app_id"
	keyXunfeiSimultAPIKey    = "xunfei_simult_api_key"
	keyXunfeiSimultAPISecret = "xunfei_simult_api_secret"
	keyXunfeiTTSAppID        = "xunfei_tts_app_id"
	keyXunfeiTTSAPIKey       = "xunfei_tts_api_key"
	keyXunfeiTTSAPISecret    = "xunfei_tts_api_secret"
	keyXunfeiTTSAssetID      = "xunfei_tts_asset_id"
	keyXunfeiTTSTaskID       = "xunfei_tts_task_id"
	keyDeepSeekKey           = "deepseek_key"
	keyZegoDigitalHumanAppID = "zego_digital_human_app_id"
	keyZegoServerSecret      = "zego_digital_human_server_secret"
	keySimliAPIKey           = "simli_api_key"
)

// 默认值常量
const (
	DefaultGhostFontSize      = 16
	DefaultGhostOpacity       = 0.85
	DefaultGhostPosition      = "bottom-right"
	DefaultDeepSeekModel      = "deepseek-chat"
	DefaultLLMBaseURL         = "https://api.deepseek.com/v1"
	DefaultHearingSourceLang  = "en"
	DefaultHearingTargetLang  = "zh"
	DefaultSpeakingInputLang  = "zh"
	DefaultSpeakingOutputLang = "en"
	DefaultMonitorVolume      = 80
	DefaultMonitorRate        = 0
	DefaultHistoryMaxTurns    = 5
)

// DefaultRAGPrompt 是 RAG 回答生成的默认提示词，Go 后端硬编码，前端不存储。
const DefaultRAGPrompt = `你是一位专业的面试助手。根据以下简历内容和面试官的问题，生成简洁、专业的英文回答建议（不超过3句话）。

简历内容：
{resume_context}

面试官问题：
{question}

请生成回答建议：`

// DefaultSpeakPolishPrompt 是说话链润色的默认提示词。
const DefaultSpeakPolishPrompt = `将以下中文内容翻译并润色为流利、专业的英文，适合在技术面试场景中使用。保持原意，去掉口语化表达，不超过3句话。

中文内容：
{input}

英文输出：`

// AppConfig 是应用运行时的完整内存配置，由 Manager 在启动时从 Keychain + 本地文件加载。
type AppConfig struct {
	// API 密钥（来自 Keychain）
	XunfeiSimultAppID     string
	XunfeiSimultAPIKey    string
	XunfeiSimultAPISecret string
	XunfeiTTSAppID        string
	XunfeiTTSAPIKey       string
	XunfeiTTSAPISecret    string
	XunfeiTTSAssetID      string
	XunfeiTTSTaskID       string
	DeepSeekKey           string
	DeepSeekModel         string
	LLMBaseURL            string
	ZegoDigitalHumanAppID string
	ZegoServerSecret      string

	// Provider 选择
	HearingASRProvider    TranslationProviderType
	HearingTransProvider  TranslationProviderType
	HearingTTSProvider    TTSProviderType
	SpeakingASRProvider   TranslationProviderType
	SpeakingTransProvider TranslationProviderType
	SpeakingTTSProvider   TTSProviderType
	LLMProvider           LLMProviderType
	EmbeddingProvider     EmbeddingProviderType
	DigitalHumanEnabled   bool
	DigitalHumanProvider  DigitalHumanProviderType
	// Simli AI 数字人配置（API Key 存 Keychain，FaceID 存本地文件）
	SimliAPIKey  string
	SimaliFaceID string
	// ZEGO 数字人配置（企业级）
	ZegoDigitalHumanID string
	ZegoRoomID         string
	ZegoStreamID       string
	ZegoRTMPPullURL    string

	// 语言设置
	HearingSourceLang  string
	HearingTargetLang  string
	SpeakingInputLang  string
	SpeakingOutputLang string

	// 设备绑定
	VirtualMicName    string
	PhysicalMicName   string
	VirtualCamName    string
	MonitorOutputName string

	// 听力链译文耳机播报
	HearingMonitorEnabled bool
	HearingMonitorVolume  int
	HearingMonitorRate    int

	// 提词窗外观
	GhostFontSize int
	GhostOpacity  float64
	GhostPosition string

	// 高级 Prompt
	RAGPrompt         string
	SpeakPolishPrompt string
	PolishEnabled     bool
	HistoryMaxTurns   int

	// 界面语言
	UILocale string // "zh-CN" | "en-US"

	// 初始化状态
	SetupCompleted bool
	ActiveResumeID string
}

// Manager 协调配置的加载与保存，在应用启动时完成 Keychain 预读。
type Manager struct {
	store  *KeyringStore
	local  *localStore
	Config *AppConfig
}

// NewManager 创建 Manager 并从 Keychain + 本地文件完整加载配置。
// dataDir 是应用数据目录（用于存储本地 JSON 配置文件）。
func NewManager(dataDir string) (*Manager, error) {
	ls, err := newLocalStore(dataDir)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		store:  NewKeyringStore(),
		local:  ls,
		Config: &AppConfig{},
	}
	m.reload()
	return m, nil
}

// reload 从 Keychain 和本地文件重新加载所有配置到内存。
// 任何读取错误均静默处理（返回空字符串）以避免阻塞启动。
func (m *Manager) reload() {
	m.Config.XunfeiSimultAppID, _ = m.store.Get(keyXunfeiSimultAppID)
	m.Config.XunfeiSimultAPIKey, _ = m.store.Get(keyXunfeiSimultAPIKey)
	m.Config.XunfeiSimultAPISecret, _ = m.store.Get(keyXunfeiSimultAPISecret)
	m.Config.XunfeiTTSAppID, _ = m.store.Get(keyXunfeiTTSAppID)
	m.Config.XunfeiTTSAPIKey, _ = m.store.Get(keyXunfeiTTSAPIKey)
	m.Config.XunfeiTTSAPISecret, _ = m.store.Get(keyXunfeiTTSAPISecret)
	m.Config.XunfeiTTSAssetID, _ = m.store.Get(keyXunfeiTTSAssetID)
	m.Config.XunfeiTTSTaskID, _ = m.store.Get(keyXunfeiTTSTaskID)
	m.Config.DeepSeekKey, _ = m.store.Get(keyDeepSeekKey)
	m.Config.ZegoDigitalHumanAppID, _ = m.store.Get(keyZegoDigitalHumanAppID)
	m.Config.ZegoServerSecret, _ = m.store.Get(keyZegoServerSecret)
	m.Config.SimliAPIKey, _ = m.store.Get(keySimliAPIKey)

	lc := m.local.load()
	m.applyLocalConfig(lc)
}

// applyLocalConfig 将本地文件配置合并到内存 AppConfig，空值使用默认值填充。
func (m *Manager) applyLocalConfig(lc LocalConfig) {
	m.Config.SetupCompleted = lc.SetupCompleted
	m.Config.ActiveResumeID = lc.ActiveResumeID
	m.Config.UILocale = stringOr(lc.UILocale, "zh-CN")
	m.Config.DeepSeekModel = stringOr(lc.DeepSeekModel, DefaultDeepSeekModel)
	m.Config.LLMBaseURL = stringOr(lc.LLMBaseURL, DefaultLLMBaseURL)
	m.Config.HearingASRProvider = translationProviderOr(lc.HearingASRProvider, TranslationProviderXunfeiSimult)
	m.Config.HearingTransProvider = translationProviderOr(lc.HearingTransProvider, TranslationProviderXunfeiSimult)
	m.Config.HearingTTSProvider = hearingTTSProviderOr(lc.HearingTTSProvider, TTSProviderSystem)
	m.Config.SpeakingASRProvider = translationProviderOr(lc.SpeakingASRProvider, TranslationProviderXunfeiSimult)
	m.Config.SpeakingTransProvider = translationProviderOr(lc.SpeakingTransProvider, TranslationProviderXunfeiSimult)
	m.Config.LLMProvider = llmProviderOr(lc.LLMProvider, LLMProviderDeepSeek)
	m.Config.SpeakingTTSProvider = ttsProviderOr(lc.SpeakingTTSProvider, TTSProviderSystem)
	m.Config.EmbeddingProvider = embeddingProviderOr(lc.EmbeddingProvider, EmbeddingProviderPythonBridge)
	m.Config.DigitalHumanEnabled = lc.DigitalHumanEnabled
	m.Config.DigitalHumanProvider = digitalHumanProviderOr(lc.DigitalHumanProvider, DigitalHumanProviderSimli)
	m.Config.SimaliFaceID = lc.SimaliFaceID
	m.Config.ZegoDigitalHumanID = lc.ZegoDigitalHumanID
	m.Config.ZegoRoomID = lc.ZegoRoomID
	m.Config.ZegoStreamID = lc.ZegoStreamID
	m.Config.ZegoRTMPPullURL = lc.ZegoRTMPPullURL
	m.Config.HearingSourceLang = stringOr(lc.HearingSourceLang, DefaultHearingSourceLang)
	m.Config.HearingTargetLang = stringOr(lc.HearingTargetLang, DefaultHearingTargetLang)
	m.Config.SpeakingInputLang = stringOr(lc.SpeakingInputLang, DefaultSpeakingInputLang)
	m.Config.SpeakingOutputLang = stringOr(lc.SpeakingOutputLang, DefaultSpeakingOutputLang)
	m.Config.VirtualMicName = lc.VirtualMicName
	m.Config.PhysicalMicName = lc.PhysicalMicName
	m.Config.VirtualCamName = lc.VirtualCamName
	m.Config.MonitorOutputName = lc.MonitorOutputName
	m.Config.HearingMonitorEnabled = lc.HearingMonitorEnabled
	m.Config.HearingMonitorVolume = intOr(lc.HearingMonitorVolume, DefaultMonitorVolume)
	m.Config.HearingMonitorRate = lc.HearingMonitorRate
	m.Config.GhostFontSize = intOr(lc.GhostFontSize, DefaultGhostFontSize)
	m.Config.GhostOpacity = floatOr(lc.GhostOpacity, DefaultGhostOpacity)
	m.Config.GhostPosition = stringOr(lc.GhostPosition, DefaultGhostPosition)
	m.Config.RAGPrompt = stringOr(lc.RAGPrompt, DefaultRAGPrompt)
	m.Config.SpeakPolishPrompt = stringOr(lc.SpeakPolishPrompt, DefaultSpeakPolishPrompt)
	m.Config.PolishEnabled = lc.PolishEnabled
	m.Config.HistoryMaxTurns = clampInt(intOr(lc.HistoryMaxTurns, DefaultHistoryMaxTurns), 1, 20)
}

// SaveAPIKey 将单个 API Key 写入 Keychain 并同步内存配置。
// service 为服务名（xunfei_simult/xunfei_tts/deepseek/zego_digital_human），field 为字段名。
func (m *Manager) SaveAPIKey(service, field, value string) error {
	value = strings.TrimSpace(value)
	key := service + "_" + field
	if key == keyXunfeiTTSAssetID || key == keyXunfeiTTSTaskID {
		return errInternalVoiceCloneField(key)
	}
	return m.saveKey(key, value)
}

// SaveXunfeiTTSTaskID 保存声音复刻训练任务 ID，仅供声音复刻流程内部调用。
func (m *Manager) SaveXunfeiTTSTaskID(taskID string) error {
	return m.saveKey(keyXunfeiTTSTaskID, strings.TrimSpace(taskID))
}

// SaveXunfeiTTSAssetID 保存声音复刻训练完成后的 Asset ID，仅供声音复刻流程内部调用。
func (m *Manager) SaveXunfeiTTSAssetID(assetID string) error {
	return m.saveKey(keyXunfeiTTSAssetID, strings.TrimSpace(assetID))
}

func (m *Manager) saveKey(key, value string) error {
	if err := m.store.Set(key, value); err != nil {
		return err
	}
	// 同步到内存
	switch key {
	case keyXunfeiSimultAppID:
		m.Config.XunfeiSimultAppID = value
	case keyXunfeiSimultAPIKey:
		m.Config.XunfeiSimultAPIKey = value
	case keyXunfeiSimultAPISecret:
		m.Config.XunfeiSimultAPISecret = value
	case keyXunfeiTTSAppID:
		m.Config.XunfeiTTSAppID = value
	case keyXunfeiTTSAPIKey:
		m.Config.XunfeiTTSAPIKey = value
	case keyXunfeiTTSAPISecret:
		m.Config.XunfeiTTSAPISecret = value
	case keyXunfeiTTSAssetID:
		m.Config.XunfeiTTSAssetID = value
	case keyXunfeiTTSTaskID:
		m.Config.XunfeiTTSTaskID = value
	case keyDeepSeekKey:
		m.Config.DeepSeekKey = value
	case keyZegoDigitalHumanAppID:
		m.Config.ZegoDigitalHumanAppID = value
	case keyZegoServerSecret:
		m.Config.ZegoServerSecret = value
	case keySimliAPIKey:
		m.Config.SimliAPIKey = value
	}
	return nil
}

type internalVoiceCloneFieldError string

func errInternalVoiceCloneField(key string) error {
	return internalVoiceCloneFieldError(key)
}

func (e internalVoiceCloneFieldError) Error() string {
	return "声音复刻训练状态由系统自动保存，不能在密钥配置中手动填写：" + string(e)
}

// SaveLocalConfig 将非敏感配置写入磁盘并同步内存。
func (m *Manager) SaveLocalConfig(lc LocalConfig) error {
	if err := m.local.save(lc); err != nil {
		return err
	}
	m.applyLocalConfig(lc)
	return nil
}

// ToLocalConfig 将当前内存配置转换为 LocalConfig（用于持久化）。
func (m *Manager) ToLocalConfig() LocalConfig {
	return LocalConfig{
		SetupCompleted:        m.Config.SetupCompleted,
		ActiveResumeID:        m.Config.ActiveResumeID,
		UILocale:              m.Config.UILocale,
		DeepSeekModel:         m.Config.DeepSeekModel,
		LLMBaseURL:            m.Config.LLMBaseURL,
		HearingASRProvider:    m.Config.HearingASRProvider,
		HearingTransProvider:  m.Config.HearingTransProvider,
		HearingTTSProvider:    m.Config.HearingTTSProvider,
		SpeakingASRProvider:   m.Config.SpeakingASRProvider,
		SpeakingTransProvider: m.Config.SpeakingTransProvider,
		SpeakingTTSProvider:   m.Config.SpeakingTTSProvider,
		LLMProvider:           m.Config.LLMProvider,
		EmbeddingProvider:     m.Config.EmbeddingProvider,
		DigitalHumanEnabled:   m.Config.DigitalHumanEnabled,
		DigitalHumanProvider:  m.Config.DigitalHumanProvider,
		SimaliFaceID:          m.Config.SimaliFaceID,
		ZegoDigitalHumanID:    m.Config.ZegoDigitalHumanID,
		ZegoRoomID:            m.Config.ZegoRoomID,
		ZegoStreamID:          m.Config.ZegoStreamID,
		ZegoRTMPPullURL:       m.Config.ZegoRTMPPullURL,
		HearingSourceLang:     m.Config.HearingSourceLang,
		HearingTargetLang:     m.Config.HearingTargetLang,
		SpeakingInputLang:     m.Config.SpeakingInputLang,
		SpeakingOutputLang:    m.Config.SpeakingOutputLang,
		VirtualMicName:        m.Config.VirtualMicName,
		PhysicalMicName:       m.Config.PhysicalMicName,
		VirtualCamName:        m.Config.VirtualCamName,
		MonitorOutputName:     m.Config.MonitorOutputName,
		HearingMonitorEnabled: m.Config.HearingMonitorEnabled,
		HearingMonitorVolume:  m.Config.HearingMonitorVolume,
		HearingMonitorRate:    m.Config.HearingMonitorRate,
		GhostFontSize:         m.Config.GhostFontSize,
		GhostOpacity:          m.Config.GhostOpacity,
		GhostPosition:         m.Config.GhostPosition,
		RAGPrompt:             m.Config.RAGPrompt,
		SpeakPolishPrompt:     m.Config.SpeakPolishPrompt,
		PolishEnabled:         m.Config.PolishEnabled,
		HistoryMaxTurns:       m.Config.HistoryMaxTurns,
	}
}

func stringOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func intOr(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func floatOr(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

func translationProviderOr(v TranslationProviderType, def TranslationProviderType) TranslationProviderType {
	switch v {
	case TranslationProviderXunfeiSimult, TranslationProviderNull:
		return v
	default:
		return def
	}
}

func llmProviderOr(v LLMProviderType, def LLMProviderType) LLMProviderType {
	switch v {
	case LLMProviderDeepSeek, LLMProviderOpenAICompatible:
		return v
	default:
		return def
	}
}

func ttsProviderOr(v TTSProviderType, def TTSProviderType) TTSProviderType {
	switch v {
	case TTSProviderXunfeiVoiceClone, TTSProviderSystem, TTSProviderNull:
		return v
	default:
		return def
	}
}

func hearingTTSProviderOr(v TTSProviderType, def TTSProviderType) TTSProviderType {
	switch v {
	case TTSProviderSystem, TTSProviderNull:
		return v
	default:
		return def
	}
}

func embeddingProviderOr(v EmbeddingProviderType, def EmbeddingProviderType) EmbeddingProviderType {
	switch v {
	case EmbeddingProviderPythonBridge, EmbeddingProviderNull:
		return v
	default:
		return def
	}
}

func digitalHumanProviderOr(v DigitalHumanProviderType, def DigitalHumanProviderType) DigitalHumanProviderType {
	switch v {
	case DigitalHumanProviderSimli, DigitalHumanProviderZego:
		return v
	default:
		return def
	}
}
