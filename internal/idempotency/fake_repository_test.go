package idempotency

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// fakeRepository is an in-memory stand-in for the Postgres repository,
// guarded by a mutex the same way the real implementation is guarded by
// the DB's unique constraint on `key`. It exists purely so the race-
// condition tests can run without any database connection.
type fakeRepository struct {
	mu      sync.Mutex
	records map[uuid.UUID]*Record

	// reserveCalls counts every call to Reserve, so tests can assert on
	// how many attempts were made regardless of how many succeeded.
	reserveCalls int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{records: make(map[uuid.UUID]*Record)}
}

func (f *fakeRepository) Get(ctx context.Context, key uuid.UUID) (*Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	rec, ok := f.records[key]
	if !ok {
		return nil, ErrNotFound
	}
	copyRec := *rec
	return &copyRec, nil
}

func (f *fakeRepository) Reserve(ctx context.Context, key uuid.UUID, userID int64, endpoint, requestHash string, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.reserveCalls++

	if _, exists := f.records[key]; exists {
		return false, nil
	}

	f.records[key] = &Record{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
	}
	return true, nil
}

func (f *fakeRepository) Complete(ctx context.Context, key uuid.UUID, statusCode int, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	rec, ok := f.records[key]
	if !ok {
		return ErrNotFound
	}
	rec.Status = StatusCompleted
	rec.ResponseCode = statusCode
	rec.ResponseBody = body
	return nil
}
