// Package store persists reports, facts and the Tavily credit ledger.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// ErrNotFound is returned when a report or fact does not exist.
var ErrNotFound = errors.New("not found")

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// ListFilter selects reports for ListReports.
type ListFilter struct {
	Model string
	Since time.Time
	Limit int
}

// EffectiveLimit returns the number of reports a list returns for limit n.
func EffectiveLimit(n int) int {
	switch {
	case n <= 0:
		return defaultListLimit
	case n > maxListLimit:
		return maxListLimit
	default:
		return n
	}
}

// HashVirtualKey returns the form in which a virtual key is stored: a short
// SHA-256 prefix, because the key is a credential. It is empty for an empty
// key and leaves an already hashed value unchanged.
func HashVirtualKey(key string) string {
	if key == "" || strings.HasPrefix(key, hashPrefix) {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	return hashPrefix + hex.EncodeToString(sum[:])[:16]
}

const hashPrefix = "sha256:"

// Store is the storage used by the pipeline and the API.
type Store interface {
	// SaveReport inserts or updates the report and replaces its claims and evidence.
	SaveReport(ctx context.Context, r report.Report) error
	GetReport(ctx context.Context, id string) (report.Report, error)
	// ListReports returns reports newest first, without claims.
	ListReports(ctx context.Context, f ListFilter) ([]report.Report, error)
	LookupFact(ctx context.Context, key string) (report.Fact, error)
	// UpsertFact inserts or updates the fact with f.Key and returns its ID.
	UpsertFact(ctx context.Context, f report.Fact) (int64, error)
	// AddTimeline stores timeline entries for a fact; duplicates are ignored.
	AddTimeline(ctx context.Context, factID int64, entries []report.TimelineEntry) error
	// FailPending marks reports left pending by an earlier run as failed.
	FailPending(ctx context.Context, reason string) (int64, error)
	CreditsUsed(ctx context.Context, day string) (int, error)
	AddCredits(ctx context.Context, day string, n, budget int) error
	Close() error
}
