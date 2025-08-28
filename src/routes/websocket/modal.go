package websocket

import (
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/pion/webrtc/v4"
)

type ThreadSafeWriter struct {
	*websocket.Conn
	sync.Mutex
}

func (t *ThreadSafeWriter) WriteJSON(v interface{}) error {
	t.Lock()
	defer t.Unlock()
	return t.Conn.WriteJSON(v)
}

type PeerConnectionState struct {
	PeerConnection *webrtc.PeerConnection
	WebSocket      *ThreadSafeWriter
}

type WebSocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}
