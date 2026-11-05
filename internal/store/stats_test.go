package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

func statsClaim(id string, cat report.Category, verdict report.Verdict, checked bool) report.Claim {
	return report.Claim{ID: id, Category: cat, Verdict: verdict, Checked: checked, Evidence: []report.Evidence{}, Timeline: []report.TimelineEntry{}}
}

func statsReport(id, model, provider, mode string, status report.Status, created time.Time, claims ...report.Claim) report.Report {
	return report.Report{ID: id, CreatedAt: created, Model: model, Provider: provider, Mode: mode, Status: status, Claims: claims}
}

func TestLeaderboardCountsCheckedClaimsByModelAndCategory(t *testing.T) {
	s, _ := openStore(t)
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	since := now.Add(-7 * 24 * time.Hour)
	for _, r := range []report.Report{
		statsReport("a", "groq/x", "groq", "async", report.StatusComplete, now.Add(-time.Hour),
			statsClaim("1", report.CategoryPricing, report.VerdictStale, true),
			statsClaim("2", report.CategoryPricing, report.VerdictFresh, true),
			statsClaim("3", report.CategoryVersion, report.VerdictWrong, true),
			statsClaim("4", report.CategoryStatic, report.VerdictNotChecked, false)),
		statsReport("b", "groq/x", "groq", "sync", report.StatusComplete, now.Add(-2*time.Hour),
			statsClaim("5", report.CategoryPricing, report.VerdictStale, true)),
		// Ignored: a pasted check has no model answer, a pending report is not
		// final, and an old report is outside the window.
		statsReport("c", "", "", "check", report.StatusComplete, now.Add(-time.Hour), statsClaim("6", report.CategoryPricing, report.VerdictStale, true)),
		statsReport("d", "groq/x", "groq", "async", report.StatusPending, now.Add(-time.Hour), statsClaim("7", report.CategoryPricing, report.VerdictStale, true)),
		statsReport("e", "groq/x", "groq", "async", report.StatusComplete, since.Add(-time.Hour), statsClaim("8", report.CategoryPricing, report.VerdictStale, true)),
	} {
		require.NoError(t, s.SaveReport(ctx, r))
	}

	cells, err := s.Leaderboard(ctx, since)
	require.NoError(t, err)
	got := map[string]store.LeaderboardCell{}
	for _, c := range cells {
		got[c.Model+"/"+string(c.Category)] = c
	}
	require.Len(t, got, 2)
	assert.Equal(t, store.LeaderboardCell{Model: "groq/x", Provider: "groq", Category: report.CategoryPricing, Checked: 3, Outdated: 2}, got["groq/x/pricing"])
	assert.Equal(t, store.LeaderboardCell{Model: "groq/x", Provider: "groq", Category: report.CategoryVersion, Checked: 1, Incorrect: 1}, got["groq/x/version"])
}

func TestSummaryCountsLastDay(t *testing.T) {
	s, _ := openStore(t)
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	since := now.Add(-24 * time.Hour)
	require.NoError(t, s.SaveReport(ctx, statsReport("a", "groq/x", "groq", "async", report.StatusComplete, now.Add(-time.Hour),
		statsClaim("1", report.CategoryPricing, report.VerdictStale, true),
		statsClaim("2", report.CategoryPricing, report.VerdictWrong, true),
		statsClaim("3", report.CategoryVersion, report.VerdictFresh, true),
		statsClaim("4", report.CategoryStatic, report.VerdictNotChecked, false))))
	require.NoError(t, s.SaveReport(ctx, statsReport("b", "", "", "check", report.StatusComplete, now.Add(-time.Hour),
		statsClaim("5", report.CategoryPricing, report.VerdictFresh, true))))
	require.NoError(t, s.SaveReport(ctx, statsReport("c", "groq/x", "groq", "async", report.StatusPending, now.Add(-time.Hour))))
	require.NoError(t, s.SaveReport(ctx, statsReport("d", "groq/x", "groq", "async", report.StatusComplete, since.Add(-time.Hour),
		statsClaim("6", report.CategoryPricing, report.VerdictStale, true))))
	require.NoError(t, s.RecordPurge(ctx, store.CachePurge{CacheID: "c1", ReportID: "a", Reason: "outdated_answer", At: now.Add(-time.Minute)}))

	got, err := s.Summary(ctx, since)
	require.NoError(t, err)
	assert.Equal(t, store.SummaryCounts{Answers: 2, CheckedClaims: 4, OutdatedClaims: 1, IncorrectClaims: 1, CachePurges: 1}, got)
}
