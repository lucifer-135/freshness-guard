package facts_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/facts"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

var (
	ctx = context.Background()
	now = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
)

const day = 24 * time.Hour

type fakeStore struct {
	facts    map[string]report.Fact
	upserts  []report.Fact
	timeline []report.TimelineEntry
	calls    int
}

func (f *fakeStore) LookupFact(_ context.Context, key string) (report.Fact, error) {
	f.calls++
	fact, ok := f.facts[key]
	if !ok {
		return report.Fact{}, errors.New("not found")
	}
	return fact, nil
}

func (f *fakeStore) UpsertFact(_ context.Context, fact report.Fact) (int64, error) {
	f.calls++
	f.upserts = append(f.upserts, fact)
	return 7, nil
}

func (f *fakeStore) AddTimeline(_ context.Context, _ int64, entries []report.TimelineEntry) error {
	f.calls++
	f.timeline = append(f.timeline, entries...)
	return nil
}

func newCache(s *fakeStore) *facts.Cache {
	return &facts.Cache{Store: s, Now: func() time.Time { return now }}
}

func TestNormalizeKey(t *testing.T) {
	assert.Equal(t, "github copilot pro|monthly price", facts.NormalizeKey("  GitHub Copilot  Pro ", "Monthly price!"))
	assert.Equal(t, "openai gpt 4o api|price per 1m tokens", facts.NormalizeKey("OpenAI GPT-4o API", "price per 1M tokens"))
}

func TestFreshWithinHalfLife(t *testing.T) {
	key := facts.NormalizeKey("Example", "price")
	s := &fakeStore{facts: map[string]report.Fact{key: {Key: key, LastVerifiedAt: now.Add(-30 * day)}}}
	_, ok := newCache(s).Fresh(ctx, "Example", "price", report.CategoryPricing)
	assert.True(t, ok)

	s.facts[key] = report.Fact{Key: key, LastVerifiedAt: now.Add(-61 * day)}
	_, ok = newCache(s).Fresh(ctx, "Example", "price", report.CategoryPricing)
	assert.False(t, ok)

	fact, ok := newCache(s).Fresh(ctx, "Example", "price", report.CategoryLeadership)
	assert.True(t, ok)
	assert.Equal(t, key, fact.Key)

	_, ok = newCache(s).Fresh(ctx, "Example", "price", report.CategoryStatic)
	assert.False(t, ok)
}

func TestFreshMissingFact(t *testing.T) {
	_, ok := newCache(&fakeStore{}).Fresh(ctx, "Example", "price", report.CategoryPricing)
	assert.False(t, ok)
}

func TestRecordStoresVerifiedClaims(t *testing.T) {
	s := &fakeStore{}
	claim := report.Claim{
		Entity: "Example Cloud Pro", Attribute: "monthly price", Value: "$20", Category: report.CategoryPricing,
		Verdict: report.VerdictStale, CurrentValue: "$25", AsOf: "2026-09-01",
		Evidence: []report.Evidence{
			{URL: "https://a", Stance: report.StanceNeutral},
			{URL: "https://b", Stance: report.StanceSupports},
		},
		Timeline: []report.TimelineEntry{{Value: "$20", ValidFrom: "2025-01-01"}, {Value: "$25", ValidFrom: "2026-09-01"}},
	}
	id, recorded, err := newCache(s).Record(ctx, claim)
	require.NoError(t, err)
	assert.True(t, recorded)
	assert.Equal(t, int64(7), id)
	require.Len(t, s.upserts, 1)
	got := s.upserts[0]
	assert.Equal(t, "example cloud pro|monthly price", got.Key)
	assert.Equal(t, "Example Cloud Pro", got.Entity)
	assert.Equal(t, "$25", got.CurrentValue)
	assert.Equal(t, "2026-09-01", got.AsOf)
	assert.Equal(t, "https://b", got.SourceURL)
	assert.Equal(t, report.CategoryPricing, got.Category)
	assert.True(t, now.Equal(got.LastVerifiedAt))
	assert.Len(t, s.timeline, 2)
}

func TestRecordUsesFirstEvidenceWithoutSupport(t *testing.T) {
	s := &fakeStore{}
	claim := report.Claim{Entity: "E", Attribute: "A", Verdict: report.VerdictWrong, CurrentValue: "x",
		Evidence: []report.Evidence{{URL: "https://first"}, {URL: "https://second"}}}
	_, _, err := newCache(s).Record(ctx, claim)
	require.NoError(t, err)
	assert.Equal(t, "https://first", s.upserts[0].SourceURL)
}

func TestRecordSkipsUnverifiable(t *testing.T) {
	s := &fakeStore{}
	_, recorded, err := newCache(s).Record(ctx, report.Claim{Entity: "E", Attribute: "A", Verdict: report.VerdictUnverifiable, CurrentValue: "x"})
	require.NoError(t, err)
	assert.False(t, recorded)
	assert.Equal(t, 0, s.calls)
}

func TestRecordFreshWithoutCurrentValueUsesClaimValue(t *testing.T) {
	s := &fakeStore{}
	_, recorded, err := newCache(s).Record(ctx, report.Claim{Entity: "E", Attribute: "A", Value: "4.2", Verdict: report.VerdictFresh})
	require.NoError(t, err)
	assert.True(t, recorded)
	assert.Equal(t, "4.2", s.upserts[0].CurrentValue)
}

func TestRecordStaleWithoutValueIsSkipped(t *testing.T) {
	s := &fakeStore{}
	_, recorded, err := newCache(s).Record(ctx, report.Claim{Entity: "E", Attribute: "A", Value: "4.2", Verdict: report.VerdictStale})
	require.NoError(t, err)
	assert.False(t, recorded)
	assert.Equal(t, 0, s.calls)
}

func TestCachedEvidence(t *testing.T) {
	fact := report.Fact{Entity: "Example", Attribute: "price", CurrentValue: "$25", AsOf: "2026-09-01",
		SourceURL: "https://www.example.com/p", LastVerifiedAt: now}
	e := facts.CachedEvidence(fact)
	assert.Equal(t, "https://www.example.com/p", e.URL)
	assert.Equal(t, "example.com", e.Domain)
	assert.Equal(t, "Previously verified fact", e.Title)
	require.NotNil(t, e.PublishedDate)
	assert.True(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Equal(*e.PublishedDate))
	assert.Contains(t, e.Snippet, "as of 2026-09-01")
	assert.Contains(t, e.Snippet, "verified on 2026-11-03")
	assert.Equal(t, report.StanceNeutral, e.Stance)

	fact.AsOf = ""
	e = facts.CachedEvidence(fact)
	assert.True(t, now.Equal(*e.PublishedDate))
	assert.True(t, strings.Contains(e.Snippet, "as of an unknown date"))
}
