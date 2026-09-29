package apperror

import "net/http"

// AppError is the single error type every layer returns. Handlers translate
// it straight into the structured JSON error response; anything else that
// bubbles up is treated as an unexpected internal error and never leaks
// its detail to the client.
type AppError struct {
	HTTPStatus int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func New(httpStatus int, code, message string) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Message: message}
}

func Wrap(httpStatus int, code, message string, err error) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Message: message, Err: err}
}

// Common, reusable error codes. Handlers/services should prefer these over
// inventing ad-hoc strings so the API surface stays predictable.
var (
	ErrValidation   = func(msg string) *AppError { return New(http.StatusBadRequest, "VALIDATION_ERROR", msg) }
	ErrUnauthorized = func(msg string) *AppError { return New(http.StatusUnauthorized, "UNAUTHORIZED", msg) }
	ErrForbidden    = func(msg string) *AppError { return New(http.StatusForbidden, "FORBIDDEN", msg) }
	ErrNotFound     = func(msg string) *AppError { return New(http.StatusNotFound, "NOT_FOUND", msg) }
	ErrConflict     = func(msg string) *AppError { return New(http.StatusConflict, "CONFLICT", msg) }
	ErrInternal     = func(err error) *AppError {
		return Wrap(http.StatusInternalServerError, "INTERNAL_ERROR", "something went wrong", err)
	}
)
