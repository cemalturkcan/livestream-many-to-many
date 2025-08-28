package websocket

import (
	"sync"
	"time"

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

type Message struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type Room struct {
	ID           string
	Streamers    map[string]*Streamer
	mutex        sync.RWMutex
	CreatedBy    string
	LastActivity time.Time
	Paused       bool
}

type Streamer struct {
	ID                string
	Room              *Room
	PeerConnections   []*PeerConnectionState
	TrackLocals       map[string]*webrtc.TrackLocalStaticRTP
	ViewerConnections []*PeerConnectionState
	VideoTracks       map[string]*webrtc.TrackLocalStaticRTP
	AudioTracks       map[string]*webrtc.TrackLocalStaticRTP
	CameraEnabled     bool
	MicrophoneEnabled bool
	mutex             sync.RWMutex
}

func NewRoom(id string, createdBy string) *Room {
	return &Room{
		ID:           id,
		Streamers:    make(map[string]*Streamer),
		CreatedBy:    createdBy,
		LastActivity: time.Now(),
	}
}

func NewStreamer(id string, room *Room) *Streamer {
	return &Streamer{
		ID:                id,
		Room:              room,
		TrackLocals:       make(map[string]*webrtc.TrackLocalStaticRTP),
		VideoTracks:       make(map[string]*webrtc.TrackLocalStaticRTP),
		AudioTracks:       make(map[string]*webrtc.TrackLocalStaticRTP),
		CameraEnabled:     true,
		MicrophoneEnabled: true,
	}
}

type User struct {
	Id       string
	Username string
	Avatar   string
}
