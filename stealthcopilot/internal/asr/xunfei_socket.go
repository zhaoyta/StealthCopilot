package asr

import "github.com/gorilla/websocket"

func xunfeiWebSocketDialer() *websocket.Dialer {
	dialer := *websocket.DefaultDialer
	// Audio streams are long binary uploads; local system proxies can close them mid-segment.
	dialer.Proxy = nil
	return &dialer
}
