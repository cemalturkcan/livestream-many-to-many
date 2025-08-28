package routes

import (
	"livestream-many-to-many/src/routes/api"

	"github.com/gofiber/fiber/v2"
)

func Register(s *fiber.App) {
	api.Register(s)
}
