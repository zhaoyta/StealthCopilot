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
	state       atomic.Int32
	device      string
	mu          sync.Mutex
	stdin       io.WriteCloser
	cmd         *exec.Cmd
	done        chan struct{}
	procDone    chan struct{}
	once        sync.Once
	closed      bool
	ttsSession  int64
	ttsBytes    int64
	ttsWrites   int64
	writeErrors int64
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
		device:   deviceName,
		stdin:    stdin,
		cmd:      cmd,
		done:     make(chan struct{}),
		procDone: make(chan struct{}),
	}
	go w.zeroPCMLoop()
	go func() {
		defer close(w.procDone)
		if err := cmd.Wait(); err != nil {
			diag.Warnf("virtual mic ffmpeg exited device=%q err=%v", deviceName, err)
		} else {
			diag.Infof("virtual mic ffmpeg exited device=%q", deviceName)
		}
	}()
	go func() {
		buf, _ := io.ReadAll(stderr)
		if len(buf) > 0 {
			diag.Warnf("virtual mic ffmpeg stderr device=%q stderr=%q", deviceName, limitLogString(string(buf), 2000))
		}
	}()
	return w, nil
}

func (w *FFmpegVirtualMicWriter) BeginZeroPCM() {
	session := atomic.AddInt64(&w.ttsSession, 1)
	diag.Infof("virtual mic begin zero-pcm device=%q session=%d", w.device, session)
	atomic.StoreInt64(&w.ttsBytes, 0)
	atomic.StoreInt64(&w.ttsWrites, 0)
	atomic.StoreInt64(&w.writeErrors, 0)
	w.state.Store(int32(micStateZeroPCM))
}

func (w *FFmpegVirtualMicWriter) WriteChunk(chunk []byte) {
	session := atomic.LoadInt64(&w.ttsSession)
	if w.state.CompareAndSwap(int32(micStateZeroPCM), int32(micStateTTS)) {
		diag.Infof("virtual mic first tts chunk device=%q session=%d bytes=%d peak=%d", w.device, session, len(chunk), pcmPeak(chunk))
	}
	if err := w.write(chunk); err != nil {
		errCount := atomic.AddInt64(&w.writeErrors, 1)
		if errCount == 1 || errCount%20 == 0 {
			diag.Warnf("virtual mic write failed device=%q session=%d errors=%d err=%v", w.device, session, errCount, err)
		}
		return
	}
	atomic.AddInt64(&w.ttsBytes, int64(len(chunk)))
	atomic.AddInt64(&w.ttsWrites, 1)
}

func (w *FFmpegVirtualMicWriter) EndTTS() {
	diag.Infof("virtual mic end tts device=%q session=%d writes=%d bytes=%d write_errors=%d", w.device, atomic.LoadInt64(&w.ttsSession), atomic.LoadInt64(&w.ttsWrites), atomic.LoadInt64(&w.ttsBytes), atomic.LoadInt64(&w.writeErrors))
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
			select {
			case <-w.procDone:
			case <-time.After(500 * time.Millisecond):
				diag.Warnf("virtual mic ffmpeg wait timed out device=%q", w.device)
			}
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
			state := virtualMicState(w.state.Load())
			if state == micStateIdle || state == micStateZeroPCM {
				if err := w.write(silence); err != nil {
					errCount := atomic.AddInt64(&w.writeErrors, 1)
					if errCount == 1 || errCount%100 == 0 {
						diag.Warnf("virtual mic silence write failed device=%q state=%d errors=%d err=%v", w.device, state, errCount, err)
					}
				}
			}
		}
	}
}

func (w *FFmpegVirtualMicWriter) write(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.stdin == nil {
		return fmt.Errorf("writer closed")
	}
	select {
	case <-w.procDone:
		return fmt.Errorf("ffmpeg process exited")
	default:
	}
	n, err := w.stdin.Write(chunk)
	if err != nil {
		return err
	}
	if n != len(chunk) {
		return io.ErrShortWrite
	}
	return nil
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
