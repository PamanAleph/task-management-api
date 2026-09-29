package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
	"github.com/aliefbuscode/task-management-api/internal/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/register", h.Register)
	router.Post("/login", h.Login)
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.ErrValidation("invalid request body")
	}
	if req.Name == "" || req.Email == "" || len(req.Password) < 8 {
		return apperror.ErrValidation("name, email are required and password must be at least 8 characters")
	}

	res, appErr := h.service.Register(c.Context(), req)
	if appErr != nil {
		return appErr
	}
	return response.OK(c, fiber.StatusCreated, res)
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.ErrValidation("invalid request body")
	}
	if req.Email == "" || req.Password == "" {
		return apperror.ErrValidation("email and password are required")
	}

	res, appErr := h.service.Login(c.Context(), req)
	if appErr != nil {
		return appErr
	}
	return response.OK(c, fiber.StatusOK, res)
}
