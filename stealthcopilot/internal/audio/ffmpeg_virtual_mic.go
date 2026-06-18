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

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

// NewSystemVirtualMicWriter returns a real virtual mic writer when ffmpeg can
// expose a writable platform audio sink.
func NewSystemVirtualMicWriter(deviceName string) VirtualMicWriter {
	writer, _ := NewSystemVirtualMicWriterChecked(deviceName)
	return writer
}

func NewSystemVirtualMicWriterChecked(deviceName string) (VirtualMicWriter, string) {
	if strings.TrimSpace(deviceName) == "" {
		diag.Warnf("virtual mic writer using null writer: empty device")
		return NewNullVirtualMicWriter(), ""
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		diag.Errorf("virtual mic writer unavailable: ffmpeg missing device=%q", deviceName)
		return NewNullVirtualMicWriter(), "ffmpeg 未安装，无法写入真实虚拟麦克风"
	}
	w, err := NewFFmpegVirtualMicWriter(deviceName)
	if err != nil {
		diag.Errorf("virtual mic writer start failed device=%q err=%v", deviceName, err)
		return NewNullVirtualMicWriter(), "虚拟麦克风写入器启动失败：" + err.Error()
	}
	diag.Infof("virtual mic writer ready device=%q", deviceName)
	return w, ""
}

// FFmpegVirtualMicWriter streams PCM into a writable platform audio sink and
// preserves the Zero-PCM/TTS state machine used by the speaking chain.
type FFmpegVirtualMicWriter struct {
	state  atomic.Int32
	device string
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
	diag.Infof("virtual mic ffmpeg start device=%q args=%q", deviceName, args)
	cmd := exec.Command("ffmpeg", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	diag.Infof("virtual mic ffmpeg started pid=%d device=%q", cmd.Process.Pid, deviceName)

	w := &FFmpegVirtualMicWriter{
		device: deviceName,
		stdin:  stdin,
		cmd:    cmd,
		done:   make(chan struct{}),
	}
	go w.zeroPCMLoop()
	go func() {
		buf, _ := io.ReadAll(stderr)
		if len(buf) > 0 {
			diag.Warnf("virtual mic ffmpeg stderr device=%q stderr=%q", deviceName, limitLogString(string(buf), 2000))
		}
	}()
	return w, nil
}

func (w *FFmpegVirtualMicWriter) BeginZeroPCM() {
	diag.Infof("virtual mic begin zero-pcm device=%q", w.device)
	w.state.Store(int32(micStateZeroPCM))
}

func (w *FFmpegVirtualMicWriter) WriteChunk(chunk []byte) {
	if w.state.CompareAndSwap(int32(micStateZeroPCM), int32(micStateTTS)) {
		diag.Infof("virtual mic first tts chunk device=%q bytes=%d peak=%d", w.device, len(chunk), pcmPeak(chunk))
	}
	w.write(chunk)
}

func (w *FFmpegVirtualMicWriter) EndTTS() {
	diag.Infof("virtual mic end tts device=%q", w.device)
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
		diag.Infof("virtual mic closed device=%q", w.device)
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
		base = append(base, "-f", "audiotoolbox")
		if strings.TrimSpace(deviceName) == "" {
			return append(base, "-"), nil
		}
		if idx, ok := resolveAudioDeviceIndex(deviceName); ok {
			base = append(base, "-audio_device_index", strconv.Itoa(idx))
			diag.Infof("virtual mic darwin output resolved device=%q index=%d", deviceName, idx)
		} else {
			return nil, fmt.Errorf("无法解析 AudioToolbox 输出设备：%s", deviceName)
		}
		return append(base, "-"), nil
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
