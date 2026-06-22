package video

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const defaultOBSBrowserSourceAddr = "127.0.0.1:18765"

func DefaultOBSBrowserSourceURL() string {
	return "http://" + defaultOBSBrowserSourceAddr + "/"
}

// OBSBrowserSourceWriter exposes the latest digital-human frame as a local
// browser source. OBS owns the virtual camera; the app only supplies scene
// content that OBS can capture.
type OBSBrowserSourceWriter struct {
	mu          sync.RWMutex
	cond        *sync.Cond
	server      *http.Server
	listener    net.Listener
	url         string
	rawLatest   []byte
	rawWidth    int
	rawHeight   int
	rawSeq      uint64
	encodedSeq  uint64
	latestJPEG  []byte
	streamSeq   uint64
	closed      bool
	encoderDone chan struct{}
}

// NewOBSBrowserSourceWriter starts a local HTTP server for OBS Browser Source.
func NewOBSBrowserSourceWriter(addr string) (*OBSBrowserSourceWriter, error) {
	if addr == "" {
		addr = defaultOBSBrowserSourceAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("OBS 本地视频源端口不可用: %w", err)
	}
	w := &OBSBrowserSourceWriter{
		listener:    ln,
		encoderDone: make(chan struct{}),
	}
	w.cond = sync.NewCond(&w.mu)
	w.url = "http://" + ln.Addr().String() + "/"
	mux := http.NewServeMux()
	mux.HandleFunc("/", w.handlePage)
	mux.HandleFunc("/stream.mjpg", w.handleStream)
	w.server = &http.Server{Handler: mux}
	go func() {
		if err := w.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			diag.Warnf("obs browser source server stopped err=%v", err)
		}
	}()
	go w.encodeLoop()
	diag.Infof("obs browser source ready url=%s", w.url)
	return w, nil
}

func (w *OBSBrowserSourceWriter) URL() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.url
}

func (w *OBSBrowserSourceWriter) WriteFrame(frame Frame) error {
	if len(frame.Data) == 0 {
		return nil
	}
	width, height, ok := inferBGRAFrameSize(len(frame.Data))
	if !ok {
		return fmt.Errorf("unsupported BGRA frame size: %d bytes", len(frame.Data))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.rawLatest = append(w.rawLatest[:0], frame.Data...)
	w.rawWidth = width
	w.rawHeight = height
	w.rawSeq++
	w.cond.Broadcast()
	return nil
}

func (w *OBSBrowserSourceWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.cond.Broadcast()
	w.mu.Unlock()
	select {
	case <-w.encoderDone:
	case <-time.After(500 * time.Millisecond):
		diag.Warnf("obs browser source encoder wait timed out")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return w.server.Shutdown(ctx)
}

func (w *OBSBrowserSourceWriter) encodeLoop() {
	defer close(w.encoderDone)
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()
	for {
		<-ticker.C

		w.mu.Lock()
		for !w.closed && (w.rawSeq == 0 || w.rawSeq == w.encodedSeq) {
			w.cond.Wait()
		}
		if w.closed {
			w.mu.Unlock()
			return
		}
		raw := append([]byte(nil), w.rawLatest...)
		width := w.rawWidth
		height := w.rawHeight
		rawSeq := w.rawSeq
		w.mu.Unlock()

		jpg, err := encodeBGRAJPEG(raw, width, height)
		if err != nil {
			diag.Warnf("obs browser source encode err=%v", err)
			continue
		}

		w.mu.Lock()
		if !w.closed && rawSeq > w.encodedSeq {
			w.latestJPEG = jpg
			w.encodedSeq = rawSeq
			w.streamSeq++
			w.cond.Broadcast()
		}
		w.mu.Unlock()
	}
}

func (w *OBSBrowserSourceWriter) handlePage(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write([]byte(`<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    html, body { margin: 0; width: 100%; height: 100%; background: #000; overflow: hidden; }
    body { display: grid; place-items: center; }
    img { width: 100vw; height: 100vh; object-fit: contain; background: #000; }
  </style>
</head>
<body>
  <img src="/stream.mjpg" alt="">
</body>
</html>`))
}

func (w *OBSBrowserSourceWriter) handleStream(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	rw.Header().Set("Cache-Control", "no-store")
	flusher, _ := rw.(http.Flusher)
	var seen uint64
	for {
		w.mu.Lock()
		for !w.closed && w.streamSeq == seen {
			w.cond.Wait()
		}
		if w.closed {
			w.mu.Unlock()
			return
		}
		jpg := append([]byte(nil), w.latestJPEG...)
		seen = w.streamSeq
		w.mu.Unlock()
		select {
		case <-req.Context().Done():
			return
		default:
		}
		if _, err := fmt.Fprintf(rw, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(jpg)); err != nil {
			return
		}
		if _, err := rw.Write(jpg); err != nil {
			return
		}
		if _, err := rw.Write([]byte("\r\n")); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func inferBGRAFrameSize(bytes int) (int, int, bool) {
	switch bytes {
	case 512 * 512 * 4:
		return 512, 512, true
	case DefaultWidth * DefaultHeight * 4:
		return DefaultWidth, DefaultHeight, true
	default:
		return 0, 0, false
	}
}

func encodeBGRAJPEG(bgra []byte, width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		dstRow := y * img.Stride
		for x := 0; x < width; x++ {
			src := (y*width + x) * 4
			dst := dstRow + x*4
			img.Pix[dst] = bgra[src+2]
			img.Pix[dst+1] = bgra[src+1]
			img.Pix[dst+2] = bgra[src]
			img.Pix[dst+3] = bgra[src+3]
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
