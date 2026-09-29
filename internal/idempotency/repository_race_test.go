package idempotency

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// This exercises the Repository contract itself (no DB, no HTTP layer):
// under N concurrent Reserve calls for the same key, exactly one must
// succeed. The real Postgres repository gets this from the unique
// constraint on `key`; fakeRepository gets it from its mutex.
func TestFakeRepository_Reserve_ConcurrentSameKey_OnlyOneWins(t *testing.T) {
	repo := newFakeRepository()
	key := uuid.New()
	const n = 100

	var wg sync.WaitGroup
	results := make([]bool, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ok, err := repo.Reserve(context.Background(), key, 1, "POST /tasks", "hash", 24*time.Hour)
			assert.NoError(t, err)
			results[idx] = ok
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, ok := range results {
		if ok {
			wins++
		}
	}
	assert.Equal(t, 1, wins, "exactly one concurrent Reserve call must succeed for the same key")
	assert.Equal(t, n, repo.reserveCalls, "every goroutine should have attempted a reserve")
}

func TestFakeRepository_Reserve_DifferentKeys_AllWin(t *testing.T) {
	repo := newFakeRepository()
	const n = 20

	var wg sync.WaitGroup
	results := make([]bool, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ok, err := repo.Reserve(context.Background(), uuid.New(), 1, "POST /tasks", "hash", 24*time.Hour)
			assert.NoError(t, err)
			results[idx] = ok
		}(i)
	}
	wg.Wait()

	for _, ok := range results {
		assert.True(t, ok)
	}
}
