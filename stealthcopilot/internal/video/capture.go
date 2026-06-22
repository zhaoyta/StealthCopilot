// Package video implements local camera frame helpers and virtual-camera output primitives.
// 生产实现依赖 gocv（OpenCV CGO）；未安装时自动降级为 NullCaptureProvider（静态彩色帧）。
package video

import (
	"context"
	"image"
	"image/color"
	"sync"
	"time"
)

const (
	// TargetFPS 视频链目标帧率
	TargetFPS = 30
	// FrameDur 每帧时长（1s / 30fps ≈ 33ms）
	FrameDur = time.Second / TargetFPS
	// DefaultWidth 摄像头默认宽度（像素）
	DefaultWidth = 640
	// DefaultHeight 摄像头默认高度（像素）
	DefaultHeight = 480
)

// CaptureProvider 从物理摄像头持续捕获 BGRA 视频帧。
// 实现须以 TargetFPS 节拍输出帧；ctx 取消时关闭 channel。
type CaptureProvider interface {
	// Start 开始捕获，返回视频帧 channel；deviceName 为摄像头名称或索引。
	Start(ctx context.Context, deviceName string) (<-chan Frame, error)
	// ListDevices 返回当前系统可用摄像头设备名称列表。
	ListDevices() []string
	// Close 停止捕获并释放设备资源。
	Close() error
}

// NullCaptureProvider 以固定彩色帧模拟摄像头输入，用于：
//  1. gocv / OpenCV 未安装时的降级运行
//  2. 单元测试（无需真实摄像头）
type NullCaptureProvider struct {
	mu   sync.Mutex
	stop chan struct{}
	once sync.Once
}

// Start 以 FrameDur 间隔输出固定颜色（深蓝）的 BGRA 帧，直到 ctx 取消。
func (n *NullCaptureProvider) Start(ctx context.Context, _ string) (<-chan Frame, error) {
	n.mu.Lock()
	if n.stop != nil {
		close(n.stop)
	}
	n.stop = make(chan struct{})
	stop := n.stop
	n.mu.Unlock()

	ch := make(chan Frame, 4)
	frame := makeBlankFrame(DefaultWidth, DefaultHeight)

	go func() {
		defer close(ch)
		ticker := time.NewTicker(FrameDur)
		defer ticker.Stop()
		pts := int64(0)
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				pts += FrameDur.Milliseconds()
				f := Frame{Data: frame, PTS: pts}
				select {
				case ch <- f:
				default: // 下游消费不及时时丢帧，不阻塞捕获
				}
			}
		}
	}()
	return ch, nil
}

// ListDevices 返回空列表（Null 实现无真实设备）。
func (n *NullCaptureProvider) ListDevices() []string { return []string{} }

// Close 停止内部 goroutine。
func (n *NullCaptureProvider) Close() error {
	n.once.Do(func() {
		n.mu.Lock()
		if n.stop != nil {
			close(n.stop)
			n.stop = nil
		}
		n.mu.Unlock()
	})
	return nil
}

// makeBlankFrame 创建指定尺寸的 BGRA 纯色帧（深蓝，模拟摄像头画面）。
func makeBlankFrame(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	c := color.NRGBA{B: 80, A: 255} // BGRA: B=80, G=0, R=0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img.Pix // NRGBA Pix 与 BGRA 字节序不同，Null 场景仅测试用途
}
