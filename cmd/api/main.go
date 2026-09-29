package main

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
	"github.com/aliefbuscode/task-management-api/internal/auth"
	"github.com/aliefbuscode/task-management-api/internal/config"
	"github.com/aliefbuscode/task-management-api/internal/db"
	"github.com/aliefbuscode/task-management-api/internal/idempotency"
	"github.com/aliefbuscode/task-management-api/internal/logger"
	"github.com/aliefbuscode/task-management-api/internal/middleware"
	"github.com/aliefbuscode/task-management-api/internal/task"
)

func main() {
	log := logger.New()
	cfg := config.Load()

	sqlDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer sqlDB.Close()

	if err := db.Migrate(sqlDB, "migrations"); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}

	authRepo := auth.NewPostgresRepository(sqlDB)
	authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpiryMinutes)
	authHandler := auth.NewHandler(authService)

	taskRepo := task.NewPostgresRepository(sqlDB)
	notifier := task.NewLogNotifier(log)
	taskService := task.NewService(taskRepo, notifier)
	taskHandler := task.NewHandler(taskService)

	idempotencyRepo := idempotency.NewPostgresRepository(sqlDB)
	idempotencyTTL := time.Duration(cfg.IdempotencyTTLHours) * time.Hour

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(middleware.RequestLogger(log))
	app.Use(middleware.ErrorHandler(log))

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	authGroup := app.Group("/auth")
	authHandler.RegisterRoutes(authGroup)

	tasksGroup := app.Group("/tasks", middleware.JWTAuth(cfg.JWTSecret))
	tasksGroup.Post("/", middleware.Idempotency(idempotencyRepo, idempotencyTTL), taskHandler.Create)
	tasksGroup.Get("/", taskHandler.List)
	tasksGroup.Get("/:id", taskHandler.Get)
	tasksGroup.Put("/:id", taskHandler.Update)
	tasksGroup.Delete("/:id", taskHandler.Delete)
	tasksGroup.Post("/:id/assign", taskHandler.Assign)

	app.Use(func(c *fiber.Ctx) error {
		return apperror.ErrNotFound("route not found")
	})

	log.Info().Str("port", cfg.Port).Msg("starting server")
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
