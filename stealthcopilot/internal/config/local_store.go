package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const localConfigFileName = "config.json"

// LocalConfig 存储非敏感、可落盘的配置项（API Key 等敏感值不在此）。
type LocalConfig struct {
	SetupCompleted        bool                    `json:"setup_completed"`
	ActiveResumeID        string                  `json:"active_resume_id"`
	UILocale              string                  `json:"ui_locale"` // "zh-CN" | "en-US"
	DeepSeekModel         string                  `json:"deepseek_model"`
	LLMBaseURL            string                  `json:"llm_base_url"`
	HearingASRProvider    TranslationProviderType `json:"hearing_asr_provider"`
	HearingTransProvider  TranslationProviderType `json:"hearing_trans_provider"`
	HearingTTSProvider    TTSProviderType         `json:"hearing_tts_provider"`
	SpeakingASRProvider   TranslationProviderType `json:"speaking_asr_provider"`
	SpeakingTransProvider TranslationProviderType `json:"speaking_trans_provider"`
	SpeakingTTSProvider   TTSProviderType         `json:"speaking_tts_provider"`
	LLMProvider           LLMProviderType         `json:"llm_provider"`
	LipSyncProvider       LipSyncProviderType     `json:"lipsync_provider"`
	EmbeddingProvider     EmbeddingProviderType   `json:"embedding_provider"`
	HearingSourceLang     string                  `json:"hearing_source_lang"`
	HearingTargetLang     string                  `json:"hearing_target_lang"`
	SpeakingInputLang     string                  `json:"speaking_input_lang"`
	SpeakingOutputLang    string                  `json:"speaking_output_lang"`
	VirtualMicName        string                  `json:"virtual_mic_name"`
	PhysicalMicName       string                  `json:"physical_mic_name"`
	PhysicalCamName       string                  `json:"physical_cam_name"`
	VirtualCamName        string                  `json:"virtual_cam_name"`
	MonitorOutputName     string                  `json:"monitor_output_name"`
	HearingMonitorEnabled bool                    `json:"hearing_monitor_enabled"`
	HearingMonitorVolume  int                     `json:"hearing_monitor_volume"`
	HearingMonitorRate    int                     `json:"hearing_monitor_rate"`
	GhostFontSize         int                     `json:"ghost_font_size"`
	GhostOpacity          float64                 `json:"ghost_opacity"`
	GhostPosition         string                  `json:"ghost_position"`
	RAGPrompt             string                  `json:"rag_prompt"`
	SpeakPolishPrompt     string                  `json:"speak_polish_prompt"`
	PolishEnabled         bool                    `json:"polish_enabled"`
}

// localStore 管理本地 JSON 配置文件的读写。
type localStore struct {
	path string
}

// newLocalStore 初始化 localStore，dataDir 不存在时自动创建。
func newLocalStore(dataDir string) (*localStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	return &localStore{path: filepath.Join(dataDir, localConfigFileName)}, nil
}

// load 从磁盘读取配置；文件不存在时返回零值 LocalConfig（无错误）。
func (ls *localStore) load() LocalConfig {
	data, err := os.ReadFile(ls.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LocalConfig{}
		}
		return LocalConfig{}
	}
	var cfg LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return LocalConfig{}
	}
	return cfg
}

// save 将配置序列化并写入磁盘（原子替换）。
func (ls *localStore) save(cfg LocalConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := ls.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ls.path)
}
