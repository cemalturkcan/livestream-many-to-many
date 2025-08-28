package websocket

import (
	"sync"

	"github.com/goccy/go-json"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2/log"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func HandleWebRTCConnection(conn *websocket.Conn) {
	c := &ThreadSafeWriter{conn, sync.Mutex{}}
	defer c.Close()

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Errorf("Failed to create PeerConnection: %v", err)
		return
	}
	defer peerConnection.Close()

	if err := setupTransceivers(peerConnection); err != nil {
		log.Errorf("Failed to setup transceivers: %v", err)
		return
	}

	AddPeerConnection(peerConnection, c)

	setupPeerConnectionHandlers(peerConnection, c)
	SignalPeerConnections()

	handleWebSocketMessages(c, peerConnection)
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

func setupPeerConnectionHandlers(pc *webrtc.PeerConnection, c *ThreadSafeWriter) {
	pc.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Errorf("Failed to marshal candidate to json: %v", err)
			return
		}

		if writeErr := c.WriteJSON(&WebSocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Errorf("Failed to write JSON: %v", writeErr)
		}
	})

	pc.OnConnectionStateChange(func(p webrtc.PeerConnectionState) {
		log.Infof("Connection state change: %s", p)

		switch p {
		case webrtc.PeerConnectionStateFailed:
			if err := pc.Close(); err != nil {
				log.Errorf("Failed to close PeerConnection: %v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			SignalPeerConnections()
		default:
		}
	})

	pc.OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		log.Infof("Got remote track: Kind=%s, ID=%s, PayloadType=%d", t.Kind(), t.ID(), t.PayloadType())

		trackLocal := AddTrack(t)
		defer RemoveTrack(trackLocal)

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

	pc.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		log.Infof("ICE connection state changed: %s", is)
	})
}

func handleWebSocketMessages(c *ThreadSafeWriter, pc *webrtc.PeerConnection) {
	message := &WebSocketMessage{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Errorf("Failed to read message: %v", err)
			return
		}

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

func HandleWebRTCConnectionRoom(conn *websocket.Conn, room *Room) {
	c := &ThreadSafeWriter{conn, sync.Mutex{}}
	defer c.Close()

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Errorf("Failed to create PeerConnection: %v", err)
		return
	}
	defer peerConnection.Close()

	if err := setupTransceivers(peerConnection); err != nil {
		log.Errorf("Failed to setup transceivers: %v", err)
		return
	}

	AddPeerConnectionRoom(room, peerConnection, c)

	setupPeerConnectionHandlersRoom(peerConnection, c, room)
	SignalPeerConnectionsRoom(room)

	handleWebSocketMessages(c, peerConnection)
}

func setupPeerConnectionHandlersRoom(pc *webrtc.PeerConnection, c *ThreadSafeWriter, room *Room) {
	pc.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Errorf("Failed to marshal candidate to json: %v", err)
			return
		}

		if writeErr := c.WriteJSON(&WebSocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Errorf("Failed to write JSON: %v", writeErr)
		}
	})

	pc.OnConnectionStateChange(func(p webrtc.PeerConnectionState) {
		log.Infof("Connection state change: %s", p)

		switch p {
		case webrtc.PeerConnectionStateFailed:
			if err := pc.Close(); err != nil {
				log.Errorf("Failed to close PeerConnection: %v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			SignalPeerConnectionsRoom(room)
		default:
		}
	})

	pc.OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		log.Infof("Got remote track: Kind=%s, ID=%s, PayloadType=%d", t.Kind(), t.ID(), t.PayloadType())

		trackLocal := AddTrackRoom(room, t)
		defer RemoveTrackRoom(room, trackLocal)

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

	pc.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		log.Infof("ICE connection state changed: %s", is)
	})
}
