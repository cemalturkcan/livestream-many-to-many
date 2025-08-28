package websocket

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/pion/webrtc/v4"
	"livestream/app/firebase/livedatabase"
	"sync"
	"time"
)

var (
	rooms     = make(map[string]*Room)
	roomsLock sync.RWMutex
)

func CreateRoom(roomID string, createdBy string) *Room {
	roomsLock.Lock()
	defer roomsLock.Unlock()

	room, exists := rooms[roomID]
	if !exists {
		room = NewRoom(roomID, createdBy)
		rooms[roomID] = room
		room.GetOrCreateStreamer(createdBy)
		firebaseRoom := &livedatabase.FirebaseRoom{
			Id:      roomID,
			Streams: []livedatabase.Stream{},
		}
		if err := livedatabase.CreateRoom(roomID, firebaseRoom); err != nil {
			log.Infof("Error creating room: %v", err)
		}
	}
	return room
}

func GetRoom(roomID string) (*Room, bool) {
	room, exists := rooms[roomID]
	return room, exists
}

func (r *Room) GetOrCreateStreamer(streamerID string) *Streamer {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	streamer, exists := r.Streamers[streamerID]
	if !exists {
		streamer = NewStreamer(streamerID, r)
		r.Streamers[streamerID] = streamer
	}
	return streamer
}

func (r *Room) GetStreamer(streamerID string) *Streamer {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.Streamers[streamerID]
}

func (r *Room) RemoveStreamer(streamerID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if streamer, exists := r.Streamers[streamerID]; exists {
		streamer.Cleanup()
		delete(r.Streamers, streamerID)
	}
}

func (s *Streamer) Cleanup() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, pc := range s.PeerConnections {
		if pc.PeerConnection != nil {
			pc.PeerConnection.Close()
		}
		if pc.WebSocket != nil {
			pc.WebSocket.Close()
		}
	}

	for _, pc := range s.ViewerConnections {
		if pc.PeerConnection != nil {
			pc.PeerConnection.Close()
		}
		if pc.WebSocket != nil {
			pc.WebSocket.Close()
		}
	}
	s.PeerConnections = nil
	s.ViewerConnections = nil
	s.TrackLocals = make(map[string]*webrtc.TrackLocalStaticRTP)
	s.VideoTracks = make(map[string]*webrtc.TrackLocalStaticRTP)
	s.AudioTracks = make(map[string]*webrtc.TrackLocalStaticRTP)
}

func (s *Streamer) ToggleCamera() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.CameraEnabled = !s.CameraEnabled

	if s.CameraEnabled {
		for trackID, track := range s.VideoTracks {
			s.TrackLocals[trackID] = track
		}
	} else {
		for trackID := range s.VideoTracks {
			delete(s.TrackLocals, trackID)
		}
	}

	s.signalAllConnections()
	return nil
}

func (s *Streamer) ToggleMicrophone() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.MicrophoneEnabled = !s.MicrophoneEnabled

	if s.MicrophoneEnabled {
		for trackID, track := range s.AudioTracks {
			s.TrackLocals[trackID] = track
		}
	} else {
		for trackID := range s.AudioTracks {
			delete(s.TrackLocals, trackID)
		}
	}

	s.signalAllConnections()
	return nil
}

func (s *Streamer) signalAllConnections() {
	go SignalPeerConnectionsStreamer(s)
	go SignalViewerConnectionsStreamer(s)
}

func (r *Room) UpdateLastActivity() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.LastActivity = time.Now()
}

func (r *Room) IsInactive(duration time.Duration) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return time.Since(r.LastActivity) > duration
}

func CleanupInactiveRooms(inactivityDuration time.Duration) {
	roomsLock.Lock()
	defer roomsLock.Unlock()

	for roomID, room := range rooms {
		if room.IsInactive(inactivityDuration) {
			log.Infof("Cleaning up inactive room: %s", roomID)

			// Cleanup all streamers in the room
			for streamerID := range room.Streamers {
				room.RemoveStreamer(streamerID)
			}

			// Remove from Firebase
			if err := livedatabase.DeleteRoom(roomID); err != nil {
				log.Errorf("Failed to delete room from Firebase: %v", err)
			}

			// Remove from memory
			delete(rooms, roomID)
		}
	}
}
