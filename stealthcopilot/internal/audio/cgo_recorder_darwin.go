//go:build darwin && cgo

// Package audio — cgo_recorder_darwin.go 使用 CoreAudio AudioQueue 在进程内录音。
// 权限归属于 Wails 进程本身，避免 ffmpeg 子进程无法触发 macOS TCC 权限弹框的问题。
package audio

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AudioToolbox -framework AVFoundation -framework Foundation

#import <AudioToolbox/AudioToolbox.h>
#import <AVFoundation/AVFoundation.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

// 每个录音缓冲区的大小（字节）
#define kBufferByteSize 4096
#define kNumBuffers 3

// CGO 回调无法直接使用 Go 函数指针，改用全局 C 缓冲区 + Go 侧轮询。
// 录音数据写入此环形缓冲区，Go 侧通过 drainRecorderBuffer 周期取出。
#define kRingSize (1024 * 1024) // 1MB
static uint8_t  gRingBuf[kRingSize];
static uint32_t gRingWrite = 0;
static uint32_t gRingRead  = 0;
static int      gRecording = 0;
static char     gError[512] = {0};

static AudioQueueRef       gQueue    = NULL;
static AudioQueueBufferRef gBuffers[kNumBuffers];

static void audioQueueCallback(void *inUserData,
                                AudioQueueRef inAQ,
                                AudioQueueBufferRef inBuffer,
                                const AudioTimeStamp *inStartTime,
                                UInt32 inNumPackets,
                                const AudioStreamPacketDescription *inPacketDesc) {
    if (!gRecording) return;
    uint32_t bytes = inBuffer->mAudioDataByteSize;
    uint8_t *src   = (uint8_t *)inBuffer->mAudioData;
    for (uint32_t i = 0; i < bytes; i++) {
        gRingBuf[gRingWrite % kRingSize] = src[i];
        gRingWrite++;
    }
    AudioQueueEnqueueBuffer(inAQ, inBuffer, 0, NULL);
}

// requestMicPermission 触发 macOS AVFoundation 麦克风权限弹框。
// 返回 1 表示已授权，0 表示拒绝。
static int requestMicPermission(void) {
    __block int granted = 0;
    dispatch_semaphore_t sema = dispatch_semaphore_create(0);
    [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio
                             completionHandler:^(BOOL g) {
        granted = g ? 1 : 0;
        dispatch_semaphore_signal(sema);
    }];
    dispatch_semaphore_wait(sema, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
    return granted;
}

// startAudioQueue 初始化并启动 AudioQueue 录音（16kHz, 16bit, mono）。
// 返回 NULL 表示成功，否则返回错误描述字符串（调用者负责 free）。
static char* startAudioQueue(void) {
    gRingWrite = 0;
    gRingRead  = 0;
    gRecording = 1;
    memset(gError, 0, sizeof(gError));

    AudioStreamBasicDescription fmt = {0};
    fmt.mSampleRate       = 16000;
    fmt.mFormatID         = kAudioFormatLinearPCM;
    fmt.mFormatFlags      = kLinearPCMFormatFlagIsSignedInteger | kLinearPCMFormatFlagIsPacked;
    fmt.mBitsPerChannel   = 16;
    fmt.mChannelsPerFrame = 1;
    fmt.mBytesPerFrame    = 2;
    fmt.mFramesPerPacket  = 1;
    fmt.mBytesPerPacket   = 2;

    OSStatus st = AudioQueueNewInput(&fmt, audioQueueCallback, NULL, NULL, NULL, 0, &gQueue);
    if (st != noErr) {
        char *err = (char*)malloc(64);
        snprintf(err, 64, "AudioQueueNewInput error: %d", (int)st);
        gRecording = 0;
        return err;
    }

    for (int i = 0; i < kNumBuffers; i++) {
        AudioQueueAllocateBuffer(gQueue, kBufferByteSize, &gBuffers[i]);
        AudioQueueEnqueueBuffer(gQueue, gBuffers[i], 0, NULL);
    }

    st = AudioQueueStart(gQueue, NULL);
    if (st != noErr) {
        AudioQueueDispose(gQueue, true);
        gQueue = NULL;
        gRecording = 0;
        char *err = (char*)malloc(64);
        snprintf(err, 64, "AudioQueueStart error: %d", (int)st);
        return err;
    }
    return NULL;
}

// stopAudioQueue 停止并销毁 AudioQueue。
static void stopAudioQueue(void) {
    gRecording = 0;
    if (gQueue != NULL) {
        AudioQueueStop(gQueue, true);
        AudioQueueDispose(gQueue, true);
        gQueue = NULL;
    }
}

// drainRecorderBuffer 将环形缓冲区中已有数据复制到 out（最多 maxBytes 字节）。
// 返回实际复制的字节数。
static int drainRecorderBuffer(uint8_t *out, int maxBytes) {
    uint32_t avail = gRingWrite - gRingRead;
    if (avail == 0) return 0;
    if ((int)avail > maxBytes) avail = (uint32_t)maxBytes;
    for (uint32_t i = 0; i < avail; i++) {
        out[i] = gRingBuf[(gRingRead + i) % kRingSize];
    }
    gRingRead += avail;
    return (int)avail;
}
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

// darwinCGORecorder 使用 AudioQueue 在进程内直接录音，避免 ffmpeg 子进程的 TCC 权限问题。
type darwinCGORecorder struct{}

// requestMicPermission 触发 macOS 麦克风权限弹框，返回是否已授权。
func requestMicPermission() bool {
	return C.requestMicPermission() == 1
}

func (r *darwinCGORecorder) start() error {
	if !requestMicPermission() {
		return fmt.Errorf("麦克风权限被拒绝，请前往「系统设置 → 隐私与安全性 → 麦克风」开启本应用的访问权限")
	}
	cerr := C.startAudioQueue()
	if cerr != nil {
		msg := C.GoString(cerr)
		C.free(unsafe.Pointer(cerr))
		return fmt.Errorf("AudioQueue 启动失败：%s", msg)
	}
	return nil
}

func (r *darwinCGORecorder) stop() []byte {
	C.stopAudioQueue()
	// 给 AudioQueue 100ms 排空最后缓冲区
	time.Sleep(100 * time.Millisecond)
	buf := make([]byte, 4*1024*1024) // 最大 4MB
	n := C.drainRecorderBuffer((*C.uint8_t)(unsafe.Pointer(&buf[0])), C.int(len(buf)))
	return buf[:int(n)]
}

// newSystemVoiceRecorder 返回 CGO AudioQueue 录音实现（darwin）。
func newSystemVoiceRecorder() voiceRecorderImpl {
	return &darwinCGORecorder{}
}
