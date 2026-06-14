package config

// FrontendConfig 是暴露给前端的配置视图，不含敏感字段原值。
// API Key 以掩码形式返回（仅用于显示是否已设置）。
type FrontendConfig struct {
	// API Key 是否已设置（true = 已设置，不返回原值）
	XunfeiAppIDSet       bool `json:"xunfei_app_id_set"`
	XunfeiAPIKeySet      bool `json:"xunfei_api_key_set"`
	XunfeiAPISecretSet   bool `json:"xunfei_api_secret_set"`
	DeepSeekKeySet       bool `json:"deepseek_key_set"`
	ElevenLabsKeySet     bool `json:"elevenlabs_key_set"`
	ElevenLabsVoiceIDSet bool `json:"elevenlabs_voice_id_set"`
	SimliKeySet          bool `json:"simli_key_set"`
	SimliFaceIDSet       bool `json:"simli_face_id_set"`

	// 非敏感配置（明文）
	UILocale           string  `json:"ui_locale"` // "zh-CN" | "en-US"
	DeepSeekModel      string  `json:"deepseek_model"`
	HearingSourceLang  string  `json:"hearing_source_lang"`
	HearingTargetLang  string  `json:"hearing_target_lang"`
	SpeakingInputLang  string  `json:"speaking_input_lang"`
	SpeakingOutputLang string  `json:"speaking_output_lang"`
	VirtualMicName     string  `json:"virtual_mic_name"`
	PhysicalMicName    string  `json:"physical_mic_name"`
	PhysicalCamName    string  `json:"physical_cam_name"`
	VirtualCamName     string  `json:"virtual_cam_name"`
	GhostFontSize      int     `json:"ghost_font_size"`
	GhostOpacity       float64 `json:"ghost_opacity"`
	GhostPosition      string  `json:"ghost_position"`
	RAGPrompt          string  `json:"rag_prompt"`
	SpeakPolishPrompt  string  `json:"speak_polish_prompt"`
	PolishEnabled      bool    `json:"polish_enabled"`
	SetupCompleted     bool    `json:"setup_completed"`
	ActiveResumeID     string  `json:"active_resume_id"`
}

// SaveAPIKeyRequest 前端传入的 API Key 写入请求。
// service 取值：xunfei / deepseek / elevenlabs / simli
// field  取值：app_id / api_key / api_secret / key / voice_id / face_id 等
type SaveAPIKeyRequest struct {
	Service string `json:"service"`
	Field   string `json:"field"`
	Value   string `json:"value"`
}

// SaveLocalConfigRequest 前端传入的本地配置写入请求（不含 API Key）。
type SaveLocalConfigRequest struct {
	UILocale           string  `json:"ui_locale"`
	DeepSeekModel      string  `json:"deepseek_model"`
	HearingSourceLang  string  `json:"hearing_source_lang"`
	HearingTargetLang  string  `json:"hearing_target_lang"`
	SpeakingInputLang  string  `json:"speaking_input_lang"`
	SpeakingOutputLang string  `json:"speaking_output_lang"`
	VirtualMicName     string  `json:"virtual_mic_name"`
	PhysicalMicName    string  `json:"physical_mic_name"`
	PhysicalCamName    string  `json:"physical_cam_name"`
	VirtualCamName     string  `json:"virtual_cam_name"`
	GhostFontSize      int     `json:"ghost_font_size"`
	GhostOpacity       float64 `json:"ghost_opacity"`
	GhostPosition      string  `json:"ghost_position"`
	RAGPrompt          string  `json:"rag_prompt"`
	SpeakPolishPrompt  string  `json:"speak_polish_prompt"`
	PolishEnabled      bool    `json:"polish_enabled"`
	SetupCompleted     bool    `json:"setup_completed"`
}

// DefaultPromptsResponse 返回 Go 后端硬编码的默认 Prompt 值。
type DefaultPromptsResponse struct {
	RAGPrompt         string `json:"rag_prompt"`
	SpeakPolishPrompt string `json:"speak_polish_prompt"`
}
