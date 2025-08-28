package websocket

import (
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2/log"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

var (
	listLock        sync.RWMutex
	peerConnections []PeerConnectionState
	trackLocals     map[string]*webrtc.TrackLocalStaticRTP
)

func init() {
	trackLocals = map[string]*webrtc.TrackLocalStaticRTP{}
}

func AddPeerConnection(pc *webrtc.PeerConnection, ws *ThreadSafeWriter) {
	listLock.Lock()
	peerConnections = append(peerConnections, PeerConnectionState{pc, ws})
	listLock.Unlock()
}

func AddTrack(t *webrtc.TrackRemote) *webrtc.TrackLocalStaticRTP {
	listLock.Lock()
	defer func() {
		listLock.Unlock()
		SignalPeerConnections()
	}()

	trackLocal, err := webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())
	if err != nil {
		panic(err)
	}

	trackLocals[t.ID()] = trackLocal
	return trackLocal
}

func RemoveTrack(t *webrtc.TrackLocalStaticRTP) {
	listLock.Lock()
	defer func() {
		listLock.Unlock()
		SignalPeerConnections()
	}()

	delete(trackLocals, t.ID())
}

func attemptSync() (tryAgain bool) {
	for i := range peerConnections {
		if peerConnections[i].PeerConnection.ConnectionState() == webrtc.PeerConnectionStateClosed {
			peerConnections = append(peerConnections[:i], peerConnections[i+1:]...)
			return true
		}

		existingSenders := map[string]bool{}

		for _, sender := range peerConnections[i].PeerConnection.GetSenders() {
			if sender.Track() == nil {
				continue
			}

			existingSenders[sender.Track().ID()] = true

			if _, ok := trackLocals[sender.Track().ID()]; !ok {
				if err := peerConnections[i].PeerConnection.RemoveTrack(sender); err != nil {
					return true
				}
			}
		}

		for _, receiver := range peerConnections[i].PeerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			existingSenders[receiver.Track().ID()] = true
		}

		for trackID := range trackLocals {
			if _, ok := existingSenders[trackID]; !ok {
				if _, err := peerConnections[i].PeerConnection.AddTrack(trackLocals[trackID]); err != nil {
					return true
				}
			}
		}

		offer, err := peerConnections[i].PeerConnection.CreateOffer(nil)
		if err != nil {
			return true
		}

		if err = peerConnections[i].PeerConnection.SetLocalDescription(offer); err != nil {
			return true
		}

		offerString, err := json.Marshal(offer)
		if err != nil {
			log.Errorf("Failed to marshal offer to json: %v", err)
			return true
		}

		if err = peerConnections[i].WebSocket.WriteJSON(&WebSocketMessage{
			Event: "offer",
			Data:  string(offerString),
		}); err != nil {
			return true
		}
	}

	return tryAgain
}

func SignalPeerConnections() {
	listLock.Lock()
	defer func() {
		listLock.Unlock()
		dispatchKeyFrame()
	}()

	for syncAttempt := 0; ; syncAttempt++ {
		if syncAttempt == 25 {
			go func() {
				time.Sleep(time.Second * 3)
				SignalPeerConnections()
			}()
			return
		}

		if !attemptSync() {
			break
		}
	}
}

func dispatchKeyFrame() {
	listLock.Lock()
	defer listLock.Unlock()

	for i := range peerConnections {
		for _, receiver := range peerConnections[i].PeerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			_ = peerConnections[i].PeerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}
	}
}

// Room yapısı ve oda yönetimi

type Room struct {
	PeerConnections []PeerConnectionState
	TrackLocals     map[string]*webrtc.TrackLocalStaticRTP
	ListLock        sync.RWMutex
}

var rooms = make(map[string]*Room)
var roomsLock sync.RWMutex

func getOrCreateRoom(roomID string) *Room {
	roomsLock.Lock()
	defer roomsLock.Unlock()
	room, exists := rooms[roomID]
	if !exists {
		room = &Room{
			TrackLocals: make(map[string]*webrtc.TrackLocalStaticRTP),
		}
		rooms[roomID] = room
	}
	return room
}

// Odaya özel peer/track işlemleri
func AddPeerConnectionRoom(room *Room, pc *webrtc.PeerConnection, ws *ThreadSafeWriter) {
	room.ListLock.Lock()
	room.PeerConnections = append(room.PeerConnections, PeerConnectionState{pc, ws})
	room.ListLock.Unlock()
}

func AddTrackRoom(room *Room, t *webrtc.TrackRemote) *webrtc.TrackLocalStaticRTP {
	room.ListLock.Lock()
	defer func() {
		room.ListLock.Unlock()
		SignalPeerConnectionsRoom(room)
	}()

	trackLocal, err := webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())
	if err != nil {
		panic(err)
	}

	room.TrackLocals[t.ID()] = trackLocal
	return trackLocal
}

func RemoveTrackRoom(room *Room, t *webrtc.TrackLocalStaticRTP) {
	room.ListLock.Lock()
	defer func() {
		room.ListLock.Unlock()
		SignalPeerConnectionsRoom(room)
	}()

	delete(room.TrackLocals, t.ID())
}

func attemptSyncRoom(room *Room) (tryAgain bool) {
	for i := range room.PeerConnections {
		if room.PeerConnections[i].PeerConnection.ConnectionState() == webrtc.PeerConnectionStateClosed {
			room.PeerConnections = append(room.PeerConnections[:i], room.PeerConnections[i+1:]...)
			return true
		}

		existingSenders := map[string]bool{}

		for _, sender := range room.PeerConnections[i].PeerConnection.GetSenders() {
			if sender.Track() == nil {
				continue
			}

			existingSenders[sender.Track().ID()] = true

			if _, ok := room.TrackLocals[sender.Track().ID()]; !ok {
				if err := room.PeerConnections[i].PeerConnection.RemoveTrack(sender); err != nil {
					return true
				}
			}
		}

		for _, receiver := range room.PeerConnections[i].PeerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			existingSenders[receiver.Track().ID()] = true
		}

		for trackID := range room.TrackLocals {
			if _, ok := existingSenders[trackID]; !ok {
				if _, err := room.PeerConnections[i].PeerConnection.AddTrack(room.TrackLocals[trackID]); err != nil {
					return true
				}
			}
		}

		offer, err := room.PeerConnections[i].PeerConnection.CreateOffer(nil)
		if err != nil {
			return true
		}

		if err = room.PeerConnections[i].PeerConnection.SetLocalDescription(offer); err != nil {
			return true
		}

		offerString, err := json.Marshal(offer)
		if err != nil {
			log.Errorf("Failed to marshal offer to json: %v", err)
			return true
		}

		if err = room.PeerConnections[i].WebSocket.WriteJSON(&WebSocketMessage{
			Event: "offer",
			Data:  string(offerString),
		}); err != nil {
			return true
		}
	}

	return tryAgain
}

func SignalPeerConnectionsRoom(room *Room) {
	room.ListLock.Lock()
	defer func() {
		room.ListLock.Unlock()
		dispatchKeyFrameRoom(room)
	}()

	for syncAttempt := 0; ; syncAttempt++ {
		if syncAttempt == 25 {
			go func() {
				time.Sleep(time.Second * 3)
				SignalPeerConnectionsRoom(room)
			}()
			return
		}

		if !attemptSyncRoom(room) {
			break
		}
	}
}

func dispatchKeyFrameRoom(room *Room) {
	room.ListLock.Lock()
	defer room.ListLock.Unlock()

	for i := range room.PeerConnections {
		for _, receiver := range room.PeerConnections[i].PeerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			_ = room.PeerConnections[i].PeerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}
	}
}

// Tüm odalara keyframe dispatch
func dispatchKeyFrameAllRooms() {
	roomsLock.RLock()
	defer roomsLock.RUnlock()
	for _, room := range rooms {
		go dispatchKeyFrameRoom(room)
	}
}
