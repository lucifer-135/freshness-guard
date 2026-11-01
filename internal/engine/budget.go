// Package engine runs the verification pipeline in a bounded worker pool,
// within a daily Tavily credit budget.
package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// CreditStore persists the credits used per UTC day.
type CreditStore interface {
	CreditsUsed(ctx context.Context, day string) (int, error)
	AddCredits(ctx context.Context, day string, n, budget int) error
}

// Budget is the daily Tavily credit budget. It is safe for concurrent use.
type Budget struct {
	store  CreditStore
	daily  int
	now    func() time.Time
	logger *slog.Logger

	mu   sync.Mutex
	day  string
	used int
}

// NewBudget returns a budget of daily credits per UTC day.
func NewBudget(store CreditStore, daily int, now func() time.Time, logger *slog.Logger) *Budget {
	return &Budget{store: store, daily: daily, now: now, logger: logger}
}

// roll loads the credits already used when a new UTC day starts. The caller
// holds b.mu.
func (b *Budget) roll() {
	day := b.now().UTC().Format("2006-01-02")
	if day == b.day {
		return
	}
	b.day = day
	used, err := b.store.CreditsUsed(context.Background(), day)
	if err != nil {
		b.logger.Error("The Tavily credit ledger could not be read, so today's usage starts at zero.", "error", err)
		used = 0
	}
	b.used = used
}

// TrySpend reserves n credits. It returns false when that would exceed the
// daily budget.
func (b *Budget) TrySpend(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	if b.used+n > b.daily {
		return false
	}
	b.used += n
	if err := b.store.AddCredits(context.Background(), b.day, n, b.daily); err != nil {
		b.logger.Error("The Tavily credit ledger could not be updated.", "error", err)
	}
	return true
}

// Used returns the credits used today.
func (b *Budget) Used() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	return b.used
}

// Daily returns the daily budget.
func (b *Budget) Daily() int {
	return b.daily
}
