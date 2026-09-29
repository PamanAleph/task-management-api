package response

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
)

// Envelope is the single reusable response shape for the whole API.
// Every success and error response is emitted through the helpers below
// so the shape never drifts between handlers.
type Envelope struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Meta      interface{} `json:"meta,omitempty"`
	Code      string      `json:"code,omitempty"`
	Message   string      `json:"message,omitempty"`
	Timestamp string      `json:"timestamp"`
}

type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

func OK(c *fiber.Ctx, httpStatus int, data interface{}) error {
	return c.Status(httpStatus).JSON(Envelope{
		Status:    "success",
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func OKPaginated(c *fiber.Ctx, data interface{}, meta Pagination) error {
	return c.Status(200).JSON(Envelope{
		Status:    "success",
		Data:      data,
		Meta:      meta,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// Error renders an AppError as the standard structured error envelope.
// It never includes a stack trace or internal error detail in the body.
func Error(c *fiber.Ctx, err *apperror.AppError) error {
	return c.Status(err.HTTPStatus).JSON(Envelope{
		Status:    "error",
		Code:      err.Code,
		Message:   err.Message,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}
