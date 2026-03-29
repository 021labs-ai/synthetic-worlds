package http

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Server struct {
	app  *fiber.App
	port int
}

func NewServer(port int, readTimeout, writeTimeout time.Duration) *Server {
	app := fiber.New(fiber.Config{
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		BodyLimit:    10 * 1024 * 1024, // 10MB
	})

	app.Use(recover.New())
	app.Use(cors.New())

	return &Server{app: app, port: port}
}

func (s *Server) App() *fiber.App {
	return s.app
}

func (s *Server) Start() error {
	return s.app.Listen(fmt.Sprintf(":%d", s.port))
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}
