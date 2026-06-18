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
	if m.Config.TTSProvider != config.TTSProviderSystem {
		t.Errorf("TTSProvider: want %q, got %q", config.TTSProviderSystem, m.Config.TTSProvider)
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
		SetupCompleted:    true,
		HearingSourceLang: "ja",
		HearingTargetLang: "zh",
		GhostFontSize:     20,
		GhostOpacity:      0.5,
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

	// 验证文件确实写入
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Errorf("config.json should exist: %v", err)
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
