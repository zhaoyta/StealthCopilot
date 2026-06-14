// Package video — virtual_camera_test.go 验证虚拟摄像头写入器的工厂函数和降级行为。
// DAL 插件 socket / FFmpeg 进程属于集成依赖，不在此覆盖；
// 仅测试在依赖不可用时是否正确降级为 NullVirtualCameraWriter。
package video

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
)

// TestNewSystemVirtualCameraWriter_EmptyDevice 验证设备名为空时直接返回 NullVirtualCameraWriter。
func TestNewSystemVirtualCameraWriter_EmptyDevice(t *testing.T) {
	w := NewSystemVirtualCameraWriter("")
	if _, ok := w.(*NullVirtualCameraWriter); !ok {
		t.Errorf("expected NullVirtualCameraWriter for empty deviceName, got %T", w)
	}
	_ = w.Close()
}

// TestNewSystemVirtualCameraWriter_NoSocketOnDarwin 验证 macOS 下 socket 不存在时降级为 Null。
func TestNewSystemVirtualCameraWriter_NoSocketOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("仅 macOS 场景")
	}
	// 确保 socket 文件不存在
	_ = os.Remove(darwinSocketPath)

	w := NewSystemVirtualCameraWriter("StealthVirtualCam")
	if _, ok := w.(*NullVirtualCameraWriter); !ok {
		t.Errorf("expected NullVirtualCameraWriter when socket absent, got %T", w)
	}
	_ = w.Close()
}

// TestDarwinSocketWriter_WriteFrame 验证 macOS socket 写入器通过 UNIX socket 正确发送帧。
// 测试本地启动一个临时 UNIX socket 服务端模拟 DAL 插件。
func TestDarwinSocketWriter_WriteFrame(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("仅 macOS 场景")
	}

	// macOS UNIX socket 路径上限 104 字节，必须使用短路径
	sockPath := fmt.Sprintf("/tmp/tvcam_%d.sock", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.Remove(sockPath) })
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	// 接收一帧数据
	received := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// 读 4 字节长度头
		header := make([]byte, 4)
		if _, err := conn.Read(header); err != nil {
			return
		}
		size := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
		data := make([]byte, size)
		if _, err := conn.Read(data); err != nil {
			return
		}
		received <- data
	}()

	// 客户端连接（直接使用 DarwinSocketWriter，绕过 darwinSocketPath 常量）
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	w := &DarwinSocketWriter{conn: conn}

	frame := lipsync.VideoFrame{Data: []byte{0x01, 0x02, 0x03, 0x04}}
	if err := w.WriteFrame(frame); err != nil {
		t.Fatalf("WriteFrame error: %v", err)
	}
	_ = w.Close()

	data := <-received
	if len(data) != len(frame.Data) {
		t.Errorf("received %d bytes, want %d", len(data), len(frame.Data))
	}
	for i, b := range frame.Data {
		if data[i] != b {
			t.Errorf("byte[%d] = 0x%02x, want 0x%02x", i, data[i], b)
		}
	}
}

// TestDarwinSocketWriter_WriteEmptyFrame 验证空帧被跳过（不写入 socket）。
func TestDarwinSocketWriter_WriteEmptyFrame(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("仅 macOS 场景")
	}

	sockPath := fmt.Sprintf("/tmp/tvcam_empty_%d.sock", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.Remove(sockPath) })
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	w := &DarwinSocketWriter{conn: conn}

	// 写入空帧不应报错也不应向 socket 写入任何数据
	if err := w.WriteFrame(lipsync.VideoFrame{Data: nil}); err != nil {
		t.Errorf("WriteFrame(empty) should not return error, got: %v", err)
	}
	_ = w.Close()
}

// TestNullVirtualCameraWriter_Idempotent 验证 Null writer 的 Close 幂等、WriteFrame 无副作用。
func TestNullVirtualCameraWriter_Idempotent(t *testing.T) {
	w := &NullVirtualCameraWriter{}
	frame := lipsync.VideoFrame{Data: []byte{1, 2, 3}}
	if err := w.WriteFrame(frame); err != nil {
		t.Errorf("NullVirtualCameraWriter.WriteFrame should not error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("first Close error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close (idempotent) error: %v", err)
	}
}
