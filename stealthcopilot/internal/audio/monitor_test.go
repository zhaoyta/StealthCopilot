package audio

import "testing"

func TestNewSystemMonitorSink_DisabledReturnsNull(t *testing.T) {
	sink := NewSystemMonitorSink(MonitorConfig{Enabled: false})
	if _, ok := sink.(NullMonitorSink); !ok {
		t.Fatalf("sink = %T, want NullMonitorSink", sink)
	}
}

func TestNewSystemMonitorSink_PreservesOutputDevice(t *testing.T) {
	sink := NewSystemMonitorSink(MonitorConfig{
		Enabled:      true,
		OutputDevice: "2",
		Rate:         99,
		Volume:       -1,
	})
	monitor, ok := sink.(*systemSpeechMonitor)
	if !ok {
		t.Fatalf("sink = %T, want *systemSpeechMonitor", sink)
	}
	if monitor.outputDevice != "2" {
		t.Fatalf("outputDevice = %q, want %q", monitor.outputDevice, "2")
	}
	if monitor.rate != 10 {
		t.Fatalf("rate = %d, want clamped 10", monitor.rate)
	}
	if monitor.volume != 0 {
		t.Fatalf("volume = %d, want clamped 0", monitor.volume)
	}
}
