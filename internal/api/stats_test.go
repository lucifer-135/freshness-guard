package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/api"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

type fakeStats struct {
	cells        []store.LeaderboardCell
	summary      store.SummaryCounts
	leaderSince  time.Time
	summarySince time.Time
}

func (f *fakeStats) Leaderboard(_ context.Context, since time.Time) ([]store.LeaderboardCell, error) {
	f.leaderSince = since
	return f.cells, nil
}

func (f *fakeStats) Summary(_ context.Context, since time.Time) (store.SummaryCounts, error) {
	f.summarySince = since
	return f.summary, nil
}

func newStatsHandler(stats *fakeStats, purgeEnabled bool) http.Handler {
	return api.NewHandler(api.Deps{
		Token: "t", Stats: stats, Credits: fakeCredits{used: 4, daily: 200}, CachePurgeEnabled: purgeEnabled,
		Now: func() time.Time { return fixedNow },
	})
}

func TestLeaderboardGroupsCellsByModel(t *testing.T) {
	stats := &fakeStats{cells: []store.LeaderboardCell{
		{Model: "groq/small", Provider: "groq", Category: report.CategoryPricing, Checked: 12, Outdated: 4},
		{Model: "groq/small", Provider: "groq", Category: report.CategoryVersion, Checked: 28, Outdated: 4, Incorrect: 2},
		{Model: "gemini/big", Provider: "gemini", Category: report.CategoryPricing, Checked: 50, Outdated: 5},
		{Model: "groq/large", Provider: "groq", Category: report.CategoryPricing, Checked: 60, Incorrect: 1},
	}}
	rec := do(newStatsHandler(stats, true), http.MethodGet, "/freshness/leaderboard", "t", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		WindowDays int       `json:"window_days"`
		Since      time.Time `json:"since"`
		MinSample  int       `json:"min_sample"`
		Rows       []struct {
			Model, Provider string
			Total           map[string]int            `json:"total"`
			Cells           map[string]map[string]int `json:"cells"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 7, body.WindowDays)
	assert.Equal(t, 10, body.MinSample)
	assert.True(t, fixedNow.Add(-7*24*time.Hour).Equal(body.Since))
	assert.True(t, fixedNow.Add(-7*24*time.Hour).Equal(stats.leaderSince))

	// Sorted by provider, then by checked claims, largest first.
	require.Len(t, body.Rows, 3)
	assert.Equal(t, []string{"gemini/big", "groq/large", "groq/small"}, []string{body.Rows[0].Model, body.Rows[1].Model, body.Rows[2].Model})
	small := body.Rows[2]
	assert.Equal(t, map[string]int{"checked": 40, "outdated": 8, "incorrect": 2}, small.Total)
	assert.Equal(t, map[string]int{"checked": 12, "outdated": 4, "incorrect": 0}, small.Cells["pricing"])
	assert.Equal(t, map[string]int{"checked": 28, "outdated": 4, "incorrect": 2}, small.Cells["version"])
}

func TestLeaderboardWithoutDataHasEmptyRows(t *testing.T) {
	rec := do(newStatsHandler(&fakeStats{}, true), http.MethodGet, "/freshness/leaderboard", "t", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"rows":[]`)
}

func TestLeaderboardUsesThirtyDayWindow(t *testing.T) {
	stats := &fakeStats{}
	rec := do(newStatsHandler(stats, true), http.MethodGet, "/freshness/leaderboard?window=30d", "t", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, fixedNow.Add(-30*24*time.Hour).Equal(stats.leaderSince))
	assert.Contains(t, rec.Body.String(), `"window_days":30`)
}

func TestLeaderboardRejectsUnknownWindow(t *testing.T) {
	rec := do(newStatsHandler(&fakeStats{}, true), http.MethodGet, "/freshness/leaderboard?window=1d", "t", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "The window must be 7d or 30d.", errorOf(t, rec))
}

func TestSummaryIncludesCreditsAndPurgeSetting(t *testing.T) {
	stats := &fakeStats{summary: store.SummaryCounts{Answers: 3, CheckedClaims: 6, OutdatedClaims: 2, IncorrectClaims: 1, CachePurges: 1}}
	rec := do(newStatsHandler(stats, true), http.MethodGet, "/freshness/summary", "t", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{
		"window_hours": 24.0, "answers": 3.0, "checked_claims": 6.0, "outdated_claims": 2.0, "incorrect_claims": 1.0,
		"cache_purges": 1.0, "cache_purge_enabled": true, "credits": map[string]any{"used": 4.0, "budget": 200.0},
	}, body)
	assert.True(t, fixedNow.Add(-24*time.Hour).Equal(stats.summarySince))
}

func TestStatsRoutesNeedToken(t *testing.T) {
	h := newStatsHandler(&fakeStats{}, true)
	assert.Equal(t, http.StatusUnauthorized, do(h, http.MethodGet, "/freshness/leaderboard", "", "").Code)
	assert.Equal(t, http.StatusUnauthorized, do(h, http.MethodGet, "/freshness/summary", "", "").Code)
}
