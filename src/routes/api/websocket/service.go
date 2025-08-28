package websocket

import (
	"livestream/app/firebase/livedatabase"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2/log"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func HandleWebRTCConnectionRoomStreamer(conn *websocket.Conn, room *Room, streamerID string) {
	c := &ThreadSafeWriter{conn, sync.Mutex{}}
	defer c.Close()

	// Update room activity when streamer connects
	room.UpdateLastActivity()

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			{URLs: []string{"stun:stun1.l.google.com:19302"}},
		},
	})
	if err != nil {
		log.Errorf("Failed to create PeerConnection: %v", err)
		return
	}
	defer peerConnection.Close()

	if err := setupTransceivers(peerConnection); err != nil {
		log.Errorf("Failed to setup transceivers: %v", err)
		return
	}

	streamer := room.GetStreamer(streamerID)
	if streamer == nil {
		return
	}

	streamer.AddPeerConnection(peerConnection, c)
	setupPeerConnectionHandlers(peerConnection, c, streamer, true)

	user, err := GetUserById(streamerID)
	if err != nil {
		log.Errorf("Failed to get user %s: %v", streamerID, err)
		return
	}

	if err := livedatabase.AddStream(room.ID, streamerID, user.Username, user.Avatar); err != nil {
		log.Errorf("Failed to add stream to room: %v", err)
		return
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		SignalPeerConnectionsStreamer(streamer)
	}()
	handleWebSocketMessages(c, peerConnection, room)
}

func HandleWebRTCConnectionRoomSViewer(conn *websocket.Conn, room *Room, streamerID string) {
	c := &ThreadSafeWriter{conn, sync.Mutex{}}
	defer c.Close()

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			{URLs: []string{"stun:stun1.l.google.com:19302"}},
		},
	})
	if err != nil {
		log.Errorf("Failed to create PeerConnection: %v", err)
		return
	}
	defer peerConnection.Close()

	if err := setupTransceivers(peerConnection); err != nil {
		log.Errorf("Failed to setup transceivers: %v", err)
		return
	}

	streamer := room.GetStreamer(streamerID)
	if streamer == nil {
		return
	}

	streamer.AddViewerConnection(peerConnection, c)
	setupPeerConnectionHandlers(peerConnection, c, streamer, false)

	go func() {
		time.Sleep(100 * time.Millisecond)
		SignalViewerConnectionsStreamer(streamer)
	}()

	handleWebSocketMessages(c, peerConnection, room)
}

func setupTransceivers(pc *webrtc.PeerConnection) error {
	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := pc.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			return err
		}
	}
	return nil
}

func setupPeerConnectionHandlers(pc *webrtc.PeerConnection, c *ThreadSafeWriter, streamer *Streamer, isPublisher bool) {
	pc.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Errorf("Failed to marshal candidate to json: %v", err)
			return
		}

		if writeErr := c.WriteJSON(&Message{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Errorf("Failed to write JSON: %v", writeErr)
		}
	})

	pc.OnConnectionStateChange(func(p webrtc.PeerConnectionState) {
		switch p {
		case webrtc.PeerConnectionStateFailed:
			if err := pc.Close(); err != nil {
				log.Errorf("Failed to close PeerConnection: %v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			streamer.RemoveClosedConnections()
			if isPublisher {
				if err := livedatabase.RemoveStream(streamer.Room.ID, streamer.ID); err != nil {
					log.Errorf("Failed to remove stream from Firebase: %v", err)
				}
				SignalPeerConnectionsStreamer(streamer)
			} else {
				SignalViewerConnectionsStreamer(streamer)
			}
		default:
		}
	})

	if isPublisher {
		pc.OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			trackLocal, err := streamer.AddTrack(t)
			if err != nil {
				log.Errorf("Failed to add track: %v", err)
				return
			}
			defer streamer.RemoveTrack(t.ID())

			SignalViewerConnectionsStreamer(streamer)

			buf := make([]byte, 1500)
			rtpPkt := &rtp.Packet{}

			for {
				i, _, err := t.Read(buf)
				if err != nil {
					return
				}

				if err = rtpPkt.Unmarshal(buf[:i]); err != nil {
					log.Errorf("Failed to unmarshal incoming RTP packet: %v", err)
					return
				}

				rtpPkt.Extension = false
				rtpPkt.Extensions = nil

				if err = trackLocal.WriteRTP(rtpPkt); err != nil {
					return
				}
			}
		})
	}

	pc.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
	})
}

func handleWebSocketMessages(c *ThreadSafeWriter, pc *webrtc.PeerConnection, room *Room) {
	message := &Message{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Errorf("Failed to read message: %v", err)
			return
		}

		room.UpdateLastActivity()

		if err := json.Unmarshal(raw, &message); err != nil {
			log.Errorf("Failed to unmarshal json to message: %v", err)
			return
		}

		switch message.Event {
		case "candidate":
			if err := handleICECandidate(pc, message.Data); err != nil {
				log.Errorf("Failed to handle ICE candidate: %v", err)
				return
			}
		case "answer":
			if err := handleAnswer(pc, message.Data); err != nil {
				log.Errorf("Failed to handle answer: %v", err)
				return
			}
		default:
			log.Errorf("Unknown message: %+v", message)
		}
	}
}

func handleICECandidate(pc *webrtc.PeerConnection, data string) error {
	candidate := webrtc.ICECandidateInit{}
	if err := json.Unmarshal([]byte(data), &candidate); err != nil {
		return err
	}
	return pc.AddICECandidate(candidate)
}

func handleAnswer(pc *webrtc.PeerConnection, data string) error {
	answer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(data), &answer); err != nil {
		return err
	}
	return pc.SetRemoteDescription(answer)
}

func pauseStream(room *Room) {
	room.mutex.RLock()
	if room.Paused {
		room.mutex.RUnlock()
		return
	}

	room.Paused = true
	for _, streamer := range room.Streamers {
		streamer.mutex.Lock()
		if streamer.CameraEnabled || streamer.MicrophoneEnabled {
			streamer.CameraEnabled = false
			streamer.MicrophoneEnabled = false
			for trackID := range streamer.VideoTracks {
				delete(streamer.TrackLocals, trackID)
			}
			for trackID := range streamer.AudioTracks {
				delete(streamer.TrackLocals, trackID)
			}
		}
		streamer.mutex.Unlock()
		go streamer.signalAllConnections()
	}
	room.mutex.RUnlock()
}

func resumeStream(room *Room) {
	room.mutex.RLock()
	if !room.Paused {
		room.mutex.RUnlock()
		return
	}

	room.Paused = false

	for _, streamer := range room.Streamers {
		streamer.mutex.Lock()
		if !streamer.CameraEnabled || !streamer.MicrophoneEnabled {
			streamer.CameraEnabled = true
			streamer.MicrophoneEnabled = true

			for trackID, track := range streamer.VideoTracks {
				streamer.TrackLocals[trackID] = track
			}
			for trackID, track := range streamer.AudioTracks {
				streamer.TrackLocals[trackID] = track
			}
		}
		streamer.mutex.Unlock()
		go streamer.signalAllConnections()
	}

	room.mutex.RUnlock()
}
