package audio

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// NewSystemVirtualMicWriter returns a real virtual mic writer when ffmpeg can
// expose a writable platform audio sink.
func NewSystemVirtualMicWriter(deviceName string) VirtualMicWriter {
	writer, _ := NewSystemVirtualMicWriterChecked(deviceName)
	return writer
}

func NewSystemVirtualMicWriterChecked(deviceName string) (VirtualMicWriter, string) {
	if strings.TrimSpace(deviceName) == "" {
		return NewNullVirtualMicWriter(), ""
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return NewNullVirtualMicWriter(), "ffmpeg 未安装，无法写入真实虚拟麦克风"
	}
	w, err := NewFFmpegVirtualMicWriter(deviceName)
	if err != nil {
		return NewNullVirtualMicWriter(), "虚拟麦克风写入器启动失败：" + err.Error()
	}
	return w, ""
}

// FFmpegVirtualMicWriter streams PCM into a writable platform audio sink and
// preserves the Zero-PCM/TTS state machine used by the speaking chain.
type FFmpegVirtualMicWriter struct {
	state  atomic.Int32
	mu     sync.Mutex
	stdin  io.WriteCloser
	cmd    *exec.Cmd
	done   chan struct{}
	once   sync.Once
	closed bool
}

func NewFFmpegVirtualMicWriter(deviceName string) (*FFmpegVirtualMicWriter, error) {
	args, err := ffmpegVirtualMicArgs(deviceName)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("ffmpeg", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}

	w := &FFmpegVirtualMicWriter{
		stdin: stdin,
		cmd:   cmd,
		done:  make(chan struct{}),
	}
	go w.zeroPCMLoop()
	return w, nil
}

func (w *FFmpegVirtualMicWriter) BeginZeroPCM() {
	w.state.Store(int32(micStateZeroPCM))
}

func (w *FFmpegVirtualMicWriter) WriteChunk(chunk []byte) {
	w.state.CompareAndSwap(int32(micStateZeroPCM), int32(micStateTTS))
	w.write(chunk)
}

func (w *FFmpegVirtualMicWriter) EndTTS() {
	w.state.Store(int32(micStateIdle))
}

func (w *FFmpegVirtualMicWriter) Close() {
	w.once.Do(func() {
		close(w.done)
		w.mu.Lock()
		w.closed = true
		stdin := w.stdin
		cmd := w.cmd
		w.stdin = nil
		w.cmd = nil
		w.mu.Unlock()
		if stdin != nil {
			_ = stdin.Close()
		}
		if cmd != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
		}
	})
}

func (w *FFmpegVirtualMicWriter) zeroPCMLoop() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	silence := make([]byte, VirtualMicSampleRate/100*BytesPerSample)
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			if virtualMicState(w.state.Load()) == micStateZeroPCM {
				w.write(silence)
			}
		}
	}
}

func (w *FFmpegVirtualMicWriter) write(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.stdin == nil {
		return
	}
	_, _ = w.stdin.Write(chunk)
}

func ffmpegVirtualMicArgs(deviceName string) ([]string, error) {
	return ffmpegVirtualMicArgsForGOOS(runtime.GOOS, deviceName)
}

func ffmpegVirtualMicArgsForGOOS(goos, deviceName string) ([]string, error) {
	base := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-f", "s16le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", VirtualMicSampleRate),
		"-i", "pipe:0",
	}

	switch goos {
	case "darwin":
		args := append(base, "-f", "audiotoolbox")
		if idx, ok := parseAudioDeviceIndex(deviceName); ok {
			args = append(args, "-audio_device_index", strconv.Itoa(idx))
		}
		return append(args, "-"), nil
	case "windows":
		if strings.TrimSpace(deviceName) == "" {
			return nil, fmt.Errorf("Windows 虚拟麦克风设备名称未配置")
		}
		return append(base, "-f", "dshow", "audio="+deviceName), nil
	default:
		return nil, fmt.Errorf("当前系统不支持直接写入 ffmpeg 虚拟麦克风: %s", goos)
	}
}

func parseAudioDeviceIndex(deviceName string) (int, bool) {
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		return -1, false
	}
	idx, err := strconv.Atoi(deviceName)
	if err != nil || idx < 0 {
		return -1, false
	}
	return idx, true
}
