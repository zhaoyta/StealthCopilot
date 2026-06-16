package audio

import (
	"reflect"
	"testing"
	"time"
)

func TestNullVirtualMicWriter_StateTransitions(t *testing.T) {
	w := NewNullVirtualMicWriter()
	defer w.Close()

	// 初始状态 idle
	if virtualMicState(w.state.Load()) != micStateIdle {
		t.Error("initial state should be idle")
	}

	// BeginZeroPCM → zeroPCM
	w.BeginZeroPCM()
	if virtualMicState(w.state.Load()) != micStateZeroPCM {
		t.Error("state should be zeroPCM after BeginZeroPCM")
	}

	// WriteChunk 首次调用 → TTS
	w.WriteChunk([]byte{0x01, 0x02})
	if virtualMicState(w.state.Load()) != micStateTTS {
		t.Error("state should be tts after first WriteChunk")
	}

	// WriteChunk 再次调用 → 状态不变（仍是 TTS）
	w.WriteChunk([]byte{0x03, 0x04})
	if virtualMicState(w.state.Load()) != micStateTTS {
		t.Error("state should remain tts after subsequent WriteChunk calls")
	}

	// EndTTS → idle
	w.EndTTS()
	if virtualMicState(w.state.Load()) != micStateIdle {
		t.Error("state should be idle after EndTTS")
	}
}

func TestNullVirtualMicWriter_CloseTwice(t *testing.T) {
	w := NewNullVirtualMicWriter()
	// Close 两次不应 panic（sync.Once 保护）
	w.Close()
	w.Close()
}

func TestNullVirtualMicWriter_ZeroPCMLoopRuns(t *testing.T) {
	w := NewNullVirtualMicWriter()
	w.BeginZeroPCM()
	// 等待几个帧周期（10ms/帧），zeroPCMLoop 不崩溃即视为通过
	time.Sleep(50 * time.Millisecond)
	w.EndTTS()
	w.Close()
}

func TestFFmpegVirtualMicArgs_DarwinDefaultOutput(t *testing.T) {
	got, err := ffmpegVirtualMicArgsForGOOS("darwin", "")
	if err != nil {
		t.Fatalf("ffmpegVirtualMicArgsForGOOS(darwin): %v", err)
	}
	want := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-f", "s16le",
		"-ac", "1",
		"-ar", "24000",
		"-i", "pipe:0",
		"-f", "audiotoolbox",
		"-",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("darwin args = %#v, want %#v", got, want)
	}
}

func TestFFmpegVirtualMicArgs_DarwinDeviceIndex(t *testing.T) {
	got, err := ffmpegVirtualMicArgsForGOOS("darwin", "2")
	if err != nil {
		t.Fatalf("ffmpegVirtualMicArgsForGOOS(darwin): %v", err)
	}
	want := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-f", "s16le",
		"-ac", "1",
		"-ar", "24000",
		"-i", "pipe:0",
		"-f", "audiotoolbox",
		"-audio_device_index", "2",
		"-",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("darwin indexed args = %#v, want %#v", got, want)
	}
}

func TestFFmpegVirtualMicArgs_WindowsRequiresNamedDevice(t *testing.T) {
	if _, err := ffmpegVirtualMicArgsForGOOS("windows", ""); err == nil {
		t.Fatal("expected error for empty Windows virtual mic device")
	}
}

func TestFFmpegVirtualMicArgs_WindowsNamedDevice(t *testing.T) {
	got, err := ffmpegVirtualMicArgsForGOOS("windows", "VB-Cable")
	if err != nil {
		t.Fatalf("ffmpegVirtualMicArgsForGOOS(windows): %v", err)
	}
	want := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-f", "s16le",
		"-ac", "1",
		"-ar", "24000",
		"-i", "pipe:0",
		"-f", "dshow",
		"audio=VB-Cable",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows args = %#v, want %#v", got, want)
	}
}

func TestNewSystemVirtualMicWriterChecked_EmptyDeviceAllowsNull(t *testing.T) {
	writer, msg := NewSystemVirtualMicWriterChecked("")
	defer writer.Close()
	if msg != "" {
		t.Fatalf("message = %q, want empty", msg)
	}
	if _, ok := writer.(*NullVirtualMicWriter); !ok {
		t.Fatalf("writer = %T, want *NullVirtualMicWriter", writer)
	}
}

func TestNewSystemVirtualMicWriterChecked_MissingFFmpegReportsError(t *testing.T) {
	t.Setenv("PATH", "")
	writer, msg := NewSystemVirtualMicWriterChecked("1")
	defer writer.Close()
	if msg == "" {
		t.Fatal("expected missing ffmpeg error")
	}
	if _, ok := writer.(*NullVirtualMicWriter); !ok {
		t.Fatalf("writer = %T, want *NullVirtualMicWriter", writer)
	}
}
