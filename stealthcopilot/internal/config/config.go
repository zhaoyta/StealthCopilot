package config

import "strings"

// keyring key 常量 —— 严禁在代码中硬编码字符串
const (
	keyXunfeiRTASRAppID   = "xunfei_rtasr_app_id"
	keyXunfeiRTASRAPIKey  = "xunfei_rtasr_api_key"
	keyXunfeiMTAppID      = "xunfei_mt_app_id"
	keyXunfeiMTAPIKey     = "xunfei_mt_api_key"
	keyXunfeiMTAPISecret  = "xunfei_mt_api_secret"
	keyXunfeiTTSAppID     = "xunfei_tts_app_id"
	keyXunfeiTTSAPIKey    = "xunfei_tts_api_key"
	keyXunfeiTTSAPISecret = "xunfei_tts_api_secret"
	keyXunfeiTTSAssetID   = "xunfei_tts_asset_id"
	keyXunfeiTTSTaskID    = "xunfei_tts_task_id"
	keyDeepSeekKey        = "deepseek_key"
	keySimliKey           = "simli_key"
	keySimliFaceID        = "simli_face_id"
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
	XunfeiRTASRAppID   string
	XunfeiRTASRAPIKey  string
	XunfeiMTAppID      string
	XunfeiMTAPIKey     string
	XunfeiMTAPISecret  string
	XunfeiTTSAppID     string
	XunfeiTTSAPIKey    string
	XunfeiTTSAPISecret string
	XunfeiTTSAssetID   string
	XunfeiTTSTaskID    string
	DeepSeekKey        string
	DeepSeekModel      string
	LLMBaseURL         string
	SimliKey           string
	SimliFaceID        string

	// Provider 选择
	TranslationProvider TranslationProviderType
	LLMProvider         LLMProviderType
	TTSProvider         TTSProviderType
	LipSyncProvider     LipSyncProviderType
	EmbeddingProvider   EmbeddingProviderType

	// 语言设置
	HearingSourceLang  string
	HearingTargetLang  string
	SpeakingInputLang  string
	SpeakingOutputLang string

	// 设备绑定
	VirtualMicName    string
	PhysicalMicName   string
	PhysicalCamName   string
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
	m.Config.XunfeiRTASRAppID, _ = m.store.Get(keyXunfeiRTASRAppID)
	m.Config.XunfeiRTASRAPIKey, _ = m.store.Get(keyXunfeiRTASRAPIKey)
	m.Config.XunfeiMTAppID, _ = m.store.Get(keyXunfeiMTAppID)
	m.Config.XunfeiMTAPIKey, _ = m.store.Get(keyXunfeiMTAPIKey)
	m.Config.XunfeiMTAPISecret, _ = m.store.Get(keyXunfeiMTAPISecret)
	m.Config.XunfeiTTSAppID, _ = m.store.Get(keyXunfeiTTSAppID)
	m.Config.XunfeiTTSAPIKey, _ = m.store.Get(keyXunfeiTTSAPIKey)
	m.Config.XunfeiTTSAPISecret, _ = m.store.Get(keyXunfeiTTSAPISecret)
	m.Config.XunfeiTTSAssetID, _ = m.store.Get(keyXunfeiTTSAssetID)
	m.Config.XunfeiTTSTaskID, _ = m.store.Get(keyXunfeiTTSTaskID)
	m.Config.DeepSeekKey, _ = m.store.Get(keyDeepSeekKey)
	m.Config.SimliKey, _ = m.store.Get(keySimliKey)
	m.Config.SimliFaceID, _ = m.store.Get(keySimliFaceID)

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
	m.Config.TranslationProvider = translationProviderOr(lc.TranslationProvider, TranslationProviderXunfei)
	m.Config.LLMProvider = llmProviderOr(lc.LLMProvider, LLMProviderDeepSeek)
	m.Config.TTSProvider = ttsProviderOr(lc.TTSProvider, TTSProviderSystem)
	m.Config.LipSyncProvider = lipSyncProviderOr(lc.LipSyncProvider, LipSyncProviderSimli)
	m.Config.EmbeddingProvider = embeddingProviderOr(lc.EmbeddingProvider, EmbeddingProviderPythonBridge)
	m.Config.HearingSourceLang = stringOr(lc.HearingSourceLang, DefaultHearingSourceLang)
	m.Config.HearingTargetLang = stringOr(lc.HearingTargetLang, DefaultHearingTargetLang)
	m.Config.SpeakingInputLang = stringOr(lc.SpeakingInputLang, DefaultSpeakingInputLang)
	m.Config.SpeakingOutputLang = stringOr(lc.SpeakingOutputLang, DefaultSpeakingOutputLang)
	m.Config.VirtualMicName = lc.VirtualMicName
	m.Config.PhysicalMicName = lc.PhysicalMicName
	m.Config.PhysicalCamName = lc.PhysicalCamName
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
}

// SaveAPIKey 将单个 API Key 写入 Keychain 并同步内存配置。
// service 为服务名（xunfei_rtasr/xunfei_mt/xunfei_tts/deepseek/simli），field 为字段名。
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
	case keyXunfeiRTASRAppID:
		m.Config.XunfeiRTASRAppID = value
	case keyXunfeiRTASRAPIKey:
		m.Config.XunfeiRTASRAPIKey = value
	case keyXunfeiMTAppID:
		m.Config.XunfeiMTAppID = value
	case keyXunfeiMTAPIKey:
		m.Config.XunfeiMTAPIKey = value
	case keyXunfeiMTAPISecret:
		m.Config.XunfeiMTAPISecret = value
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
	case keySimliKey:
		m.Config.SimliKey = value
	case keySimliFaceID:
		m.Config.SimliFaceID = value
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
		TranslationProvider:   m.Config.TranslationProvider,
		LLMProvider:           m.Config.LLMProvider,
		TTSProvider:           m.Config.TTSProvider,
		LipSyncProvider:       m.Config.LipSyncProvider,
		EmbeddingProvider:     m.Config.EmbeddingProvider,
		HearingSourceLang:     m.Config.HearingSourceLang,
		HearingTargetLang:     m.Config.HearingTargetLang,
		SpeakingInputLang:     m.Config.SpeakingInputLang,
		SpeakingOutputLang:    m.Config.SpeakingOutputLang,
		VirtualMicName:        m.Config.VirtualMicName,
		PhysicalMicName:       m.Config.PhysicalMicName,
		PhysicalCamName:       m.Config.PhysicalCamName,
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

func floatOr(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

func translationProviderOr(v TranslationProviderType, def TranslationProviderType) TranslationProviderType {
	switch v {
	case TranslationProviderXunfei, TranslationProviderNull:
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

func lipSyncProviderOr(v LipSyncProviderType, def LipSyncProviderType) LipSyncProviderType {
	switch v {
	case LipSyncProviderSimli, LipSyncProviderStealth, LipSyncProviderNull:
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
