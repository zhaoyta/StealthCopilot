package circuit

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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

// TestBreaker_HTTPHeartbeat_Success 验证 HTTP URL 心跳：服务端返回 200 时视为存活。
func TestBreaker_HTTPHeartbeat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b := NewBreaker(srv.URL, nil)
	if !b.sendHeartbeat() {
		t.Fatal("HTTP heartbeat should succeed when server returns 200")
	}
}

// TestBreaker_HTTPHeartbeat_4xx 验证 HTTP 4xx（如 401）仍视为连通（网络正常，业务层问题）。
func TestBreaker_HTTPHeartbeat_4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	b := NewBreaker(srv.URL, nil)
	if !b.sendHeartbeat() {
		t.Fatal("HTTP 4xx should still be treated as alive (network is reachable)")
	}
}

// TestBreaker_HTTPHeartbeat_5xx 验证 HTTP 5xx 视为失联（服务端异常）。
func TestBreaker_HTTPHeartbeat_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b := NewBreaker(srv.URL, nil)
	if b.sendHeartbeat() {
		t.Fatal("HTTP 5xx should be treated as failure")
	}
}

// TestBreaker_HTTPHeartbeat_Unreachable 验证无法连接的 HTTP 地址视为失联。
func TestBreaker_HTTPHeartbeat_Unreachable(t *testing.T) {
	// 使用不存在的本地地址
	b := NewBreaker("http://127.0.0.1:19999", nil)
	if b.sendHeartbeat() {
		t.Fatal("unreachable HTTP address should return false")
	}
}

// TestBreaker_TripsOnHTTPFailure 验证连续 HTTP 心跳失败后触发熔断。
func TestBreaker_TripsOnHTTPFailure(t *testing.T) {
	// 立即关闭服务端，模拟断线
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var tripped bool
	b := NewBreaker(srv.URL, func(s State) {
		if s == StateOpen {
			tripped = true
		}
	})

	// 手动执行 tripThreshold 次失败心跳
	for range tripThreshold {
		b.handleHeartbeat(false)
	}

	if !tripped {
		t.Error("breaker should have tripped after consecutive HTTP failures")
	}
	if !b.IsOpen() {
		t.Error("IsOpen should be true after trip")
	}
}
