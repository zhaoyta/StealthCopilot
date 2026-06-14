package audio

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
)

// NewSystemCaptureProvider returns the best available audio capture provider.
// ffmpeg is already used for device enumeration, so this keeps runtime
// requirements aligned with the rest of the app while avoiding a hard CGO
// dependency on PortAudio.
func NewSystemCaptureProvider() CaptureProvider {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return &FFmpegCaptureProvider{}
	}
	return &NullCaptureProvider{}
}

// NewSystemMicProvider returns the best available microphone provider.
func NewSystemMicProvider() MicProvider {
	return &systemMicProvider{capture: NewSystemCaptureProvider()}
}

type systemMicProvider struct {
	capture CaptureProvider
}

func (p *systemMicProvider) Start(ctx context.Context, deviceName string) (<-chan []byte, error) {
	return p.capture.Start(ctx, deviceName)
}

func (p *systemMicProvider) Close() error {
	return p.capture.Close()
}

// FFmpegCaptureProvider captures 16kHz mono s16le PCM frames from a system
// audio input using ffmpeg.
type FFmpegCaptureProvider struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

func (p *FFmpegCaptureProvider) Start(ctx context.Context, deviceName string) (<-chan []byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg 未安装，无法启动真实音频采集: %w", err)
	}

	args, err := ffmpegAudioCaptureArgs(deviceName)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = cancel
	p.cmd = cmd
	p.mu.Unlock()

	ch := make(chan []byte, 8)
	go func() {
		defer close(ch)
		defer func() {
			cancel()
			_ = cmd.Wait()
		}()

		for {
			frame := make([]byte, FrameBytes)
			if _, err := io.ReadFull(stdout, frame); err != nil {
				return
			}
			select {
			case ch <- frame:
			case <-runCtx.Done():
				return
			default:
			}
		}
	}()

	return ch, nil
}

func (p *FFmpegCaptureProvider) Close() error {
	p.mu.Lock()
	cancel := p.cancel
	cmd := p.cmd
	p.cancel = nil
	p.cmd = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return nil
}

func ffmpegAudioCaptureArgs(deviceName string) ([]string, error) {
	base := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
	}

	switch runtime.GOOS {
	case "darwin":
		input := ":0"
		if deviceName != "" {
			input = ":" + deviceName
		}
		return append(base,
			"-f", "avfoundation",
			"-i", input,
			"-ac", "1",
			"-ar", fmt.Sprintf("%d", SampleRate),
			"-f", "s16le",
			"pipe:1",
		), nil
	case "windows":
		input := "audio=default"
		if deviceName != "" {
			input = "audio=" + deviceName
		}
		return append(base,
			"-f", "dshow",
			"-i", input,
			"-ac", "1",
			"-ar", fmt.Sprintf("%d", SampleRate),
			"-f", "s16le",
			"pipe:1",
		), nil
	default:
		return nil, fmt.Errorf("当前系统暂不支持 ffmpeg 音频采集: %s", runtime.GOOS)
	}
}
