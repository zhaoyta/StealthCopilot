package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zhaoyta/stealthcopilot/internal/config"
)

// TestManager_DefaultValues 验证 Manager 对空配置正确应用默认值。
func TestManager_DefaultValues(t *testing.T) {
	dir := t.TempDir()
	m, err := config.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if m.Config.GhostFontSize != config.DefaultGhostFontSize {
		t.Errorf("GhostFontSize: want %d, got %d", config.DefaultGhostFontSize, m.Config.GhostFontSize)
	}
	if m.Config.GhostOpacity != config.DefaultGhostOpacity {
		t.Errorf("GhostOpacity: want %f, got %f", config.DefaultGhostOpacity, m.Config.GhostOpacity)
	}
	if m.Config.DeepSeekModel != config.DefaultDeepSeekModel {
		t.Errorf("DeepSeekModel: want %q, got %q", config.DefaultDeepSeekModel, m.Config.DeepSeekModel)
	}
	if m.Config.RAGPrompt == "" {
		t.Error("RAGPrompt should not be empty")
	}
	if m.Config.SpeakPolishPrompt == "" {
		t.Error("SpeakPolishPrompt should not be empty")
	}
	if m.Config.HearingASRProvider != config.TranslationProviderXunfeiSimult {
		t.Errorf("HearingASRProvider: want %q, got %q", config.TranslationProviderXunfeiSimult, m.Config.HearingASRProvider)
	}
	if m.Config.HearingTransProvider != config.TranslationProviderXunfeiSimult {
		t.Errorf("HearingTransProvider: want %q, got %q", config.TranslationProviderXunfeiSimult, m.Config.HearingTransProvider)
	}
	if m.Config.HearingTTSProvider != config.TTSProviderSystem {
		t.Errorf("HearingTTSProvider: want %q, got %q", config.TTSProviderSystem, m.Config.HearingTTSProvider)
	}
	if m.Config.SpeakingASRProvider != config.TranslationProviderXunfeiSimult {
		t.Errorf("SpeakingASRProvider: want %q, got %q", config.TranslationProviderXunfeiSimult, m.Config.SpeakingASRProvider)
	}
	if m.Config.SpeakingTransProvider != config.TranslationProviderXunfeiSimult {
		t.Errorf("SpeakingTransProvider: want %q, got %q", config.TranslationProviderXunfeiSimult, m.Config.SpeakingTransProvider)
	}
	if m.Config.SpeakingTTSProvider != config.TTSProviderSystem {
		t.Errorf("SpeakingTTSProvider: want %q, got %q", config.TTSProviderSystem, m.Config.SpeakingTTSProvider)
	}
}

// TestManager_SaveLocalConfig 验证 SaveLocalConfig 持久化并同步内存。
func TestManager_SaveLocalConfig(t *testing.T) {
	dir := t.TempDir()
	m, err := config.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	lc := config.LocalConfig{
		SetupCompleted:        true,
		HearingASRProvider:    config.TranslationProviderNull,
		HearingTransProvider:  config.TranslationProviderXunfeiSimult,
		HearingTTSProvider:    config.TTSProviderNull,
		SpeakingASRProvider:   config.TranslationProviderXunfeiSimult,
		SpeakingTransProvider: config.TranslationProviderNull,
		SpeakingTTSProvider:   config.TTSProviderXunfeiVoiceClone,
		HearingSourceLang:     "ja",
		HearingTargetLang:     "zh",
		DigitalHumanEnabled:   true,
		ZegoDigitalHumanID:    "dh-1",
		ZegoRoomID:            "room-1",
		ZegoStreamID:          "stream-1",
		GhostFontSize:         20,
		GhostOpacity:          0.5,
	}
	if err := m.SaveLocalConfig(lc); err != nil {
		t.Fatalf("SaveLocalConfig: %v", err)
	}

	if !m.Config.SetupCompleted {
		t.Error("SetupCompleted should be true")
	}
	if m.Config.HearingSourceLang != "ja" {
		t.Errorf("HearingSourceLang: want 'ja', got %q", m.Config.HearingSourceLang)
	}
	if m.Config.GhostFontSize != 20 {
		t.Errorf("GhostFontSize: want 20, got %d", m.Config.GhostFontSize)
	}
	if m.Config.HearingASRProvider != config.TranslationProviderNull {
		t.Errorf("HearingASRProvider: want %q, got %q", config.TranslationProviderNull, m.Config.HearingASRProvider)
	}
	if m.Config.SpeakingTransProvider != config.TranslationProviderNull {
		t.Errorf("SpeakingTransProvider: want %q, got %q", config.TranslationProviderNull, m.Config.SpeakingTransProvider)
	}
	if m.Config.HearingTTSProvider != config.TTSProviderNull {
		t.Errorf("HearingTTSProvider: want %q, got %q", config.TTSProviderNull, m.Config.HearingTTSProvider)
	}
	if m.Config.SpeakingTTSProvider != config.TTSProviderXunfeiVoiceClone {
		t.Errorf("SpeakingTTSProvider: want %q, got %q", config.TTSProviderXunfeiVoiceClone, m.Config.SpeakingTTSProvider)
	}
	if !m.Config.DigitalHumanEnabled {
		t.Error("DigitalHumanEnabled should be true")
	}
	if m.Config.ZegoDigitalHumanID != "dh-1" || m.Config.ZegoRoomID != "room-1" || m.Config.ZegoStreamID != "stream-1" {
		t.Errorf("ZEGO local config mismatch: %+v", m.Config)
	}

	// 验证文件确实写入
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Errorf("config.json should exist: %v", err)
	}
}

func TestManager_SaveAPIKeyZegoDigitalHuman(t *testing.T) {
	dir := t.TempDir()
	m, err := config.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := m.SaveAPIKey("zego_digital_human", "app_id", "123456"); err != nil {
		t.Fatalf("SaveAPIKey app_id: %v", err)
	}
	if err := m.SaveAPIKey("zego_digital_human", "server_secret", "secret"); err != nil {
		t.Fatalf("SaveAPIKey server_secret: %v", err)
	}
	if m.Config.ZegoDigitalHumanAppID != "123456" {
		t.Fatalf("ZegoDigitalHumanAppID = %q", m.Config.ZegoDigitalHumanAppID)
	}
	if m.Config.ZegoServerSecret != "secret" {
		t.Fatalf("ZegoServerSecret = %q", m.Config.ZegoServerSecret)
	}
}

func TestAppConfig_ValidateDigitalHumanOutput(t *testing.T) {
	// 默认 provider 为 Simli，空配置应缺少 API Key + Face ID
	cfg := &config.AppConfig{}
	result := cfg.ValidateDigitalHumanOutput()
	if result.OK() {
		t.Fatal("empty config should not validate")
	}
	if len(result.MissingCredentials) != 1 {
		t.Fatalf("simli: expected 1 missing credential (API Key), got %v", result.MissingCredentials)
	}

	// Simli 完整配置应通过校验
	cfg.SimliAPIKey = "sk-test"
	cfg.SimaliFaceID = "face-id"
	cfg.VirtualMicName = "BlackHole"
	cfg.VirtualCamName = "OBS Virtual Camera"
	if result := cfg.ValidateDigitalHumanOutput(); !result.OK() {
		t.Fatalf("complete simli config should validate: %+v", result)
	}

	// ZEGO provider 校验
	zegoCfg := &config.AppConfig{
		DigitalHumanProvider: config.DigitalHumanProviderZego,
	}
	zegoResult := zegoCfg.ValidateDigitalHumanOutput()
	if zegoResult.OK() {
		t.Fatal("empty zego config should not validate")
	}
	if len(zegoResult.MissingCredentials) != 2 {
		t.Fatalf("zego: expected 2 missing credentials, got %v", zegoResult.MissingCredentials)
	}
	zegoCfg.ZegoDigitalHumanAppID = "123"
	zegoCfg.ZegoServerSecret = "secret"
	zegoCfg.ZegoDigitalHumanID = "dh"
	zegoCfg.VirtualMicName = "BlackHole"
	zegoCfg.VirtualCamName = "OBS Virtual Camera"
	if result := zegoCfg.ValidateDigitalHumanOutput(); !result.OK() {
		t.Fatalf("complete zego config should validate: %+v", result)
	}
}

// TestManager_SaveAPIKeyRejectsVoiceCloneState 验证训练状态 ID 不能从通用密钥入口写入。
func TestManager_SaveAPIKeyRejectsVoiceCloneState(t *testing.T) {
	dir := t.TempDir()
	m, err := config.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := m.SaveAPIKey("xunfei_tts", "asset_id", "asset"); err == nil {
		t.Fatal("SaveAPIKey should reject asset_id")
	}
	if err := m.SaveAPIKey("xunfei_tts", "task_id", "task"); err == nil {
		t.Fatal("SaveAPIKey should reject task_id")
	}
}

// TestManager_ToLocalConfig 验证 ToLocalConfig 与内存状态一致。
func TestManager_ToLocalConfig(t *testing.T) {
	dir := t.TempDir()
	m, err := config.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_ = m.SaveLocalConfig(config.LocalConfig{SetupCompleted: true, GhostFontSize: 24})
	lc := m.ToLocalConfig()

	if !lc.SetupCompleted {
		t.Error("SetupCompleted mismatch")
	}
	if lc.GhostFontSize != 24 {
		t.Errorf("GhostFontSize: want 24, got %d", lc.GhostFontSize)
	}
}
