package websocket

import (
	"github.com/pion/webrtc/v4"
)

func (s *Streamer) AddTrack(t *webrtc.TrackRemote) (*webrtc.TrackLocalStaticRTP, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	trackLocal, err := webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())
	if err != nil {
		return nil, err
	}

	if t.Kind() == webrtc.RTPCodecTypeVideo {
		s.VideoTracks[t.ID()] = trackLocal
		if s.CameraEnabled {
			s.TrackLocals[t.ID()] = trackLocal
		}
	} else if t.Kind() == webrtc.RTPCodecTypeAudio {
		s.AudioTracks[t.ID()] = trackLocal
		if s.MicrophoneEnabled {
			s.TrackLocals[t.ID()] = trackLocal
		}
	}

	return trackLocal, nil
}

func (s *Streamer) RemoveTrack(trackID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.TrackLocals, trackID)
	delete(s.VideoTracks, trackID)
	delete(s.AudioTracks, trackID)
}

func (s *Streamer) GetActiveTracks() map[string]*webrtc.TrackLocalStaticRTP {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	tracks := make(map[string]*webrtc.TrackLocalStaticRTP)
	for id, track := range s.TrackLocals {
		tracks[id] = track
	}
	return tracks
}

func (s *Streamer) AddPeerConnection(pc *webrtc.PeerConnection, ws *ThreadSafeWriter) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.PeerConnections = append(s.PeerConnections, &PeerConnectionState{
		PeerConnection: pc,
		WebSocket:      ws,
	})
}

func (s *Streamer) AddViewerConnection(pc *webrtc.PeerConnection, ws *ThreadSafeWriter) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.ViewerConnections = append(s.ViewerConnections, &PeerConnectionState{
		PeerConnection: pc,
		WebSocket:      ws,
	})
}

func (s *Streamer) RemoveClosedConnections() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	activePeers := make([]*PeerConnectionState, 0)
	for _, pc := range s.PeerConnections {
		if pc.PeerConnection.ConnectionState() != webrtc.PeerConnectionStateClosed {
			activePeers = append(activePeers, pc)
		}
	}
	s.PeerConnections = activePeers

	activeViewers := make([]*PeerConnectionState, 0)
	for _, pc := range s.ViewerConnections {
		if pc.PeerConnection.ConnectionState() != webrtc.PeerConnectionStateClosed {
			activeViewers = append(activeViewers, pc)
		}
	}
	s.ViewerConnections = activeViewers
}
