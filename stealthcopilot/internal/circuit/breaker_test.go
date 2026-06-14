package circuit

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestBreaker_InitialState(t *testing.T) {
	b := NewBreaker("", nil)
	if b.CurrentState() != StateClosed {
		t.Error("initial state should be Closed")
	}
	if b.IsOpen() {
		t.Error("IsOpen should be false initially")
	}
}

func TestBreaker_TripFromLag(t *testing.T) {
	var got State
	b := NewBreaker("", func(s State) { got = s })

	b.TripFromLag(500)

	if b.CurrentState() != StateOpen {
		t.Errorf("expected StateOpen after trip, got %v", b.CurrentState())
	}
	if !b.IsOpen() {
		t.Error("IsOpen should be true after trip")
	}
	if got != StateOpen {
		t.Errorf("onStateChange: expected StateOpen, got %v", got)
	}
}

func TestBreaker_TripIdempotent(t *testing.T) {
	callCount := 0
	b := NewBreaker("", func(_ State) { callCount++ })

	b.TripFromLag(500)
	b.TripFromLag(500) // 重复触发不应重复回调

	if callCount != 1 {
		t.Errorf("expected 1 callback, got %d", callCount)
	}
}

func TestBreaker_StartStop(t *testing.T) {
	b := NewBreaker("", nil) // 空地址：心跳视为存活，不触发熔断
	ctx := context.Background()
	b.Start(ctx)

	time.Sleep(60 * time.Millisecond) // 等待至少一次心跳

	done := make(chan struct{})
	go func() {
		b.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Stop did not return within 2 seconds")
	}
}

func TestBreaker_NoTripOnEmptyAddr(t *testing.T) {
	b := NewBreaker("", nil) // 空地址 → sendHeartbeat 返回 true
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	b.Start(ctx)
	<-ctx.Done()
	b.Stop()

	if b.CurrentState() != StateClosed {
		t.Errorf("should remain Closed with empty addr, got %v", b.CurrentState())
	}
}

func TestBreaker_SendHeartbeatRequiresResponse(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer conn.Close()

	go func() {
		buf := make([]byte, 16)
		n, addr, readErr := conn.ReadFrom(buf)
		if readErr != nil || n == 0 {
			return
		}
		_, _ = conn.WriteTo([]byte("pong"), addr)
	}()

	b := NewBreaker(conn.LocalAddr().String(), nil)
	if !b.sendHeartbeat() {
		t.Fatal("sendHeartbeat should succeed when peer responds")
	}
}

func TestBreaker_SendHeartbeatFailsWithoutResponse(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer conn.Close()

	b := NewBreaker(conn.LocalAddr().String(), nil)
	if b.sendHeartbeat() {
		t.Fatal("sendHeartbeat should fail when peer does not respond")
	}
}
