package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
	"github.com/aliefbuscode/task-management-api/internal/idempotency"
	"github.com/aliefbuscode/task-management-api/internal/response"
)

const HeaderIdempotencyKey = "Idempotency-Key"

// Idempotency prevents duplicate task creation when a client retries the
// same POST /tasks request. The header is optional: requests without it
// behave normally. Requests that carry it are deduplicated for `ttl` via
// an atomic Reserve on the repository, so that even N concurrent requests
// with the same key only ever let one of them through to the handler.
func Idempotency(repo idempotency.Repository, ttl time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		keyStr := c.Get(HeaderIdempotencyKey)
		if keyStr == "" {
			return c.Next()
		}

		key, err := uuid.Parse(keyStr)
		if err != nil {
			return apperror.ErrValidation("Idempotency-Key header must be a valid UUID")
		}

		userID := UserID(c)
		endpoint := c.Method() + " " + c.Path()
		hash := sha256.Sum256(c.Body())
		requestHash := hex.EncodeToString(hash[:])

		AddField(c, "idempotency_key", key.String())

		existing, getErr := repo.Get(c.Context(), key)
		if getErr != nil && !errors.Is(getErr, idempotency.ErrNotFound) {
			return apperror.ErrInternal(getErr)
		}

		if getErr == nil {
			if existing.UserID != userID || existing.Endpoint != endpoint {
				return apperror.ErrConflict("idempotency key was already used for a different request")
			}
			if existing.RequestHash != requestHash {
				return apperror.ErrConflict("idempotency key was already used with a different request body")
			}
			if existing.Status == idempotency.StatusCompleted {
				AddField(c, "idempotency_replayed", true)
				c.Status(existing.ResponseCode)
				c.Response().Header.SetContentType(fiber.MIMEApplicationJSON)
				return c.Send(existing.ResponseBody)
			}
			return apperror.New(fiber.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "a request with this idempotency key is already being processed")
		}

		reserved, err := repo.Reserve(c.Context(), key, userID, endpoint, requestHash, ttl)
		if err != nil {
			return apperror.ErrInternal(err)
		}
		if !reserved {
			return apperror.New(fiber.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "a request with this idempotency key is already being processed")
		}

		nextErr := c.Next()

		// If the handler failed, render the error here (instead of letting
		// it bubble to the outer ErrorHandler) so the exact rendered
		// response — success or error — is what gets cached for replay.
		if nextErr != nil {
			appErr := ToAppError(nextErr)
			if renderErr := response.Error(c, appErr); renderErr != nil {
				return renderErr
			}
			nextErr = nil
		}

		status := c.Response().StatusCode()
		body := make([]byte, len(c.Response().Body()))
		copy(body, c.Response().Body())
		_ = repo.Complete(c.Context(), key, status, body)

		return nextErr
	}
}
