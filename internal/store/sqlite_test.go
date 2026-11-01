package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

var ctx = context.Background()

func openStore(t *testing.T) (*store.SQLite, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.db")
	s, err := store.OpenSQLite(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func intPtr(v int) *int { return &v }

func sampleReport(id string, created time.Time) report.Report {
	published := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return report.Report{
		ID: id, CreatedAt: created, Model: "groq/x", Provider: "groq", VirtualKey: "vk", Mode: "async",
		Status: report.StatusComplete, Prompt: "What does it cost?", AnswerText: "It costs $20.",
		Score: intPtr(42), StaleCount: 1, CheckedCount: 1, TotalClaims: 2, LatencyMS: 1500, CreditsUsed: 1,
		Claims: []report.Claim{
			{
				ID: "c1", Quote: "costs $20", SpanStart: 3, SpanEnd: 12, Entity: "Example", Attribute: "price", Value: "$20",
				Category: report.CategoryPricing, TemporalMarker: "currently", SearchQuery: "example price",
				PChanged: 0.9, Checked: true, Verdict: report.VerdictStale, CurrentValue: "$25", AsOf: "2026-09-01",
				Score: intPtr(2), Confidence: 0.9, Rationale: "The price changed.",
				Evidence: []report.Evidence{
					{Ref: "E1", URL: "https://example.com/p", Title: "Pricing", Domain: "example.com", PublishedDate: &published,
						Snippet: "Now $25.", Authority: 1, Stance: report.StanceSupports},
					{Ref: "E2", URL: "https://blog.dev/p", Title: "Blog", Domain: "blog.dev", Snippet: "Was $20.",
						Authority: 0.5, Stance: report.StanceNeutral},
				},
				Timeline: []report.TimelineEntry{},
			},
			{
				ID: "c2", Quote: "It", SpanStart: -1, SpanEnd: -1, Category: report.CategoryStatic,
				Verdict: report.VerdictNotChecked, Evidence: []report.Evidence{}, Timeline: []report.TimelineEntry{},
			},
		},
	}
}

func TestOpenAppliesMigrationsOnce(t *testing.T) {
	s, path := openStore(t)
	require.NoError(t, s.Close())
	s2, err := store.OpenSQLite(path)
	require.NoError(t, err)
	require.NoError(t, s2.Close())

	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer db.Close()
	var version int
	require.NoError(t, db.QueryRow("PRAGMA user_version").Scan(&version))
	assert.Equal(t, 1, version)
}

func TestSaveAndGetReportRoundTrip(t *testing.T) {
	s, _ := openStore(t)
	want := sampleReport("r1", time.Date(2026, 11, 3, 10, 0, 0, 123000000, time.UTC))
	require.NoError(t, s.SaveReport(ctx, want))
	got, err := s.GetReport(ctx, "r1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.NotNil(t, got.Claims[1].Evidence)
	assert.NotNil(t, got.Claims[1].Timeline)
}

func TestSaveReportReplacesClaims(t *testing.T) {
	s, _ := openStore(t)
	r := sampleReport("r1", time.Now().UTC())
	require.NoError(t, s.SaveReport(ctx, r))
	r.Claims = r.Claims[:1]
	require.NoError(t, s.SaveReport(ctx, r))
	got, err := s.GetReport(ctx, "r1")
	require.NoError(t, err)
	assert.Len(t, got.Claims, 1)
}

func TestGetReportNotFound(t *testing.T) {
	s, _ := openStore(t)
	_, err := s.GetReport(ctx, "missing")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestListReportsFiltersAndOrders(t *testing.T) {
	s, _ := openStore(t)
	t1 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	t2, t3 := t1.Add(time.Hour), t1.Add(2*time.Hour)
	for _, r := range []struct {
		id, model string
		at        time.Time
	}{{"r1", "a", t1}, {"r2", "b", t2}, {"r3", "a", t3}} {
		rep := sampleReport(r.id, r.at)
		rep.Model = r.model
		require.NoError(t, s.SaveReport(ctx, rep))
	}

	got, err := s.ListReports(ctx, store.ListFilter{Model: "a", Since: t1.Add(time.Second)})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "r3", got[0].ID)

	got, err = s.ListReports(ctx, store.ListFilter{})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"r3", "r2", "r1"}, []string{got[0].ID, got[1].ID, got[2].ID})
	for _, r := range got {
		assert.Nil(t, r.Claims)
	}

	got, err = s.ListReports(ctx, store.ListFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "r3", got[0].ID)
}

func TestEffectiveLimit(t *testing.T) {
	assert.Equal(t, 50, store.EffectiveLimit(0))
	assert.Equal(t, 50, store.EffectiveLimit(-5))
	assert.Equal(t, 80, store.EffectiveLimit(80))
	assert.Equal(t, 200, store.EffectiveLimit(1000))
}

func TestUpsertFactAndTimeline(t *testing.T) {
	s, _ := openStore(t)
	verified := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	fact := report.Fact{Key: "example|price", Entity: "Example", Attribute: "price", Category: report.CategoryPricing,
		CurrentValue: "$20", AsOf: "2025-01-01", SourceURL: "https://example.com/p", LastVerifiedAt: verified}
	id1, err := s.UpsertFact(ctx, fact)
	require.NoError(t, err)
	fact.CurrentValue = "$25"
	id2, err := s.UpsertFact(ctx, fact)
	require.NoError(t, err)
	assert.Equal(t, id1, id2)

	got, err := s.LookupFact(ctx, "example|price")
	require.NoError(t, err)
	assert.Equal(t, "$25", got.CurrentValue)
	assert.Equal(t, id1, got.ID)
	assert.True(t, verified.Equal(got.LastVerifiedAt))

	_, err = s.LookupFact(ctx, "missing")
	assert.ErrorIs(t, err, store.ErrNotFound)

	entries := []report.TimelineEntry{
		{Value: "$25", ValidFrom: "2026-09-01", SourceURL: "https://example.com/p"},
		{Value: "$20", ValidFrom: "2025-01-01", SourceURL: "https://example.com/p"},
	}
	require.NoError(t, s.AddTimeline(ctx, id1, entries))
	require.NoError(t, s.AddTimeline(ctx, id1, entries[:1]))

	r := sampleReport("r1", time.Now().UTC())
	r.Claims[0].FactID = &id1
	require.NoError(t, s.SaveReport(ctx, r))
	rep, err := s.GetReport(ctx, "r1")
	require.NoError(t, err)
	require.Len(t, rep.Claims[0].Timeline, 2)
	assert.Equal(t, "$20", rep.Claims[0].Timeline[0].Value)
	assert.Equal(t, "$25", rep.Claims[0].Timeline[1].Value)
}

func TestFailPendingMarksLeftoverReports(t *testing.T) {
	s, _ := openStore(t)
	pending := sampleReport("r-pending", time.Now().UTC())
	pending.Status = report.StatusPending
	require.NoError(t, s.SaveReport(ctx, pending))
	require.NoError(t, s.SaveReport(ctx, sampleReport("r-done", time.Now().UTC())))

	n, err := s.FailPending(ctx, "The gateway stopped before verification finished.")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, err := s.GetReport(ctx, "r-pending")
	require.NoError(t, err)
	assert.Equal(t, report.StatusError, got.Status)
	assert.Equal(t, "The gateway stopped before verification finished.", got.Reason)
	done, err := s.GetReport(ctx, "r-done")
	require.NoError(t, err)
	assert.Equal(t, report.StatusComplete, done.Status)
}

func TestCreditsLedger(t *testing.T) {
	s, _ := openStore(t)
	used, err := s.CreditsUsed(ctx, "2026-11-03")
	require.NoError(t, err)
	assert.Equal(t, 0, used)
	require.NoError(t, s.AddCredits(ctx, "2026-11-03", 2, 30))
	require.NoError(t, s.AddCredits(ctx, "2026-11-03", 3, 30))
	used, err = s.CreditsUsed(ctx, "2026-11-03")
	require.NoError(t, err)
	assert.Equal(t, 5, used)
}

func TestConcurrentSavesSucceed(t *testing.T) {
	s, _ := openStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.SaveReport(ctx, sampleReport(fmt.Sprintf("r%d", i), time.Now().UTC()))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err)
	}
	got, err := s.ListReports(ctx, store.ListFilter{Limit: 50})
	require.NoError(t, err)
	assert.Len(t, got, 20)
}
