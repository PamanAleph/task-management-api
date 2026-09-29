package middleware

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aliefbuscode/task-management-api/internal/idempotency"
)

// fakeIdempotencyRepo mirrors the Postgres implementation's atomicity
// guarantee (there, a unique constraint on `key`; here, a mutex) so the
// tests below run entirely without a database, per the test requirements.
type fakeIdempotencyRepo struct {
	mu      sync.Mutex
	records map[uuid.UUID]*idempotency.Record
}

func newFakeIdempotencyRepo() *fakeIdempotencyRepo {
	return &fakeIdempotencyRepo{records: make(map[uuid.UUID]*idempotency.Record)}
}

func (f *fakeIdempotencyRepo) Get(_ context.Context, key uuid.UUID) (*idempotency.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	rec, ok := f.records[key]
	if !ok {
		return nil, idempotency.ErrNotFound
	}
	copyRec := *rec
	return &copyRec, nil
}

func (f *fakeIdempotencyRepo) Reserve(_ context.Context, key uuid.UUID, userID int64, endpoint, requestHash string, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.records[key]; exists {
		return false, nil
	}
	f.records[key] = &idempotency.Record{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		Status:      idempotency.StatusPending,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
	}
	return true, nil
}

func (f *fakeIdempotencyRepo) Complete(_ context.Context, key uuid.UUID, statusCode int, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	rec, ok := f.records[key]
	if !ok {
		return idempotency.ErrNotFound
	}
	rec.Status = idempotency.StatusCompleted
	rec.ResponseCode = statusCode
	rec.ResponseBody = body
	return nil
}

func discardLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func newTestApp(repo idempotency.Repository, createCount *atomic.Int64, handlerDelay time.Duration) *fiber.App {
	app := fiber.New()
	app.Use(RequestLogger(discardLogger()))
	app.Use(ErrorHandler(discardLogger()))

	app.Post("/tasks", Idempotency(repo, 24*time.Hour), func(c *fiber.Ctx) error {
		if handlerDelay > 0 {
			time.Sleep(handlerDelay)
		}
		n := createCount.Add(1)
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"status": "success",
			"data":   fiber.Map{"id": n, "title": "created"},
		})
	})
	return app
}

func TestIdempotency_Sequential_SameKeyReturnsIdenticalResponse(t *testing.T) {
	repo := newFakeIdempotencyRepo()
	var createCount atomic.Int64
	app := newTestApp(repo, &createCount, 0)

	key := uuid.NewString()

	req1 := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"title":"a"}`))
	req1.Header.Set("Idempotency-Key", key)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	body1, _ := io.ReadAll(resp1.Body)
	require.Equal(t, fiber.StatusCreated, resp1.StatusCode)

	req2 := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"title":"a"}`))
	req2.Header.Set("Idempotency-Key", key)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)

	assert.Equal(t, resp1.StatusCode, resp2.StatusCode)
	assert.Equal(t, string(body1), string(body2), "replayed response must be identical to the first")
	assert.Equal(t, int64(1), createCount.Load(), "second request must not create a new task")
}

func TestIdempotency_Concurrent_DuplicateKeyOnlyCreatesOnce(t *testing.T) {
	repo := newFakeIdempotencyRepo()
	var createCount atomic.Int64
	// small delay widens the race window so concurrent goroutines actually
	// overlap inside the handler instead of running sequentially.
	app := newTestApp(repo, &createCount, 5*time.Millisecond)

	key := uuid.NewString()
	const n = 50

	var wg sync.WaitGroup
	statuses := make([]int, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"title":"a"}`))
			req.Header.Set("Idempotency-Key", key)
			resp, err := app.Test(req, 5000)
			require.NoError(t, err)
			statuses[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(1), createCount.Load(),
		"exactly one task must be created even when N requests share the same Idempotency-Key concurrently")

	created := 0
	for _, s := range statuses {
		if s == fiber.StatusCreated {
			created++
		}
	}
	assert.GreaterOrEqual(t, created, 1, "at least the winning request should observe 201")
}

func TestIdempotency_NoKey_AlwaysCreates(t *testing.T) {
	repo := newFakeIdempotencyRepo()
	var createCount atomic.Int64
	app := newTestApp(repo, &createCount, 0)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"title":"a"}`))
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	}

	assert.Equal(t, int64(3), createCount.Load(), "without an idempotency key every request creates a task")
}
