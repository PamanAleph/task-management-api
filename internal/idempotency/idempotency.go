package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
)

var ErrNotFound = errors.New("idempotency key not found")

type Record struct {
	Key          uuid.UUID
	UserID       int64
	Endpoint     string
	RequestHash  string
	Status       Status
	ResponseCode int
	ResponseBody []byte
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// Repository is intentionally small: Reserve is the only operation that
// needs to be atomic (it is what prevents duplicate task creation under
// concurrent identical requests), backed by a DB unique constraint in the
// Postgres implementation and by a mutex-guarded map in the test fake.
type Repository interface {
	Get(ctx context.Context, key uuid.UUID) (*Record, error)
	// Reserve returns (true, nil) if this call created the pending record,
	// or (false, nil) if a record for that key already existed — exactly
	// one caller among any number of concurrent ones gets true.
	Reserve(ctx context.Context, key uuid.UUID, userID int64, endpoint, requestHash string, ttl time.Duration) (bool, error)
	Complete(ctx context.Context, key uuid.UUID, statusCode int, body []byte) error
}
