package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	LocalsRequestID = "request_id"
	LocalsWideEvent = "wide_event"
)

// WideEvent lets any downstream handler attach extra context (task_id,
// idempotency_key, etc.) that gets folded into the single request-level
// log line emitted at the end — the "wide event" pattern: one rich
// structured event per request instead of many narrow log statements.
type WideEvent map[string]interface{}

// AddField enriches the current request's wide event. Safe to call from
// any handler/service that has access to the fiber.Ctx.
func AddField(c *fiber.Ctx, key string, value interface{}) {
	if ev, ok := c.Locals(LocalsWideEvent).(WideEvent); ok {
		ev[key] = value
	}
}

func RequestID(c *fiber.Ctx) string {
	if v, ok := c.Locals(LocalsRequestID).(string); ok {
		return v
	}
	return ""
}

// RequestLogger must be registered first (outermost), before ErrorHandler,
// so that by the time it inspects the response, ErrorHandler has already
// written the final status code for both success and error paths.
func RequestLogger(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		reqID := c.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Locals(LocalsRequestID, reqID)
		c.Locals(LocalsWideEvent, WideEvent{})
		c.Set("X-Request-ID", reqID)

		err := c.Next()

		status := c.Response().StatusCode()
		latency := time.Since(start)

		event := log.With().
			Str("request_id", reqID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status_code", status).
			Dur("latency_ms", latency).
			Str("ip", c.IP()).
			Str("user_agent", c.Get("User-Agent")).
			Int("response_bytes", len(c.Response().Body()))

		if ev, ok := c.Locals(LocalsWideEvent).(WideEvent); ok {
			for k, v := range ev {
				event = event.Interface(k, v)
			}
		}

		logger := event.Logger()

		switch {
		case status >= 500:
			logger.Error().Msg("request completed")
		case status >= 400:
			logger.Warn().Msg("request completed")
		default:
			logger.Info().Msg("request completed")
		}

		return err
	}
}
