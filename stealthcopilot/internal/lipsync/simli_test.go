package lipsync

import (
	"context"
	"testing"
)

func TestMarshalParseAudioFrame(t *testing.T) {
	pts := int64(12345678)
	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	payload := marshalAudioFrame(pts, pcm)

	if len(payload) != 8+len(pcm) {
		t.Fatalf("payload len: expected %d, got %d", 8+len(pcm), len(payload))
	}

	// 重新解析 PTS
	gotPTS := int64(payload[0])<<56 | int64(payload[1])<<48 | int64(payload[2])<<40 |
		int64(payload[3])<<32 | int64(payload[4])<<24 | int64(payload[5])<<16 |
		int64(payload[6])<<8 | int64(payload[7])
	if gotPTS != pts {
		t.Errorf("PTS round-trip: expected %d, got %d", pts, gotPTS)
	}
}

func TestParseSimliFrame_TooShort(t *testing.T) {
	_, err := parseSimliFrame([]byte{0x01, 0x02})
	if err == nil {
		t.Error("expected error for too-short frame")
	}
}

func TestParseSimliFrame_Valid(t *testing.T) {
	pts := int64(999)
	data := marshalAudioFrame(pts, []byte{0xFF, 0xD8}) // fake JPEG
	frame, err := parseSimliFrame(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frame.PTS != pts {
		t.Errorf("PTS: expected %d, got %d", pts, frame.PTS)
	}
	if len(frame.Data) != 2 {
		t.Errorf("Data len: expected 2, got %d", len(frame.Data))
	}
}

func TestNullLipSyncProvider_Passthrough(t *testing.T) {
	p := NewNullLipSyncProvider()
	if err := p.Start(context.Background(), "face-id"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	frame := VideoFrame{Data: []byte{1, 2, 3}, PTS: 42}
	if err := p.SendVideo(frame); err != nil {
		t.Fatalf("SendVideo: %v", err)
	}

	select {
	case got := <-p.Output():
		if got.PTS != 42 {
			t.Errorf("Output PTS: expected 42, got %d", got.PTS)
		}
	default:
		t.Error("expected frame in Output channel")
	}

	p.Close()
}

func TestNullLipSyncProvider_SendAudio(t *testing.T) {
	p := NewNullLipSyncProvider()
	// SendAudio 不应返回错误（丢弃操作）
	if err := p.SendAudio(AudioChunk{PTS: 1}); err != nil {
		t.Errorf("SendAudio: unexpected error: %v", err)
	}
}
