// Package digitalhuman receives Simli WebRTC video and decodes it to BGRA
// frames for the configured video output.
package digitalhuman

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/video"
)

const (
	simliVideoWidth    = 512
	simliVideoHeight   = 512
	simliVideoBGRASize = simliVideoWidth * simliVideoHeight * 4

	simliVideoFPS   = 30
	simliFrameDurNs = 1_000_000_000 / simliVideoFPS
	simliWriteEvery = time.Second / simliVideoFPS
)

var annexBStartCode = []byte{0x00, 0x00, 0x00, 0x01}

func startSimliVideoReceive(ctx context.Context, track *webrtc.TrackRemote, cw video.VirtualCameraWriter) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		diag.Warnf("simli video: ffmpeg not found, video output disabled")
		return
	}

	if strings.EqualFold(track.Codec().MimeType, webrtc.MimeTypeVP8) {
		startSimliVP8RTPReceive(ctx, track, cw)
		return
	}
	startSimliH264Receive(ctx, track, cw)
}

func startSimliH264Receive(ctx context.Context, track *webrtc.TrackRemote, cw video.VirtualCameraWriter) {
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "h264",
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("scale=%d:%d", simliVideoWidth, simliVideoHeight),
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"pipe:1",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		diag.Warnf("simli video: stdin pipe err=%v", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		diag.Warnf("simli video: stdout pipe err=%v", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		diag.Warnf("simli video: stderr pipe err=%v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		diag.Warnf("simli video: ffmpeg start err=%v", err)
		return
	}
	diag.Infof("simli video: ffmpeg decoder started codec=%s input=h264", track.Codec().MimeType)

	go pipeDecodedSimliFrames(stdout, cw, cmd)
	go logSimliVideoStderr(stderr)

	defer stdin.Close()
	writeSimliH264RTP(ctx, track, stdin)
}

func startSimliVP8RTPReceive(ctx context.Context, track *webrtc.TrackRemote, cw video.VirtualCameraWriter) {
	port, err := reserveUDPPort()
	if err != nil {
		diag.Warnf("simli video: reserve rtp port err=%v", err)
		return
	}
	sdp := fmt.Sprintf(`v=0
o=- 0 0 IN IP4 127.0.0.1
s=Simli VP8
c=IN IP4 127.0.0.1
t=0 0
m=video %d RTP/AVP %d
a=rtpmap:%d VP8/90000
`, port, track.PayloadType(), track.PayloadType())

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-protocol_whitelist", "file,pipe,udp,rtp",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-f", "sdp",
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("scale=%d:%d", simliVideoWidth, simliVideoHeight),
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"pipe:1",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		diag.Warnf("simli video: stdin pipe err=%v", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		diag.Warnf("simli video: stdout pipe err=%v", err)
		_ = stdin.Close()
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		diag.Warnf("simli video: stderr pipe err=%v", err)
		_ = stdin.Close()
		return
	}
	if err := cmd.Start(); err != nil {
		diag.Warnf("simli video: ffmpeg start err=%v", err)
		_ = stdin.Close()
		return
	}
	if _, err := io.WriteString(stdin, sdp); err != nil {
		diag.Warnf("simli video: write sdp err=%v", err)
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		return
	}
	_ = stdin.Close()
	diag.Infof("simli video: ffmpeg decoder started codec=%s input=rtp port=%d payload_type=%d", track.Codec().MimeType, port, track.PayloadType())

	go pipeDecodedSimliFrames(stdout, cw, cmd)
	go logSimliVideoStderr(stderr)
	writeSimliVP8RTPToUDP(ctx, track, port)
}

func reserveUDPPort() (int, error) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port, nil
}

func pipeDecodedSimliFrames(stdout io.Reader, cw video.VirtualCameraWriter, cmd *exec.Cmd) {
	defer cmd.Wait()
	buf := make([]byte, simliVideoBGRASize)
	var pts int64
	var frames int64
	var written int64
	var lastWrite time.Time
	for {
		if _, err := io.ReadFull(stdout, buf); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				diag.Warnf("simli video: frame read err=%v", err)
			}
			return
		}
		frames++
		now := time.Now()
		if !lastWrite.IsZero() && now.Sub(lastWrite) < simliWriteEvery {
			continue
		}
		lastWrite = now
		frame := make([]byte, simliVideoBGRASize)
		copy(frame, buf)
		if writeErr := cw.WriteFrame(video.Frame{Data: frame, PTS: pts}); writeErr != nil {
			diag.Warnf("simli video: write frame err=%v", writeErr)
		}
		written++
		if written == 1 || written%150 == 0 {
			diag.Infof("simli video: frame written frames=%d emitted=%d pts=%d", frames, written, pts)
		}
		pts += simliFrameDurNs
	}
}

func logSimliVideoStderr(stderr io.Reader) {
	buf, _ := io.ReadAll(stderr)
	if len(buf) > 0 {
		diag.Warnf("simli video: ffmpeg stderr=%q", trimSimliVideoLog(string(buf), 2000))
	}
}

func trimSimliVideoLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func writeSimliVP8RTPToUDP(ctx context.Context, track *webrtc.TrackRemote, port int) {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err != nil {
		diag.Warnf("simli video: udp dial err=%v", err)
		return
	}
	defer conn.Close()
	started := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		pkt, _, err := track.ReadRTP()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			diag.Warnf("simli video: rtp read err=%v", err)
			return
		}
		raw, err := pkt.Marshal()
		if err != nil {
			diag.Warnf("simli video: marshal vp8 rtp err=%v", err)
			return
		}
		if _, err := conn.Write(raw); err != nil {
			if ctx.Err() != nil {
				return
			}
			if strings.Contains(err.Error(), "connection refused") && time.Since(started) < 2*time.Second {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			diag.Warnf("simli video: write vp8 rtp udp err=%v", err)
			return
		}
	}
}

func writeSimliH264RTP(ctx context.Context, track *webrtc.TrackRemote, stdin io.Writer) {
	h264 := &codecs.H264Packet{}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		pkt, _, err := track.ReadRTP()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			diag.Warnf("simli video: rtp read err=%v", err)
			return
		}
		if len(pkt.Payload) == 0 {
			continue
		}
		nalData, err := h264.Unmarshal(pkt.Payload)
		if err != nil || len(nalData) == 0 {
			continue
		}
		if _, err := stdin.Write(annexBStartCode); err != nil {
			if ctx.Err() != nil {
				return
			}
			diag.Warnf("simli video: write start code err=%v", err)
			return
		}
		if _, err := stdin.Write(nalData); err != nil {
			if ctx.Err() != nil {
				return
			}
			diag.Warnf("simli video: write nal err=%v", err)
			return
		}
	}
}
