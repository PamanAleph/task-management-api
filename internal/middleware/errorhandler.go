package middleware

import (
	"errors"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
	"github.com/aliefbuscode/task-management-api/internal/response"
)

// ErrorHandler is the single place that turns any error or panic raised
// anywhere downstream into the structured JSON error envelope. It never
// lets a stack trace or raw internal error message reach the client;
// those details only ever go to the log.
func ErrorHandler(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Str("request_id", RequestID(c)).
					Interface("panic", r).
					Str("stack", string(debug.Stack())).
					Msg("panic recovered")
				_ = response.Error(c, apperror.New(fiber.StatusInternalServerError, "INTERNAL_ERROR", "internal server error"))
				err = nil
			}
		}()

		nextErr := c.Next()
		if nextErr == nil {
			return nil
		}

		appErr := ToAppError(nextErr)
		if appErr.Code == "INTERNAL_ERROR" {
			log.Error().
				Str("request_id", RequestID(c)).
				Err(nextErr).
				Msg("unhandled error")
		}
		return response.Error(c, appErr)
	}
}

// ToAppError normalizes any error into the structured AppError shape.
// Shared by ErrorHandler and Idempotency, which — when the wrapped
// handler fails — must render and cache the exact same error response
// the global error handler would have produced.
func ToAppError(err error) *apperror.AppError {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code := "HTTP_ERROR"
		if fiberErr.Code == fiber.StatusNotFound {
			code = "NOT_FOUND"
		}
		return apperror.New(fiberErr.Code, code, fiberErr.Message)
	}
	return apperror.New(fiber.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}
