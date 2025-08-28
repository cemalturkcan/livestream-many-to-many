package websocket

import (
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func livestreamSocket(c *websocket.Conn) {
	roomID := c.Params("roomID")
	log.Info(roomID)
	room := getOrCreateRoom(roomID)
	log.Infof("WebRTC connection for room: %s, room: %+v", roomID, room)
	HandleWebRTCConnectionRoom(c, room)
}

func Register(router fiber.Router) {
	group := router.Group("/websocket")
	group.Get("/:roomID", websocket.New(livestreamSocket))
	go func() {
		for range time.NewTicker(time.Second * 3).C {
			dispatchKeyFrameAllRooms()
		}
	}()
}
