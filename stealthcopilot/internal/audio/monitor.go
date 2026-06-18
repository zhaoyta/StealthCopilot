package audio

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const (
	DefaultMonitorRate   = 0
	DefaultMonitorVolume = 80
)

// MonitorConfig controls the private translated-audio monitor used by the
// hearing chain. On macOS OutputDevice may be a numeric AudioToolbox output
// index; otherwise system speech backends play through the OS default output.
type MonitorConfig struct {
	Enabled      bool
	OutputDevice string
	Rate         int
	Volume       int
}

// MonitorSink speaks translated text to the interviewer's private monitor path.
type MonitorSink interface {
	Speak(ctx context.Context, text string) error
	Close() error
}

type NullMonitorSink struct{}

func (NullMonitorSink) Speak(context.Context, string) error { return nil }
func (NullMonitorSink) Close() error                        { return nil }

func NewSystemMonitorSink(cfg MonitorConfig) MonitorSink {
	if !cfg.Enabled {
		diag.Infof("monitor sink disabled")
		return NullMonitorSink{}
	}
	diag.Infof("monitor sink enabled output=%q rate=%d volume=%d", cfg.OutputDevice, cfg.Rate, cfg.Volume)
	return &systemSpeechMonitor{
		outputDevice: cfg.OutputDevice,
		rate:         clamp(cfg.Rate, -10, 10, DefaultMonitorRate),
		volume:       clamp(cfg.Volume, 0, 100, DefaultMonitorVolume),
	}
}

type systemSpeechMonitor struct {
	mu           sync.Mutex
	outputDevice string
	rate         int
	volume       int
}

func (m *systemSpeechMonitor) Speak(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch runtime.GOOS {
	case "darwin":
		return m.speakDarwin(ctx, text)
	case "windows":
		script := `$s=New-Object System.Speech.Synthesis.SpeechSynthesizer; $s.SetOutputToDefaultAudioDevice(); $s.Volume=[int]$args[0]; $s.Rate=[int]$args[1]; $s.Speak($args[2])`
		return exec.CommandContext(
			ctx,
			"powershell",
			"-NoProfile",
			"-ExecutionPolicy", "Bypass",
			"-Command", script,
			strconv.Itoa(m.volume),
			strconv.Itoa(m.rate),
			text,
		).Run()
	default:
		return nil
	}
}

func (m *systemSpeechMonitor) Close() error { return nil }

func (m *systemSpeechMonitor) speakDarwin(ctx context.Context, text string) error {
	if idx, ok := resolveAudioDeviceIndex(m.outputDevice); ok {
		if _, err := exec.LookPath("ffmpeg"); err == nil {
			diag.Infof("monitor speak via audiotoolbox output=%q index=%d chars=%d", m.outputDevice, idx, len(text))
			return m.speakDarwinAudioToolbox(ctx, text, idx)
		}
	}
	diag.Infof("monitor speak via say default output=%q chars=%d", m.outputDevice, len(text))
	args := []string{"-r", macSpeechRate(m.rate), text}
	return exec.CommandContext(ctx, "say", args...).Run()
}

func (m *systemSpeechMonitor) speakDarwinAudioToolbox(ctx context.Context, text string, deviceIndex int) error {
	f, err := os.CreateTemp("", "stealthcopilot-monitor-*.aiff")
	if err != nil {
		return err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)

	if err := exec.CommandContext(ctx, "say", "-r", macSpeechRate(m.rate), "-o", path, text).Run(); err != nil {
		return err
	}

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-i", path,
		"-f", "audiotoolbox",
		"-audio_device_index", strconv.Itoa(deviceIndex),
		"-",
	}
	return exec.CommandContext(ctx, "ffmpeg", args...).Run()
}

func macSpeechRate(rate int) string {
	// macOS say uses words per minute. Keep the UI scale small and predictable.
	return strconv.Itoa(190 + rate*12)
}

func clamp(v, min, max, def int) int {
	if v == 0 {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
