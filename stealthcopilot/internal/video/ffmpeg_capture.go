package video

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// NewSystemCaptureProvider returns the best available camera capture provider.
func NewSystemCaptureProvider() CaptureProvider {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return &FFmpegCaptureProvider{}
	}
	return &NullCaptureProvider{}
}

// FFmpegCaptureProvider captures BGRA raw frames from the system camera.
type FFmpegCaptureProvider struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

func (p *FFmpegCaptureProvider) Start(ctx context.Context, deviceName string) (<-chan Frame, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg 未安装，无法启动真实摄像头采集: %w", err)
	}

	args, err := ffmpegVideoCaptureArgs(deviceName)
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

	ch := make(chan Frame, 4)
	go func() {
		defer close(ch)
		defer func() {
			cancel()
			_ = cmd.Wait()
		}()

		frameBytes := DefaultWidth * DefaultHeight * 4
		pts := int64(0)
		for {
			frame := make([]byte, frameBytes)
			if _, err := io.ReadFull(stdout, frame); err != nil {
				return
			}
			pts += FrameDur.Milliseconds()
			select {
			case ch <- Frame{Data: frame, PTS: pts}:
			case <-runCtx.Done():
				return
			default:
			}
		}
	}()

	return ch, nil
}

func (p *FFmpegCaptureProvider) ListDevices() []string {
	args := []string{"-hide_banner", "-f", ffmpegDeviceFormat(), "-list_devices", "true", "-i", ffmpegListInput()}
	out, _ := exec.Command("ffmpeg", args...).CombinedOutput()
	return append([]string{}, parseFFmpegVideoDevices(string(out))...)
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

func ffmpegVideoCaptureArgs(deviceName string) ([]string, error) {
	base := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
	}

	switch runtime.GOOS {
	case "darwin":
		input := "0:none"
		if deviceName != "" {
			input = deviceName + ":none"
		}
		return append(base,
			"-f", "avfoundation",
			"-framerate", fmt.Sprintf("%d", TargetFPS),
			"-video_size", fmt.Sprintf("%dx%d", DefaultWidth, DefaultHeight),
			"-i", input,
			"-pix_fmt", "bgra",
			"-f", "rawvideo",
			"pipe:1",
		), nil
	case "windows":
		input := "video=default"
		if deviceName != "" {
			input = "video=" + deviceName
		}
		return append(base,
			"-f", "dshow",
			"-framerate", fmt.Sprintf("%d", TargetFPS),
			"-video_size", fmt.Sprintf("%dx%d", DefaultWidth, DefaultHeight),
			"-i", input,
			"-pix_fmt", "bgra",
			"-f", "rawvideo",
			"pipe:1",
		), nil
	default:
		return nil, fmt.Errorf("当前系统暂不支持 ffmpeg 摄像头采集: %s", runtime.GOOS)
	}
}

func ffmpegDeviceFormat() string {
	if runtime.GOOS == "windows" {
		return "dshow"
	}
	return "avfoundation"
}

func ffmpegListInput() string {
	if runtime.GOOS == "windows" {
		return "dummy"
	}
	return ""
}

func parseFFmpegVideoDevices(raw string) []string {
	devices := make([]string, 0)
	lines := strings.Split(raw, "\n")
	inVideo := false
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if strings.Contains(l, "AVFoundation video devices") || strings.Contains(l, "DirectShow video devices") {
			inVideo = true
			continue
		}
		if strings.Contains(l, "AVFoundation audio devices") || strings.Contains(l, "DirectShow audio devices") {
			inVideo = false
			continue
		}
		if !inVideo {
			continue
		}
		if runtime.GOOS == "windows" {
			if strings.HasPrefix(l, "\"") {
				end := strings.LastIndex(l, "\"")
				if end > 0 {
					devices = append(devices, l[1:end])
				}
			}
			continue
		}
		if !strings.Contains(l, "] [") {
			continue
		}
		parts := strings.SplitN(l, "] [", 2)
		if len(parts) != 2 {
			continue
		}
		right := parts[1]
		idEnd := strings.Index(right, "]")
		if idEnd < 0 {
			continue
		}
		name := strings.TrimSpace(right[idEnd+1:])
		if name != "" {
			devices = append(devices, name)
		}
	}
	return devices
}
