package audio

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const (
	captureFrameLogEvery = 1000
	captureQueueSize     = 512
	captureSlowSendWarn  = 200
)

// NewSystemCaptureProvider returns the best available audio capture provider.
// ffmpeg is already used for device enumeration, so this keeps runtime
// requirements aligned with the rest of the app while avoiding a hard CGO
// dependency on PortAudio.
func NewSystemCaptureProvider() CaptureProvider {
	provider, _ := NewSystemCaptureProviderChecked()
	return provider
}

func NewSystemCaptureProviderChecked() (CaptureProvider, string) {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return &FFmpegCaptureProvider{}, ""
	}
	return &NullCaptureProvider{}, "ffmpeg 未安装，无法启动真实音频采集"
}

// NewSystemMicProvider returns the best available microphone provider.
func NewSystemMicProvider() MicProvider {
	provider, _ := NewSystemMicProviderChecked()
	return provider
}

func NewSystemMicProviderChecked() (MicProvider, string) {
	capture, msg := NewSystemCaptureProviderChecked()
	return &systemMicProvider{capture: capture}, msg
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
	done   chan struct{}
}

func (p *FFmpegCaptureProvider) Start(ctx context.Context, deviceName string) (<-chan []byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg 未安装，无法启动真实音频采集: %w", err)
	}

	args, err := ffmpegAudioCaptureArgs(deviceName)
	if err != nil {
		return nil, err
	}
	diag.Infof("audio capture start device=%q args=%q", deviceName, args)

	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	diag.Infof("audio capture ffmpeg started pid=%d device=%q", cmd.Process.Pid, deviceName)

	done := make(chan struct{})
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = cancel
	p.cmd = cmd
	p.done = done
	p.mu.Unlock()

	ch := make(chan []byte, captureQueueSize)
	go func() {
		defer close(done)
		defer close(ch)
		defer func() {
			cancel()
			if err := cmd.Wait(); err != nil && runCtx.Err() == nil {
				diag.Warnf("audio capture ffmpeg exited device=%q err=%v", deviceName, err)
			} else {
				diag.Infof("audio capture stopped device=%q", deviceName)
			}
		}()

		frameCount := 0
		for {
			frame := make([]byte, FrameBytes)
			if _, err := io.ReadFull(stdout, frame); err != nil {
				if runCtx.Err() == nil {
					diag.Warnf("audio capture read ended device=%q frames=%d err=%v", deviceName, frameCount, err)
				}
				return
			}
			frameCount++
			peak := pcmPeak(frame)
			if frameCount == 1 || frameCount%captureFrameLogEvery == 0 {
				diag.Infof("audio capture summary device=%q frames=%d peak=%d queue_depth=%d", deviceName, frameCount, peak, len(ch))
			}
			sendStarted := time.Now()
			select {
			case ch <- frame:
			case <-runCtx.Done():
				return
			}
			if blockedMs := time.Since(sendStarted).Milliseconds(); blockedMs >= captureSlowSendWarn {
				diag.Warnf("audio capture backpressure device=%q frames=%d blocked_ms=%d queue_depth=%d peak=%d", deviceName, frameCount, blockedMs, len(ch), peak)
			}
		}
	}()
	go func() {
		buf, _ := io.ReadAll(stderr)
		if len(buf) > 0 && runCtx.Err() == nil {
			diag.Warnf("audio capture ffmpeg stderr device=%q stderr=%q", deviceName, limitLogString(string(buf), 2000))
		}
	}()

	return ch, nil
}

func (p *FFmpegCaptureProvider) Close() error {
	p.mu.Lock()
	cancel := p.cancel
	cmd := p.cmd
	done := p.done
	p.cancel = nil
	p.cmd = nil
	p.done = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		if done != nil {
			select {
			case <-done:
			case <-time.After(500 * time.Millisecond):
				diag.Warnf("audio capture ffmpeg wait timed out")
			}
		}
	}
	return nil
}

func limitLogString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
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
