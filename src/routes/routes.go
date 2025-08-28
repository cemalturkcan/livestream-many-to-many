package routes

import (
	"github.com/gofiber/fiber/v2"
	"livestream/src/routes/api"
)

func Register(s *fiber.App) {
	api.Register(s)
}
