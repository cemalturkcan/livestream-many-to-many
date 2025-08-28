package rest

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

type Meta struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Page[T any] struct {
	Size    int   `json:"size"`
	Total   int   `json:"total"`
	Content *[]*T `json:"content"`
}

func jsonResponse(c *fiber.Ctx, data any, code string, message string) error {
	return c.JSON(&fiber.Map{
		"data": data,
		"meta": Meta{
			Code:    code,
			Message: message,
		},
	})
}

// wrap data with meta error
func ErrorRes(c *fiber.Ctx, code string, message string) error {
	log.Error(message)
	log.Error(code)
	log.Error(c.Path())
	log.Error(c.Method())
	return jsonResponse(c, nil, code, message)
}
