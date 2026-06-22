package config

// FrontendConfig 是暴露给前端的配置视图，不含敏感字段原值。
// API Key 以掩码形式返回（仅用于显示是否已设置）。
type FrontendConfig struct {
	// API Key 是否已设置（true = 已设置，不返回原值）
	XunfeiSimultAppIDSet     bool `json:"xunfei_simult_app_id_set"`
	XunfeiSimultAPIKeySet    bool `json:"xunfei_simult_api_key_set"`
	XunfeiSimultAPISecretSet bool `json:"xunfei_simult_api_secret_set"`
	XunfeiTTSAppIDSet        bool `json:"xunfei_tts_app_id_set"`
	XunfeiTTSAPIKeySet       bool `json:"xunfei_tts_api_key_set"`
	XunfeiTTSAPISecretSet    bool `json:"xunfei_tts_api_secret_set"`
	XunfeiTTSAssetIDSet      bool `json:"xunfei_tts_asset_id_set"`
	XunfeiTTSTaskIDSet       bool `json:"xunfei_tts_task_id_set"`
	DeepSeekKeySet           bool `json:"deepseek_key_set"`
	ZegoAppIDSet             bool `json:"zego_app_id_set"`
	ZegoServerSecretSet      bool `json:"zego_server_secret_set"`
	SimliAPIKeySet           bool `json:"simli_api_key_set"`

	// 非敏感配置（明文）
	UILocale              string  `json:"ui_locale"` // "zh-CN" | "en-US"
	DeepSeekModel         string  `json:"deepseek_model"`
	LLMBaseURL            string  `json:"llm_base_url"`
	HearingASRProvider    string  `json:"hearing_asr_provider"`
	HearingTransProvider  string  `json:"hearing_trans_provider"`
	HearingTTSProvider    string  `json:"hearing_tts_provider"`
	SpeakingASRProvider   string  `json:"speaking_asr_provider"`
	SpeakingTransProvider string  `json:"speaking_trans_provider"`
	SpeakingTTSProvider   string  `json:"speaking_tts_provider"`
	LLMProvider           string  `json:"llm_provider"`
	EmbeddingProvider     string  `json:"embedding_provider"`
	DigitalHumanEnabled   bool    `json:"digital_human_enabled"`
	DigitalHumanProvider  string  `json:"digital_human_provider"`
	SimaliFaceID          string  `json:"simli_face_id"`
	ZegoDigitalHumanID    string  `json:"zego_digital_human_id"`
	ZegoRoomID            string  `json:"zego_room_id"`
	ZegoStreamID          string  `json:"zego_stream_id"`
	ZegoRTMPPullURL       string  `json:"zego_rtmp_pull_url"`
	HearingSourceLang     string  `json:"hearing_source_lang"`
	HearingTargetLang     string  `json:"hearing_target_lang"`
	SpeakingInputLang     string  `json:"speaking_input_lang"`
	SpeakingOutputLang    string  `json:"speaking_output_lang"`
	VirtualMicName        string  `json:"virtual_mic_name"`
	PhysicalMicName       string  `json:"physical_mic_name"`
	VirtualCamName        string  `json:"virtual_cam_name"`
	MonitorOutputName     string  `json:"monitor_output_name"`
	HearingMonitorEnabled bool    `json:"hearing_monitor_enabled"`
	HearingMonitorVolume  int     `json:"hearing_monitor_volume"`
	HearingMonitorRate    int     `json:"hearing_monitor_rate"`
	GhostFontSize         int     `json:"ghost_font_size"`
	GhostOpacity          float64 `json:"ghost_opacity"`
	GhostPosition         string  `json:"ghost_position"`
	RAGPrompt             string  `json:"rag_prompt"`
	SpeakPolishPrompt     string  `json:"speak_polish_prompt"`
	PolishEnabled         bool    `json:"polish_enabled"`
	HistoryMaxTurns       int     `json:"history_max_turns"`
	SetupCompleted        bool    `json:"setup_completed"`
	ActiveResumeID        string  `json:"active_resume_id"`
}

// SaveAPIKeyRequest 前端传入的 API Key 写入请求。
// service 取值：xunfei_simult / xunfei_tts / deepseek / zego_digital_human
// field  取值：app_id / api_key / api_secret / key / server_secret 等。
// 声音复刻 task_id / asset_id 只能由训练流程内部写入，不接受前端手填。
type SaveAPIKeyRequest struct {
	Service string `json:"service"`
	Field   string `json:"field"`
	Value   string `json:"value"`
}

// SaveLocalConfigRequest 前端传入的本地配置写入请求（不含 API Key）。
type SaveLocalConfigRequest struct {
	UILocale              string  `json:"ui_locale"`
	DeepSeekModel         string  `json:"deepseek_model"`
	LLMBaseURL            string  `json:"llm_base_url"`
	HearingASRProvider    string  `json:"hearing_asr_provider"`
	HearingTransProvider  string  `json:"hearing_trans_provider"`
	HearingTTSProvider    string  `json:"hearing_tts_provider"`
	SpeakingASRProvider   string  `json:"speaking_asr_provider"`
	SpeakingTransProvider string  `json:"speaking_trans_provider"`
	SpeakingTTSProvider   string  `json:"speaking_tts_provider"`
	LLMProvider           string  `json:"llm_provider"`
	EmbeddingProvider     string  `json:"embedding_provider"`
	DigitalHumanEnabled   bool    `json:"digital_human_enabled"`
	DigitalHumanProvider  string  `json:"digital_human_provider"`
	SimaliFaceID          string  `json:"simli_face_id"`
	ZegoDigitalHumanID    string  `json:"zego_digital_human_id"`
	ZegoRoomID            string  `json:"zego_room_id"`
	ZegoStreamID          string  `json:"zego_stream_id"`
	ZegoRTMPPullURL       string  `json:"zego_rtmp_pull_url"`
	HearingSourceLang     string  `json:"hearing_source_lang"`
	HearingTargetLang     string  `json:"hearing_target_lang"`
	SpeakingInputLang     string  `json:"speaking_input_lang"`
	SpeakingOutputLang    string  `json:"speaking_output_lang"`
	VirtualMicName        string  `json:"virtual_mic_name"`
	PhysicalMicName       string  `json:"physical_mic_name"`
	VirtualCamName        string  `json:"virtual_cam_name"`
	MonitorOutputName     string  `json:"monitor_output_name"`
	HearingMonitorEnabled bool    `json:"hearing_monitor_enabled"`
	HearingMonitorVolume  int     `json:"hearing_monitor_volume"`
	HearingMonitorRate    int     `json:"hearing_monitor_rate"`
	GhostFontSize         int     `json:"ghost_font_size"`
	GhostOpacity          float64 `json:"ghost_opacity"`
	GhostPosition         string  `json:"ghost_position"`
	RAGPrompt             string  `json:"rag_prompt"`
	SpeakPolishPrompt     string  `json:"speak_polish_prompt"`
	PolishEnabled         bool    `json:"polish_enabled"`
	HistoryMaxTurns       int     `json:"history_max_turns"`
	SetupCompleted        bool    `json:"setup_completed"`
}

// DefaultPromptsResponse 返回 Go 后端硬编码的默认 Prompt 值。
type DefaultPromptsResponse struct {
	RAGPrompt         string `json:"rag_prompt"`
	SpeakPolishPrompt string `json:"speak_polish_prompt"`
}
