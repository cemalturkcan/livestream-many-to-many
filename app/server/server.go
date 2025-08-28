package server

import (
	"livestream-many-to-many/app/config"
	"livestream-many-to-many/app/exitcode"
	"livestream-many-to-many/app/middlewares"
	"livestream-many-to-many/app/rest"
	"livestream-many-to-many/src/routes"
	"os"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

type FiberServer struct {
	*fiber.App
}

func New() *FiberServer {
	server := &FiberServer{
		App: fiber.New(fiber.Config{
			AppName:      config.AppName,
			ErrorHandler: ErrorHandler,
			JSONDecoder:  json.Unmarshal,
			JSONEncoder:  json.Marshal,
		}),
	}
	middlewares.RegisterMiddlewares(server.App)
	routes.Register(server.App)
	middlewares.RegisterFinalMiddlewares(server.App)

	err := server.Listen(config.Port)
	if err != nil {
		os.Exit(exitcode.ServerStartError)
	}
	return server
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	log.Error("Error: ", err)
	code, message := rest.Error(err)
	return rest.ErrorRes(c, code, message)
}
