package middlewares

import (
	"livestream-many-to-many/app/config"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func RegisterMiddlewares(s *fiber.App) {
	s.Use(recover.New())
	s.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173",
		AllowCredentials: true,
	}))
	if config.LoggerEnabled {
		s.Use(logger.New())
	}
	s.Use(compress.New())

	s.Use("/api/websocket", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
}

func RegisterFinalMiddlewares(s *fiber.App) {
	s.Static("/", "./public")
	s.Use(func(c *fiber.Ctx) error {
		return c.SendFile("./public/index.html")
	})

}
