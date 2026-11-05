package bench

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

func rep(status report.Status, claims ...report.Claim) report.Report {
	checked := 0
	for _, c := range claims {
		if c.Checked {
			checked++
		}
	}
	return report.Report{ID: "r1", Status: status, TotalClaims: len(claims), CheckedCount: checked, Claims: claims, CreditsUsed: 2, LatencyMS: 900}
}

func checkedClaim(v report.Verdict, current string) report.Claim {
	return report.Claim{Checked: true, Verdict: v, CurrentValue: current}
}

func TestClassify(t *testing.T) {
	item := Item{ID: "n1", Category: report.CategoryVersion, Label: LabelOutdated, CurrentValues: []string{"24"}}

	stale := Classify(item, rep(report.StatusComplete, checkedClaim(report.VerdictStale, "Node.js 24 LTS")))
	assert.Equal(t, PredictedOutdated, stale.Predicted)
	assert.True(t, stale.ValueMatched)
	assert.False(t, stale.Unverifiable)
	assert.Equal(t, 2, stale.CreditsUsed)
	assert.Equal(t, int64(900), stale.LatencyMS)

	wrongValue := Classify(item, rep(report.StatusComplete, checkedClaim(report.VerdictStale, "Node.js 22")))
	assert.Equal(t, PredictedOutdated, wrongValue.Predicted)
	assert.False(t, wrongValue.ValueMatched)

	wrong := Classify(item, rep(report.StatusComplete, checkedClaim(report.VerdictWrong, "24")))
	assert.Equal(t, PredictedOutdated, wrong.Predicted)

	fresh := Classify(item, rep(report.StatusComplete, checkedClaim(report.VerdictFresh, "")))
	assert.Equal(t, PredictedCurrent, fresh.Predicted)
	assert.False(t, fresh.ValueMatched)

	unverifiable := Classify(item, rep(report.StatusComplete, checkedClaim(report.VerdictUnverifiable, "")))
	assert.Equal(t, PredictedCurrent, unverifiable.Predicted)
	assert.True(t, unverifiable.Unverifiable)

	notChecked := Classify(item, rep(report.StatusComplete, report.Claim{Verdict: report.VerdictNotChecked}))
	assert.Equal(t, PredictedCurrent, notChecked.Predicted)
	assert.Equal(t, 0, notChecked.ClaimsChecked)
	assert.Equal(t, 1, notChecked.ClaimsFound)

	mixed := Classify(item, rep(report.StatusComplete, checkedClaim(report.VerdictFresh, ""), checkedClaim(report.VerdictStale, "24")))
	assert.Equal(t, PredictedOutdated, mixed.Predicted, "one outdated claim makes the text outdated")
	assert.Equal(t, []report.Verdict{report.VerdictFresh, report.VerdictStale}, mixed.Verdicts)

	pending := Classify(item, report.Report{ID: "r2", Status: report.StatusPending})
	assert.Equal(t, PredictedError, pending.Predicted)
	assert.Equal(t, "pending", pending.Error)

	failed := Classify(item, report.Report{ID: "r3", Status: report.StatusError, Reason: "queue full"})
	assert.Equal(t, PredictedError, failed.Predicted)
	assert.Equal(t, "queue full", failed.Error)
}

func outcome(label Label, predicted Prediction, cat report.Category) Outcome {
	return Outcome{Label: label, Predicted: predicted, Category: cat, ClaimsChecked: 1, CreditsUsed: 1, LatencyMS: 100}
}

func repeat(n int, o Outcome) []Outcome {
	out := make([]Outcome, n)
	for i := range out {
		out[i] = o
	}
	return out
}

func TestSummarize(t *testing.T) {
	var os []Outcome
	os = append(os, repeat(8, outcome(LabelOutdated, PredictedOutdated, report.CategoryPricing))...)
	os = append(os, repeat(2, outcome(LabelCurrent, PredictedOutdated, report.CategoryPricing))...)
	os = append(os, repeat(2, outcome(LabelOutdated, PredictedCurrent, report.CategoryVersion))...)
	os = append(os, repeat(6, outcome(LabelCurrent, PredictedCurrent, report.CategoryVersion))...)
	os = append(os, repeat(2, Outcome{Label: LabelOutdated, Predicted: PredictedError, Category: report.CategoryLaw, CreditsUsed: 9, LatencyMS: 99999})...)

	s := Summarize(os)
	assert.Equal(t, 20, s.Items)
	assert.Equal(t, 2, s.Errors)
	assert.Equal(t, Counts{TP: 8, FP: 2, FN: 2, TN: 6}, s.Counts)
	require.NotNil(t, s.Precision)
	require.NotNil(t, s.Recall)
	require.NotNil(t, s.F1)
	assert.InDelta(t, 0.8, *s.Precision, 1e-9)
	assert.InDelta(t, 0.8, *s.Recall, 1e-9)
	assert.InDelta(t, 0.8, *s.F1, 1e-9)
	assert.Equal(t, 18, s.CreditsTotal, "errors do not count in credits")
	assert.InDelta(t, 1.0, s.CreditsPerItem, 1e-9)
	assert.Equal(t, int64(100), s.LatencyP95MS, "errors do not count in latency")
	assert.Equal(t, Counts{TP: 8, FP: 2}, s.ByCategory[report.CategoryPricing])
	assert.Equal(t, Counts{FN: 2, TN: 6}, s.ByCategory[report.CategoryVersion])
	assert.NotContains(t, s.ByCategory, report.CategoryLaw)
}

func TestSummarizeCountsUncheckedOutdatedItemAsMissed(t *testing.T) {
	s := Summarize([]Outcome{{Label: LabelOutdated, Predicted: PredictedCurrent, ClaimsChecked: 0, ClaimsFound: 0, Category: report.CategoryPricing}})
	assert.Equal(t, 1, s.FN)
	assert.Equal(t, 1, s.NotChecked)
}

func TestSummarizeCountsUnverifiableAndValueMatches(t *testing.T) {
	s := Summarize([]Outcome{
		{Label: LabelCurrent, Predicted: PredictedCurrent, ClaimsChecked: 1, Unverifiable: true},
		{Label: LabelOutdated, Predicted: PredictedOutdated, ClaimsChecked: 1, ValueMatched: true},
		{Label: LabelOutdated, Predicted: PredictedOutdated, ClaimsChecked: 1},
	})
	assert.Equal(t, 1, s.Unverifiable)
	assert.Equal(t, 1, s.ValueMatches)
}

func TestSummarizeLeavesRatesNilWithoutDenominator(t *testing.T) {
	s := Summarize([]Outcome{outcome(LabelCurrent, PredictedCurrent, report.CategoryPricing)})
	assert.Nil(t, s.Precision, "no outdated predictions")
	assert.Nil(t, s.Recall, "no outdated items")
	assert.Nil(t, s.F1)
	assert.Equal(t, Counts{TN: 1}, s.Counts)
}

func TestPercentile(t *testing.T) {
	values := make([]int64, 100)
	for i := range values {
		values[i] = int64(100 - i) // descending, to show the input is not reordered
	}
	assert.Equal(t, int64(50), Percentile(values, 50))
	assert.Equal(t, int64(95), Percentile(values, 95))
	assert.Equal(t, int64(99), Percentile(values, 99))
	assert.Equal(t, int64(100), values[0])
	assert.Equal(t, int64(0), Percentile(nil, 50))
}
