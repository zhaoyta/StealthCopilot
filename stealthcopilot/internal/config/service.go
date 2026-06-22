package config

import (
	"context"
	"fmt"
)

// Service 是暴露给 Wails 前端的配置服务，所有公开方法均可在 JS 侧调用。
type Service struct {
	manager *Manager
}

// NewService 创建 ConfigService 并完成启动时预加载。
// dataDir 是应用数据目录，同 Manager.
func NewService(dataDir string) (*Service, error) {
	m, err := NewManager(dataDir)
	if err != nil {
		return nil, fmt.Errorf("config.NewService: %w", err)
	}
	return &Service{manager: m}, nil
}

// Startup 在 Wails OnStartup 时调用，持有 context 以备后用。
func (s *Service) Startup(_ context.Context) {}

// GetConfig 返回当前配置的前端视图（API Key 以 bool 表示是否已设置）。
func (s *Service) GetConfig() FrontendConfig {
	c := s.manager.Config
	return FrontendConfig{
		XunfeiSimultAppIDSet:     c.XunfeiSimultAppID != "",
		XunfeiSimultAPIKeySet:    c.XunfeiSimultAPIKey != "",
		XunfeiSimultAPISecretSet: c.XunfeiSimultAPISecret != "",
		XunfeiTTSAppIDSet:        c.XunfeiTTSAppID != "",
		XunfeiTTSAPIKeySet:       c.XunfeiTTSAPIKey != "",
		XunfeiTTSAPISecretSet:    c.XunfeiTTSAPISecret != "",
		XunfeiTTSAssetIDSet:      c.XunfeiTTSAssetID != "",
		XunfeiTTSTaskIDSet:       c.XunfeiTTSTaskID != "",
		DeepSeekKeySet:           c.DeepSeekKey != "",
		ZegoAppIDSet:             c.ZegoDigitalHumanAppID != "",
		ZegoServerSecretSet:      c.ZegoServerSecret != "",
		SimliAPIKeySet:           c.SimliAPIKey != "",
		UILocale:                 c.UILocale,
		DeepSeekModel:            c.DeepSeekModel,
		LLMBaseURL:               c.LLMBaseURL,
		HearingASRProvider:       string(c.HearingASRProvider),
		HearingTransProvider:     string(c.HearingTransProvider),
		HearingTTSProvider:       string(c.HearingTTSProvider),
		SpeakingASRProvider:      string(c.SpeakingASRProvider),
		SpeakingTransProvider:    string(c.SpeakingTransProvider),
		SpeakingTTSProvider:      string(c.SpeakingTTSProvider),
		LLMProvider:              string(c.LLMProvider),
		EmbeddingProvider:        string(c.EmbeddingProvider),
		DigitalHumanEnabled:      c.DigitalHumanEnabled,
		DigitalHumanProvider:     string(c.DigitalHumanProvider),
		SimaliFaceID:             c.SimaliFaceID,
		ZegoDigitalHumanID:       c.ZegoDigitalHumanID,
		ZegoRoomID:               c.ZegoRoomID,
		ZegoStreamID:             c.ZegoStreamID,
		ZegoRTMPPullURL:          c.ZegoRTMPPullURL,
		HearingSourceLang:        c.HearingSourceLang,
		HearingTargetLang:        c.HearingTargetLang,
		SpeakingInputLang:        c.SpeakingInputLang,
		SpeakingOutputLang:       c.SpeakingOutputLang,
		VirtualMicName:           c.VirtualMicName,
		PhysicalMicName:          c.PhysicalMicName,
		VirtualCamName:           c.VirtualCamName,
		MonitorOutputName:        c.MonitorOutputName,
		HearingMonitorEnabled:    c.HearingMonitorEnabled,
		HearingMonitorVolume:     c.HearingMonitorVolume,
		HearingMonitorRate:       c.HearingMonitorRate,
		GhostFontSize:            c.GhostFontSize,
		GhostOpacity:             c.GhostOpacity,
		GhostPosition:            c.GhostPosition,
		RAGPrompt:                c.RAGPrompt,
		SpeakPolishPrompt:        c.SpeakPolishPrompt,
		PolishEnabled:            c.PolishEnabled,
		HistoryMaxTurns:          c.HistoryMaxTurns,
		SetupCompleted:           c.SetupCompleted,
		ActiveResumeID:           c.ActiveResumeID,
	}
}

// SaveAPIKey 将单个 API Key 写入系统 Keychain。
// 返回 error 字符串，空字符串表示成功（JS 端友好格式）。
func (s *Service) SaveAPIKey(req SaveAPIKeyRequest) string {
	if err := s.manager.SaveAPIKey(req.Service, req.Field, req.Value); err != nil {
		return err.Error()
	}
	return ""
}

// SaveLocalConfig 将非敏感配置写入本地文件并同步内存。
func (s *Service) SaveLocalConfig(req SaveLocalConfigRequest) string {
	lc := LocalConfig{
		UILocale:              req.UILocale,
		DeepSeekModel:         req.DeepSeekModel,
		LLMBaseURL:            req.LLMBaseURL,
		HearingASRProvider:    TranslationProviderType(req.HearingASRProvider),
		HearingTransProvider:  TranslationProviderType(req.HearingTransProvider),
		HearingTTSProvider:    TTSProviderType(req.HearingTTSProvider),
		SpeakingASRProvider:   TranslationProviderType(req.SpeakingASRProvider),
		SpeakingTransProvider: TranslationProviderType(req.SpeakingTransProvider),
		SpeakingTTSProvider:   TTSProviderType(req.SpeakingTTSProvider),
		LLMProvider:           LLMProviderType(req.LLMProvider),
		EmbeddingProvider:     EmbeddingProviderType(req.EmbeddingProvider),
		DigitalHumanEnabled:   req.DigitalHumanEnabled,
		DigitalHumanProvider:  DigitalHumanProviderType(req.DigitalHumanProvider),
		SimaliFaceID:          req.SimaliFaceID,
		ZegoDigitalHumanID:    req.ZegoDigitalHumanID,
		ZegoRoomID:            req.ZegoRoomID,
		ZegoStreamID:          req.ZegoStreamID,
		ZegoRTMPPullURL:       req.ZegoRTMPPullURL,
		HearingSourceLang:     req.HearingSourceLang,
		HearingTargetLang:     req.HearingTargetLang,
		SpeakingInputLang:     req.SpeakingInputLang,
		SpeakingOutputLang:    req.SpeakingOutputLang,
		VirtualMicName:        req.VirtualMicName,
		PhysicalMicName:       req.PhysicalMicName,
		VirtualCamName:        req.VirtualCamName,
		MonitorOutputName:     req.MonitorOutputName,
		HearingMonitorEnabled: req.HearingMonitorEnabled,
		HearingMonitorVolume:  req.HearingMonitorVolume,
		HearingMonitorRate:    req.HearingMonitorRate,
		GhostFontSize:         req.GhostFontSize,
		GhostOpacity:          req.GhostOpacity,
		GhostPosition:         req.GhostPosition,
		RAGPrompt:             req.RAGPrompt,
		SpeakPolishPrompt:     req.SpeakPolishPrompt,
		PolishEnabled:         req.PolishEnabled,
		HistoryMaxTurns:       req.HistoryMaxTurns,
		SetupCompleted:        req.SetupCompleted,
		ActiveResumeID:        s.manager.Config.ActiveResumeID, // 简历激活由 ResumeService 管理
	}
	if err := s.manager.SaveLocalConfig(lc); err != nil {
		return err.Error()
	}
	return ""
}

// MarkSetupComplete 标记初始化向导已完成，后续启动将直接进入主界面。
func (s *Service) MarkSetupComplete() string {
	lc := s.manager.ToLocalConfig()
	lc.SetupCompleted = true
	if err := s.manager.SaveLocalConfig(lc); err != nil {
		return err.Error()
	}
	return ""
}

// GetDefaultPrompts 返回 Go 后端硬编码的默认 Prompt，供前端"恢复默认"按钮使用。
func (s *Service) GetDefaultPrompts() DefaultPromptsResponse {
	return DefaultPromptsResponse{
		RAGPrompt:         DefaultRAGPrompt,
		SpeakPolishPrompt: DefaultSpeakPolishPrompt,
	}
}

// InternalManager 供其他 Go 包访问底层 Manager（不暴露给前端）。
func (s *Service) InternalManager() *Manager {
	return s.manager
}
