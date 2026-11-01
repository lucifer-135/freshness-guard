package score_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/score"
)

var now = time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)

func ev(stance report.Stance, authority float64, published *time.Time) report.Evidence {
	return report.Evidence{Stance: stance, Authority: authority, PublishedDate: published}
}

func at(t time.Time) *time.Time { return &t }

func assertScore(t *testing.T, want int, got *int) {
	t.Helper()
	require.NotNil(t, got)
	assert.Equal(t, want, *got)
}

func fresh(evidence ...report.Evidence) report.Claim {
	return report.Claim{Category: report.CategoryPricing, Verdict: report.VerdictFresh, Checked: true, Evidence: evidence}
}

func TestFreshAllFactorsMax(t *testing.T) {
	c := fresh(ev(report.StanceSupports, 1, at(now)), ev(report.StanceSupports, 1, at(now)))
	assertScore(t, 100, score.Claim(c, now))
}

func TestFreshHalfLifeOldLowAuthoritySingleSource(t *testing.T) {
	c := fresh(ev(report.StanceSupports, 0.5, at(now.Add(-60*24*time.Hour))))
	assertScore(t, 65, score.Claim(c, now))
}

func TestFreshWithoutDatedEvidence(t *testing.T) {
	c := fresh(ev(report.StanceSupports, 1, nil), ev(report.StanceSupports, 1, nil))
	assertScore(t, 80, score.Claim(c, now))
}

func TestFreshWithoutSupport(t *testing.T) {
	c := fresh(ev(report.StanceNeutral, 1, at(now)))
	assertScore(t, 60, score.Claim(c, now))
}

func TestFreshStaticCategoryHasFullRecency(t *testing.T) {
	c := fresh(ev(report.StanceSupports, 1, at(now.Add(-1000*24*time.Hour))), ev(report.StanceSupports, 1, nil))
	c.Category = report.CategoryStatic
	assertScore(t, 100, score.Claim(c, now))
}

func TestUnverifiableClamped(t *testing.T) {
	for p, want := range map[float64]int{0.9: 21, 0.5: 50, 0.1: 59} {
		c := report.Claim{Verdict: report.VerdictUnverifiable, PChanged: p, Checked: true}
		assertScore(t, want, score.Claim(c, now))
	}
}

func TestStaleAndWrong(t *testing.T) {
	assertScore(t, 2, score.Claim(report.Claim{Verdict: report.VerdictStale, Confidence: 0.9}, now))
	assertScore(t, 20, score.Claim(report.Claim{Verdict: report.VerdictWrong, Confidence: 0}, now))
	assertScore(t, 0, score.Claim(report.Claim{Verdict: report.VerdictStale, Confidence: 1.5}, now))
}

func TestNotCheckedHasNoScore(t *testing.T) {
	assert.Nil(t, score.Claim(report.Claim{Verdict: report.VerdictNotChecked, PChanged: 0.2}, now))
}

func intPtr(v int) *int { return &v }

func TestAnswerWeightedByPChanged(t *testing.T) {
	claims := []report.Claim{
		{Score: intPtr(100), PChanged: 0.9},
		{Score: intPtr(0), PChanged: 0.3},
		{Score: nil, PChanged: 0.1},
	}
	assertScore(t, 75, score.Answer(claims))
}

func TestAnswerPlainMeanWhenWeightsAreZero(t *testing.T) {
	claims := []report.Claim{{Score: intPtr(100)}, {Score: intPtr(50)}}
	assertScore(t, 75, score.Answer(claims))
}

func TestAnswerNilWithoutScores(t *testing.T) {
	assert.Nil(t, score.Answer([]report.Claim{{PChanged: 0.5}}))
	assert.Nil(t, score.Answer(nil))
}
