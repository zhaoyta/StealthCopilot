package audio

import (
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
