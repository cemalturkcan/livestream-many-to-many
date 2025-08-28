package api

import (
	"livestream/src/routes/api/websocket"

	"github.com/gofiber/fiber/v2"
)

func Register(router fiber.Router) {
	group := router.Group("/room")
	websocket.Register(group)
}
