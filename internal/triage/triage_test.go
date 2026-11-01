package triage_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/triage"
)

const day = 24 * time.Hour

func TestHalfLives(t *testing.T) {
	want := map[report.Category]int{
		report.CategoryEvent: 30, report.CategoryPricing: 60, report.CategoryVersion: 90,
		report.CategoryStatistic: 120, report.CategoryFeature: 120, report.CategoryLaw: 180, report.CategoryLeadership: 365,
	}
	for cat, days := range want {
		hl, ok := triage.HalfLife(cat)
		assert.True(t, ok, cat)
		assert.Equal(t, time.Duration(days)*day, hl, cat)
	}
	_, ok := triage.HalfLife(report.CategoryStatic)
	assert.False(t, ok)
	_, ok = triage.HalfLife("unknown")
	assert.False(t, ok)
}

func TestPChanged(t *testing.T) {
	assert.InDelta(t, 0.5, triage.PChanged(report.CategoryPricing, 60*day, false), 1e-9)
	assert.InDelta(t, 0.75, triage.PChanged(report.CategoryPricing, 120*day, false), 1e-9)
	assert.InDelta(t, 0.75, triage.PChanged(report.CategoryPricing, 60*day, true), 1e-9)
	assert.Equal(t, 0.0, triage.PChanged(report.CategoryStatic, 1000*day, true))
	assert.Equal(t, 0.0, triage.PChanged(report.CategoryPricing, 0, false))
}

func TestModelAge(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cutoffs := map[string]time.Time{
		"groq/x":   time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC),
		"groq/new": time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, 365*day, triage.ModelAge("groq/x", cutoffs, now))
	assert.Equal(t, 365*day, triage.ModelAge("unknown", cutoffs, now))
	assert.Equal(t, time.Duration(0), triage.ModelAge("groq/new", cutoffs, now))
	assert.Equal(t, 365*day, triage.ModelAge("groq/x", nil, now))
}

func TestThreshold(t *testing.T) {
	assert.Equal(t, 0.3, triage.Threshold(0.3, 0, 30))
	assert.Equal(t, 0.3, triage.Threshold(0.3, 15, 30))
	assert.InDelta(t, 0.72, triage.Threshold(0.3, 24, 30), 1e-9)
	assert.True(t, math.IsInf(triage.Threshold(0.3, 30, 30), 1))
	assert.True(t, math.IsInf(triage.Threshold(0.3, 0, 0), 1))
}

type fakeSpender struct{ daily, used int }

func (f *fakeSpender) TrySpend(n int) bool {
	if f.used+n > f.daily {
		return false
	}
	f.used += n
	return true
}
func (f *fakeSpender) Used() int  { return f.used }
func (f *fakeSpender) Daily() int { return f.daily }

func claimsWithP(ps ...float64) []report.Claim {
	claims := make([]report.Claim, len(ps))
	for i, p := range ps {
		claims[i] = report.Claim{ID: string(rune('1' + i)), PChanged: p, Verdict: report.VerdictNotChecked}
	}
	return claims
}

func checked(claims []report.Claim) []bool {
	out := make([]bool, len(claims))
	for i, c := range claims {
		out[i] = c.Checked
	}
	return out
}

func TestSelectChecksRiskiestFirstWithinBudget(t *testing.T) {
	in := claimsWithP(0.4, 0.9, 0.2, 0.8)
	got := triage.Select(in, 0.3, &fakeSpender{daily: 2})
	require.Len(t, got, 4)
	assert.Equal(t, []bool{false, true, false, true}, checked(got))
	assert.Equal(t, []string{"1", "2", "3", "4"}, []string{got[0].ID, got[1].ID, got[2].ID, got[3].ID})
}

func TestSelectKeepsCacheHitsChecked(t *testing.T) {
	in := claimsWithP(0.1)
	in[0].Checked, in[0].CacheHit = true, true
	spender := &fakeSpender{daily: 0}
	got := triage.Select(in, 0.3, spender)
	assert.True(t, got[0].Checked)
	assert.Equal(t, 0, spender.used)
}

func TestSelectRaisesThresholdAsBudgetIsUsed(t *testing.T) {
	got := triage.Select(claimsWithP(0.9, 0.6), 0.3, &fakeSpender{daily: 10, used: 8})
	assert.Equal(t, []bool{true, false}, checked(got))
}

func TestSelectSkipsLowRisk(t *testing.T) {
	got := triage.Select(claimsWithP(0.2), 0.3, &fakeSpender{daily: 30})
	assert.False(t, got[0].Checked)
}
