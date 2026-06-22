package video

import (
	"io"
	"net/http"
	"testing"
	"time"
)

func TestOBSBrowserSourceWriterServesMJPEG(t *testing.T) {
	w, err := NewOBSBrowserSourceWriter("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewOBSBrowserSourceWriter: %v", err)
	}
	defer w.Close()

	frame := make([]byte, 512*512*4)
	for i := 0; i < len(frame); i += 4 {
		frame[i] = 0x20
		frame[i+1] = 0x80
		frame[i+2] = 0xd0
		frame[i+3] = 0xff
	}
	if err := w.WriteFrame(Frame{Data: frame}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(w.URL() + "stream.mjpg")
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()
	if got, want := resp.Header.Get("Content-Type"), "multipart/x-mixed-replace; boundary=frame"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
	buf := make([]byte, 4096)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("read stream: %v", err)
	}
	if n == 0 {
		t.Fatal("expected non-empty mjpeg response")
	}
}

func TestOBSBrowserSourceWriterRejectsUnknownFrameSize(t *testing.T) {
	w, err := NewOBSBrowserSourceWriter("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewOBSBrowserSourceWriter: %v", err)
	}
	defer w.Close()
	if err := w.WriteFrame(Frame{Data: []byte{1, 2, 3}}); err == nil {
		t.Fatal("expected error for unknown frame size")
	}
}

func TestOBSBrowserSourceWriterWriteFrameIsAsync(t *testing.T) {
	w, err := NewOBSBrowserSourceWriter("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewOBSBrowserSourceWriter: %v", err)
	}
	defer w.Close()

	frame := make([]byte, 512*512*4)
	started := time.Now()
	for i := 0; i < 20; i++ {
		if err := w.WriteFrame(Frame{Data: frame}); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}
	if elapsed := time.Since(started); elapsed > 150*time.Millisecond {
		t.Fatalf("WriteFrame took %s, expected async raw-frame handoff", elapsed)
	}
}
