package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const systemTTSChunkSize = 4096

// SystemProvider 使用操作系统默认语音合成生成普通人声音频。
// 它不依赖声音复刻训练，适合作为说话链默认音色。
type SystemProvider struct{}

// NewSystemProvider 创建默认音色 TTS Provider。
func NewSystemProvider() *SystemProvider {
	return &SystemProvider{}
}

// Synthesize 将文本合成为 24kHz mono s16le PCM chunk。
func (p *SystemProvider) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	pcm, err := synthesizeSystemPCM(ctx, text, runtime.GOOS)
	if err != nil {
		return nil, err
	}
	ch := make(chan []byte, 16)
	go func() {
		defer close(ch)
		reader := bytes.NewReader(pcm)
		buf := make([]byte, systemTTSChunkSize)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				select {
				case ch <- chunk:
				case <-ctx.Done():
					return
				}
			}
			if readErr == io.EOF {
				return
			}
			if readErr != nil {
				return
			}
		}
	}()
	return ch, nil
}

// VoiceID 返回默认音色标识，便于日志区分个人复刻音色。
func (p *SystemProvider) VoiceID() string { return "system-default" }

// Close 无需释放外部资源。
func (p *SystemProvider) Close() error { return nil }

func synthesizeSystemPCM(ctx context.Context, text, goos string) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("system_tts: ffmpeg 未安装，无法生成默认音色音频")
	}
	speechFile, err := createSystemSpeechFile(ctx, text, goos)
	if err != nil {
		return nil, err
	}
	defer os.Remove(speechFile)

	args := systemTTSFFmpegArgs(speechFile)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("system_tts: ffmpeg 转码失败：%s", strings.TrimSpace(stderr.String()))
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("system_tts: 默认音色未生成音频")
	}
	return out.Bytes(), nil
}

func createSystemSpeechFile(ctx context.Context, text, goos string) (string, error) {
	switch goos {
	case "darwin":
		return createDarwinSpeechFile(ctx, text)
	case "windows":
		return createWindowsSpeechFile(ctx, text)
	default:
		return "", fmt.Errorf("system_tts: 当前系统暂不支持默认音色 TTS: %s", goos)
	}
}

func createDarwinSpeechFile(ctx context.Context, text string) (string, error) {
	file, err := os.CreateTemp("", "stealthcopilot-system-tts-*.aiff")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_ = file.Close()
	_ = os.Remove(path)

	cmd := exec.CommandContext(ctx, "say", "-o", path, text)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("system_tts: macOS say 生成失败：%s", strings.TrimSpace(string(out)))
	}
	return path, nil
}

func createWindowsSpeechFile(ctx context.Context, text string) (string, error) {
	file, err := os.CreateTemp("", "stealthcopilot-system-tts-*.wav")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_ = file.Close()
	_ = os.Remove(path)

	script := `$text = [Console]::In.ReadToEnd(); ` +
		`Add-Type -AssemblyName System.Speech; ` +
		`$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; ` +
		`$s.SetOutputToWaveFile($args[0]); ` +
		`$s.Speak($text); ` +
		`$s.Dispose()`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script, path)
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("system_tts: Windows SAPI 生成失败：%s", strings.TrimSpace(string(out)))
	}
	return path, nil
}

func systemTTSFFmpegArgs(inputPath string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-i", inputPath,
		"-f", "s16le",
		"-ac", "1",
		"-ar", "24000",
		"pipe:1",
	}
}
