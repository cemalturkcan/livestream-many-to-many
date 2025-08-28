package api

import (
	"livestream-many-to-many/src/routes/websocket"

	"github.com/gofiber/fiber/v2"
)

func Register(router fiber.Router) {
	group := router.Group("/api")
	websocket.Register(group)
}
