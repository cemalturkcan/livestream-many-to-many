package websocket

import (
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2/log"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

const (
	maxSyncAttempts = 25
	syncRetryDelay  = 3 * time.Second
)

func SignalPeerConnectionsStreamer(s *Streamer) {
	s.mutex.Lock()
	connections := make([]*PeerConnectionState, len(s.PeerConnections))
	copy(connections, s.PeerConnections)

	tracks := make(map[string]*webrtc.TrackLocalStaticRTP)
	for id, track := range s.TrackLocals {
		tracks[id] = track
	}
	s.mutex.Unlock()

	go signalConnections(connections, tracks, "streamer peer connections")
}

func SignalViewerConnectionsStreamer(s *Streamer) {
	s.mutex.Lock()
	connections := make([]*PeerConnectionState, len(s.ViewerConnections))
	copy(connections, s.ViewerConnections)

	tracks := make(map[string]*webrtc.TrackLocalStaticRTP)
	for id, track := range s.TrackLocals {
		tracks[id] = track
	}
	s.mutex.Unlock()

	go signalConnections(connections, tracks, "viewer connections")
}

func signalConnections(connections []*PeerConnectionState, tracks map[string]*webrtc.TrackLocalStaticRTP, logPrefix string) {
	if len(connections) == 0 {
		return
	}

	for syncAttempt := 0; syncAttempt < maxSyncAttempts; syncAttempt++ {
		if !attemptSync(connections, tracks) {
			break
		}

		if syncAttempt == maxSyncAttempts-1 {
			log.Warnf("Max sync attempts reached for %s, retrying in %v", logPrefix, syncRetryDelay)
			time.Sleep(syncRetryDelay)
			go signalConnections(connections, tracks, logPrefix)
			return
		}
	}

	dispatchKeyFrames(connections)
}

func attemptSync(connections []*PeerConnectionState, tracks map[string]*webrtc.TrackLocalStaticRTP) bool {
	for i := len(connections) - 1; i >= 0; i-- {
		pc := connections[i]

		if pc.PeerConnection.ConnectionState() == webrtc.PeerConnectionStateClosed {
			connections = append(connections[:i], connections[i+1:]...)
			continue
		}

		existingSenders := make(map[string]bool)
		for _, sender := range pc.PeerConnection.GetSenders() {
			if sender.Track() == nil {
				continue
			}

			trackID := sender.Track().ID()
			existingSenders[trackID] = true

			if _, exists := tracks[trackID]; !exists {
				if err := pc.PeerConnection.RemoveTrack(sender); err != nil {
					log.Errorf("Failed to remove track %s: %v", trackID, err)
					return true
				}
			}
		}

		for trackID, track := range tracks {
			if !existingSenders[trackID] {
				if _, err := pc.PeerConnection.AddTrack(track); err != nil {
					log.Errorf("Failed to add track %s: %v", trackID, err)
					return true
				}
			}
		}

		if err := createAndSendOffer(pc); err != nil {
			log.Errorf("Failed to create and send offer: %v", err)
			return true
		}
	}

	return false
}

func createAndSendOffer(pc *PeerConnectionState) error {
	offer, err := pc.PeerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err = pc.PeerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	offerString, err := json.Marshal(offer)
	if err != nil {
		return err
	}

	return pc.WebSocket.WriteJSON(&Message{
		Event: "offer",
		Data:  string(offerString),
	})
}

func dispatchKeyFrames(connections []*PeerConnectionState) {
	for _, pc := range connections {
		for _, receiver := range pc.PeerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			_ = pc.PeerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}
	}
}

func dispatchKeyFrameAllRooms() {
	roomsLock.RLock()
	defer roomsLock.RUnlock()

	for _, room := range rooms {
		room.mutex.RLock()
		for _, streamer := range room.Streamers {
			go func(s *Streamer) {
				s.mutex.RLock()
				defer s.mutex.RUnlock()

				dispatchKeyFrames(s.PeerConnections)
				dispatchKeyFrames(s.ViewerConnections)
			}(streamer)
		}
		room.mutex.RUnlock()
	}
}
