package config

// keyring key 常量 —— 严禁在代码中硬编码字符串
const (
	keyXunfeiAppID       = "xunfei_app_id"
	keyXunfeiAPIKey      = "xunfei_api_key"
	keyXunfeiAPISecret   = "xunfei_api_secret"
	keyDeepSeekKey       = "deepseek_key"
	keyElevenLabsKey     = "elevenlabs_key"
	keyElevenLabsVoiceID = "elevenlabs_voice_id"
	keySimliKey          = "simli_key"
	keySimliFaceID       = "simli_face_id"
)

// 默认值常量
const (
	DefaultGhostFontSize      = 16
	DefaultGhostOpacity       = 0.85
	DefaultGhostPosition      = "bottom-right"
	DefaultDeepSeekModel      = "deepseek-chat"
	DefaultHearingSourceLang  = "en"
	DefaultHearingTargetLang  = "zh"
	DefaultSpeakingInputLang  = "zh"
	DefaultSpeakingOutputLang = "en"
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
	XunfeiAppID       string
	XunfeiAPIKey      string
	XunfeiAPISecret   string
	DeepSeekKey       string
	DeepSeekModel     string
	ElevenLabsKey     string
	ElevenLabsVoiceID string
	SimliKey          string
	SimliFaceID       string

	// 语言设置
	HearingSourceLang  string
	HearingTargetLang  string
	SpeakingInputLang  string
	SpeakingOutputLang string

	// 设备绑定
	VirtualMicName  string
	PhysicalMicName string
	PhysicalCamName string
	VirtualCamName  string

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
	m.Config.XunfeiAppID, _ = m.store.Get(keyXunfeiAppID)
	m.Config.XunfeiAPIKey, _ = m.store.Get(keyXunfeiAPIKey)
	m.Config.XunfeiAPISecret, _ = m.store.Get(keyXunfeiAPISecret)
	m.Config.DeepSeekKey, _ = m.store.Get(keyDeepSeekKey)
	m.Config.ElevenLabsKey, _ = m.store.Get(keyElevenLabsKey)
	m.Config.ElevenLabsVoiceID, _ = m.store.Get(keyElevenLabsVoiceID)
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
	m.Config.HearingSourceLang = stringOr(lc.HearingSourceLang, DefaultHearingSourceLang)
	m.Config.HearingTargetLang = stringOr(lc.HearingTargetLang, DefaultHearingTargetLang)
	m.Config.SpeakingInputLang = stringOr(lc.SpeakingInputLang, DefaultSpeakingInputLang)
	m.Config.SpeakingOutputLang = stringOr(lc.SpeakingOutputLang, DefaultSpeakingOutputLang)
	m.Config.VirtualMicName = lc.VirtualMicName
	m.Config.PhysicalMicName = lc.PhysicalMicName
	m.Config.PhysicalCamName = lc.PhysicalCamName
	m.Config.VirtualCamName = lc.VirtualCamName
	m.Config.GhostFontSize = intOr(lc.GhostFontSize, DefaultGhostFontSize)
	m.Config.GhostOpacity = floatOr(lc.GhostOpacity, DefaultGhostOpacity)
	m.Config.GhostPosition = stringOr(lc.GhostPosition, DefaultGhostPosition)
	m.Config.RAGPrompt = stringOr(lc.RAGPrompt, DefaultRAGPrompt)
	m.Config.SpeakPolishPrompt = stringOr(lc.SpeakPolishPrompt, DefaultSpeakPolishPrompt)
	m.Config.PolishEnabled = lc.PolishEnabled
}

// SaveAPIKey 将单个 API Key 写入 Keychain 并同步内存配置。
// service 为服务名（xunfei/deepseek/elevenlabs/simli），field 为字段名。
func (m *Manager) SaveAPIKey(service, field, value string) error {
	key := service + "_" + field
	if err := m.store.Set(key, value); err != nil {
		return err
	}
	// 同步到内存
	switch key {
	case keyXunfeiAppID:
		m.Config.XunfeiAppID = value
	case keyXunfeiAPIKey:
		m.Config.XunfeiAPIKey = value
	case keyXunfeiAPISecret:
		m.Config.XunfeiAPISecret = value
	case keyDeepSeekKey:
		m.Config.DeepSeekKey = value
	case keyElevenLabsKey:
		m.Config.ElevenLabsKey = value
	case keyElevenLabsVoiceID:
		m.Config.ElevenLabsVoiceID = value
	case keySimliKey:
		m.Config.SimliKey = value
	case keySimliFaceID:
		m.Config.SimliFaceID = value
	}
	return nil
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
		SetupCompleted:     m.Config.SetupCompleted,
		ActiveResumeID:     m.Config.ActiveResumeID,
		UILocale:           m.Config.UILocale,
		DeepSeekModel:      m.Config.DeepSeekModel,
		HearingSourceLang:  m.Config.HearingSourceLang,
		HearingTargetLang:  m.Config.HearingTargetLang,
		SpeakingInputLang:  m.Config.SpeakingInputLang,
		SpeakingOutputLang: m.Config.SpeakingOutputLang,
		VirtualMicName:     m.Config.VirtualMicName,
		PhysicalMicName:    m.Config.PhysicalMicName,
		PhysicalCamName:    m.Config.PhysicalCamName,
		VirtualCamName:     m.Config.VirtualCamName,
		GhostFontSize:      m.Config.GhostFontSize,
		GhostOpacity:       m.Config.GhostOpacity,
		GhostPosition:      m.Config.GhostPosition,
		RAGPrompt:          m.Config.RAGPrompt,
		SpeakPolishPrompt:  m.Config.SpeakPolishPrompt,
		PolishEnabled:      m.Config.PolishEnabled,
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
