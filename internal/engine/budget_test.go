package engine_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/engine"
)

type creditCall struct {
	day       string
	n, budget int
}

type fakeCreditStore struct {
	mu    sync.Mutex
	used  map[string]int
	calls []creditCall
}

func (f *fakeCreditStore) CreditsUsed(_ context.Context, day string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.used[day], nil
}

func (f *fakeCreditStore) AddCredits(_ context.Context, day string, n, budget int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.used == nil {
		f.used = map[string]int{}
	}
	f.used[day] += n
	f.calls = append(f.calls, creditCall{day, n, budget})
	return nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestBudgetSpendsUpToDaily(t *testing.T) {
	store := &fakeCreditStore{}
	c := &clock{time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)}
	b := engine.NewBudget(store, 3, c.now, slog.Default())
	assert.True(t, b.TrySpend(1))
	assert.True(t, b.TrySpend(1))
	assert.True(t, b.TrySpend(1))
	assert.False(t, b.TrySpend(1))
	assert.Equal(t, 3, b.Used())
	assert.Equal(t, 3, b.Daily())
	assert.Equal(t, []creditCall{{"2026-11-03", 1, 3}, {"2026-11-03", 1, 3}, {"2026-11-03", 1, 3}}, store.calls)

	b2 := engine.NewBudget(&fakeCreditStore{}, 3, c.now, slog.Default())
	assert.True(t, b2.TrySpend(2))
	assert.False(t, b2.TrySpend(2))
}

func TestBudgetLoadsUsedAndResetsDaily(t *testing.T) {
	store := &fakeCreditStore{used: map[string]int{"2026-11-03": 29}}
	c := &clock{time.Date(2026, 11, 3, 23, 0, 0, 0, time.UTC)}
	b := engine.NewBudget(store, 30, c.now, slog.Default())
	assert.True(t, b.TrySpend(1))
	assert.False(t, b.TrySpend(1))

	c.t = c.t.Add(2 * time.Hour)
	assert.True(t, b.TrySpend(1))
	assert.Equal(t, 1, b.Used())
}

func TestBudgetIsSafeForConcurrentUse(t *testing.T) {
	c := &clock{time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)}
	b := engine.NewBudget(&fakeCreditStore{}, 50, c.now, slog.Default())
	var wg sync.WaitGroup
	var mu sync.Mutex
	spent := 0
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.TrySpend(1) {
				mu.Lock()
				spent++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 50, spent)
}

func TestBudgetRefund(t *testing.T) {
	store := &fakeCreditStore{}
	c := &clock{time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)}
	b := engine.NewBudget(store, 3, c.now, slog.Default())
	require.True(t, b.TrySpend(1))
	require.True(t, b.TrySpend(1))
	b.Refund(1)
	assert.Equal(t, 1, b.Used())
	assert.True(t, b.TrySpend(2))
	b.Refund(10)
	assert.Equal(t, 0, b.Used())
	assert.Contains(t, store.calls, creditCall{"2026-11-03", -1, 3})
}
