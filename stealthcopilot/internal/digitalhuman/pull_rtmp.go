// Package digitalhuman — pull_rtmp.go 通过 FFmpeg 子进程从 RTMP CDN 中继地址
// 拉取 ZEGO 数字人音视频流，并将音频 PCM 和视频 BGRA 帧分别投递给注册的 sink。
//
// 使用前提：ZEGO 控制台已开启混流转推 CDN，并将数字人流中继到 RTMP 地址。
// 音频格式：S16LE，24000 Hz，单声道，10ms 分块（480 字节/块）。
// 视频格式：BGRA，640×480，30fps，每帧 1,228,800 字节。
package digitalhuman

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

const (
	// rtmpAudioSampleRate 与虚拟麦克风输出采样率一致（24000 Hz S16LE 单声道）
	rtmpAudioSampleRate = 24000
	// rtmpAudioBytesPerSample S16LE = 2 字节/样本
	rtmpAudioBytesPerSample = 2
	// rtmpAudioFrameMs 每次投递给 audioSink 的分块时长（毫秒）
	rtmpAudioFrameMs = 10
	// rtmpAudioChunkBytes 每帧字节数 = 24000 / 1000 * 10 * 2 = 480
	rtmpAudioChunkBytes = rtmpAudioSampleRate / 1000 * rtmpAudioFrameMs * rtmpAudioBytesPerSample

	// rtmpVideoWidth/Height/BPP 与 video 包的 DefaultWidth/DefaultHeight/BGRA 一致
	rtmpVideoWidth  = 640
	rtmpVideoHeight = 480
	rtmpVideoBPP    = 4 // BGRA
	rtmpVideoFPS    = 30
	// rtmpVideoFrameBytes 每帧字节数 = 640 × 480 × 4 = 1,228,800
	rtmpVideoFrameBytes = rtmpVideoWidth * rtmpVideoHeight * rtmpVideoBPP
)

// FFmpegRTMPPullClient 通过两个 FFmpeg 子进程（音频/视频分离）从 RTMP CDN 地址
// 拉取 ZEGO 数字人音视频流。Address 为 ZEGO CDN 混流转推输出的 RTMP 地址。
type FFmpegRTMPPullClient struct {
	// Address RTMP CDN 中继地址，例如 rtmp://cdn.example.com/live/stream_id
	Address string

	mu       sync.Mutex
	audioCmd *exec.Cmd
	videoCmd *exec.Cmd
}

// Start 启动音频和视频两个 FFmpeg 拉流子进程。
// audioSink 和 videoSink 均可为 nil（跳过对应拉流进程）。
// ctx 取消时，FFmpeg 进程通过 CommandContext 自动终止。
func (c *FFmpegRTMPPullClient) Start(ctx context.Context, cfg PullConfig, audioSink func([]byte), videoSink func(VideoFrame)) error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("ZEGO 数字人 RTMP 拉流地址未配置；请在设置中填写 ZEGO CDN 转推拉流地址")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errors.New("未找到 ffmpeg，无法拉取数字人 RTC 流；请安装 ffmpeg 后重试")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if audioSink != nil {
		cmd, err := c.startAudioProcess(ctx, audioSink)
		if err != nil {
			return fmt.Errorf("启动数字人音频拉流失败：%w", err)
		}
		c.audioCmd = cmd
	}

	if videoSink != nil {
		cmd, err := c.startVideoProcess(ctx, videoSink)
		if err != nil {
			if c.audioCmd != nil {
				_ = c.audioCmd.Process.Kill()
				_ = c.audioCmd.Wait()
				c.audioCmd = nil
			}
			return fmt.Errorf("启动数字人视频拉流失败：%w", err)
		}
		c.videoCmd = cmd
	}

	diag.Infof("digitalhuman rtmp pull started address=%q audio=%t video=%t", c.Address, audioSink != nil, videoSink != nil)
	return nil
}

// startAudioProcess 启动 FFmpeg 子进程拉取音频，以 S16LE PCM 送往 audioSink。
// 每次投递 10ms 分块（480 字节）。
func (c *FFmpegRTMPPullClient) startAudioProcess(ctx context.Context, audioSink func([]byte)) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", c.Address,
		"-vn",
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", rtmpAudioSampleRate),
		"-ac", "1",
		"pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go pipeAudioSink(stdout, audioSink)
	return cmd, nil
}

// startVideoProcess 启动 FFmpeg 子进程拉取视频，以 BGRA 原始帧送往 videoSink。
// 每帧大小固定为 rtmpVideoFrameBytes 字节。
func (c *FFmpegRTMPPullClient) startVideoProcess(ctx context.Context, videoSink func(VideoFrame)) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", c.Address,
		"-an",
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"-s", fmt.Sprintf("%dx%d", rtmpVideoWidth, rtmpVideoHeight),
		"-r", fmt.Sprintf("%d", rtmpVideoFPS),
		"pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go pipeVideoSink(stdout, videoSink)
	return cmd, nil
}

// Close 终止所有 FFmpeg 拉流子进程并释放资源。
func (c *FFmpegRTMPPullClient) Close() error {
	c.mu.Lock()
	audioCmd := c.audioCmd
	videoCmd := c.videoCmd
	c.audioCmd = nil
	c.videoCmd = nil
	c.mu.Unlock()

	if audioCmd != nil && audioCmd.Process != nil {
		_ = audioCmd.Process.Kill()
		_ = audioCmd.Wait()
	}
	if videoCmd != nil && videoCmd.Process != nil {
		_ = videoCmd.Process.Kill()
		_ = videoCmd.Wait()
	}
	diag.Infof("digitalhuman rtmp pull closed")
	return nil
}

// pipeAudioSink 从 FFmpeg stdout 持续读取 PCM 数据，按 rtmpAudioChunkBytes 分块投递给 sink。
// 管道关闭（FFmpeg 结束或 ctx 取消）时退出。
func pipeAudioSink(r io.Reader, sink func([]byte)) {
	buf := make([]byte, rtmpAudioChunkBytes)
	for {
		_, err := io.ReadFull(r, buf)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				diag.Warnf("digitalhuman rtmp audio pipe read error: %v", err)
			}
			return
		}
		chunk := make([]byte, rtmpAudioChunkBytes)
		copy(chunk, buf)
		sink(chunk)
	}
}

// pipeVideoSink 从 FFmpeg stdout 持续读取 BGRA 原始视频帧，每帧投递给 sink。
// PTS 以帧索引乘以帧时长（毫秒）估算。
func pipeVideoSink(r io.Reader, sink func(VideoFrame)) {
	buf := make([]byte, rtmpVideoFrameBytes)
	const frameDurMs = 1000 / rtmpVideoFPS
	var frameIdx int64
	for {
		_, err := io.ReadFull(r, buf)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				diag.Warnf("digitalhuman rtmp video pipe read error: %v", err)
			}
			return
		}
		data := make([]byte, rtmpVideoFrameBytes)
		copy(data, buf)
		pts := frameIdx * frameDurMs
		frameIdx++
		sink(VideoFrame{Data: data, PTS: pts})
	}
}
