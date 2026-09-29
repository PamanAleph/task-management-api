package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New builds the process-wide structured JSON logger. All request logging
// goes through the "wide event" middleware, which emits exactly one line
// per request carrying every field relevant to that request, rather than
// several fragmented log lines.
func New() zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.DurationFieldInteger = false

	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
