package video

import "testing"

func TestNullVirtualCameraWriter_Idempotent(t *testing.T) {
	w := &NullVirtualCameraWriter{}
	frame := Frame{Data: []byte{1, 2, 3}}
	if err := w.WriteFrame(frame); err != nil {
		t.Errorf("NullVirtualCameraWriter.WriteFrame should not error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("first Close error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close error: %v", err)
	}
}
