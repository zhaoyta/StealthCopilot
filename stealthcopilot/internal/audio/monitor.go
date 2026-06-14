package audio

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
)

const (
	DefaultMonitorRate   = 0
	DefaultMonitorVolume = 80
)

// MonitorConfig controls the private translated-audio monitor used by the
// hearing chain. OutputDevice is persisted as the user's intended headphone
// route; system speech backends currently play through the OS default output.
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
		return NullMonitorSink{}
	}
	return &systemSpeechMonitor{
		rate:   clamp(cfg.Rate, -10, 10, DefaultMonitorRate),
		volume: clamp(cfg.Volume, 0, 100, DefaultMonitorVolume),
	}
}

type systemSpeechMonitor struct {
	mu     sync.Mutex
	rate   int
	volume int
}

func (m *systemSpeechMonitor) Speak(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch runtime.GOOS {
	case "darwin":
		args := []string{"-r", macSpeechRate(m.rate), text}
		return exec.CommandContext(ctx, "say", args...).Run()
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
