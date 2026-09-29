package task

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
	"github.com/aliefbuscode/task-management-api/internal/middleware"
	"github.com/aliefbuscode/task-management-api/internal/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/", h.Create)
	router.Get("/", h.List)
	router.Get("/:id", h.Get)
	router.Put("/:id", h.Update)
	router.Delete("/:id", h.Delete)
	router.Post("/:id/assign", h.Assign)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.ErrValidation("invalid request body")
	}

	t, appErr := h.service.Create(c.Context(), middleware.TeamID(c), middleware.UserID(c), req)
	if appErr != nil {
		return appErr
	}
	middleware.AddField(c, "task_id", t.ID)
	return response.OK(c, fiber.StatusCreated, t)
}

func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	filter := ListFilter{
		OwnerID: middleware.UserID(c),
		Status:  c.Query("status"),
		Search:  c.Query("search"),
		Page:    page,
		Limit:   limit,
	}

	res, appErr := h.service.List(c.Context(), filter)
	if appErr != nil {
		return appErr
	}

	items := res.Items
	if items == nil {
		items = []Task{}
	}

	totalPages := int((res.TotalItems + int64(res.Limit) - 1) / int64(res.Limit))
	if totalPages < 1 {
		totalPages = 1
	}

	return response.OKPaginated(c, items, response.Pagination{
		Page:       res.Page,
		Limit:      res.Limit,
		TotalItems: res.TotalItems,
		TotalPages: totalPages,
	})
}

func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apperror.ErrValidation("invalid task id")
	}

	t, appErr := h.service.Get(c.Context(), id, middleware.UserID(c))
	if appErr != nil {
		return appErr
	}
	return response.OK(c, fiber.StatusOK, t)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apperror.ErrValidation("invalid task id")
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.ErrValidation("invalid request body")
	}

	t, appErr := h.service.Update(c.Context(), id, middleware.UserID(c), req)
	if appErr != nil {
		return appErr
	}
	return response.OK(c, fiber.StatusOK, t)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apperror.ErrValidation("invalid task id")
	}

	if appErr := h.service.Delete(c.Context(), id, middleware.UserID(c)); appErr != nil {
		return appErr
	}
	return response.OK(c, fiber.StatusOK, fiber.Map{"deleted": true})
}

func (h *Handler) Assign(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apperror.ErrValidation("invalid task id")
	}

	var req AssignRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.ErrValidation("invalid request body")
	}
	if req.AssigneeID == 0 {
		return apperror.ErrValidation("assignee_id is required")
	}

	t, appErr := h.service.Assign(c.Context(), id, middleware.UserID(c), req.AssigneeID, middleware.UserID(c))
	if appErr != nil {
		return appErr
	}
	middleware.AddField(c, "task_id", t.ID)
	middleware.AddField(c, "assignee_id", req.AssigneeID)
	return response.OK(c, fiber.StatusOK, t)
}
