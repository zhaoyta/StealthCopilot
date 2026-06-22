package digitalhuman

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGenerateSignature(t *testing.T) {
	got := GenerateSignature(123456, "abc", "secret", 1700000000)
	want := "8e3042f61b5129b85d2750e7a1d5d143"
	if got != want {
		t.Fatalf("GenerateSignature() = %q, want %q", got, want)
	}
}

func TestClientCreateStreamTaskPayload(t *testing.T) {
	var action string
	var payload CreateStreamTaskRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action = r.URL.Query().Get("Action")
		if got := r.URL.Query().Get("AppId"); got != "123456" {
			t.Fatalf("AppId query = %q", got)
		}
		if sig := r.URL.Query().Get("Signature"); sig == "" {
			t.Fatal("Signature query should be set")
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_, _ = w.Write([]byte(`{"Code":0,"Message":"success","RequestId":"rid","Data":{"TaskId":"task-1"}}`))
	}))
	defer srv.Close()

	client := NewClient(Config{AppID: "123456", ServerSecret: "secret", Endpoint: srv.URL})
	taskID, err := client.CreateStreamTask(context.Background(), CreateStreamTaskRequest{
		DigitalHumanConfig: DigitalHumanConfig{DigitalHumanID: "dh-1"},
		RTCConfig:          RTCConfig{RoomID: "room-1", StreamID: "stream-1"},
	})
	if err != nil {
		t.Fatalf("CreateStreamTask: %v", err)
	}
	if action != "CreateDigitalHumanStreamTask" {
		t.Fatalf("Action = %q", action)
	}
	if taskID != "task-1" {
		t.Fatalf("taskID = %q", taskID)
	}
	if payload.DigitalHumanConfig.DigitalHumanID != "dh-1" || payload.RTCConfig.RoomID != "room-1" || payload.RTCConfig.StreamID != "stream-1" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestClientAPIErrorIsSecretSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Code":1001,"Message":"bad auth","RequestId":"rid","Data":{}}`))
	}))
	defer srv.Close()

	client := NewClient(Config{AppID: "123456", ServerSecret: "super-secret-value", Endpoint: srv.URL})
	_, err := client.CreateStreamTask(context.Background(), CreateStreamTaskRequest{})
	if err == nil {
		t.Fatal("expected API error")
	}
	msg := err.Error()
	if strings.Contains(msg, "super-secret-value") {
		t.Fatalf("error leaked secret: %s", msg)
	}
	if !strings.Contains(msg, "request_id=rid") {
		t.Fatalf("error should include request id, got %s", msg)
	}
}

func TestConfigReady(t *testing.T) {
	if ConfigReady(Config{}) {
		t.Fatal("empty config should not be ready")
	}
	if !ConfigReady(Config{
		AppID:          "123",
		ServerSecret:   "secret",
		DigitalHumanID: "dh",
		RoomID:         "room",
		StreamID:       "stream",
	}) {
		t.Fatal("complete config should be ready")
	}
}

func TestDriverCleansRemoteTaskWhenWebSocketFails(t *testing.T) {
	var stopped atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("Action") {
		case "CreateDigitalHumanStreamTask":
			_, _ = w.Write([]byte(`{"Code":0,"Message":"success","RequestId":"create","Data":{"TaskId":"task-1"}}`))
		case "DriveByWsStream":
			_, _ = w.Write([]byte(`{"Code":0,"Message":"success","RequestId":"drive","Data":{"DriveId":"drive-1","WssAddress":"ws://127.0.0.1:1/unavailable"}}`))
		case "StopDigitalHumanStreamTask":
			stopped.Store(true)
			_, _ = w.Write([]byte(`{"Code":0,"Message":"success","RequestId":"stop","Data":{}}`))
		default:
			t.Fatalf("unexpected action %q", r.URL.Query().Get("Action"))
		}
	}))
	defer srv.Close()

	driver := NewZegoDriver(Config{
		AppID:          "123456",
		ServerSecret:   "secret",
		DigitalHumanID: "dh",
		RoomID:         "room",
		StreamID:       "stream",
		Endpoint:       srv.URL,
		PullClient:     NullPullClient{},
	})
	if err := driver.Start(context.Background(), nil); err == nil {
		t.Fatal("expected websocket startup failure")
	}
	if !stopped.Load() {
		t.Fatal("expected remote task cleanup")
	}
}
