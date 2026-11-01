// Package score turns verdicts and evidence into claim and answer scores from
// 0 to 100.
package score

import (
	"math"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/triage"
)

// Claim returns the claim's score, or nil for a claim that was not checked.
//
//   - FRESH: 60 + 40 x R x A x G, where R is the recency of the newest dated
//     supporting source, A the highest authority among supporting sources and
//     G = min(1, supporting sources / 2).
//   - UNVERIFIABLE: 100 x (1 - P(changed)), limited to 21..59.
//   - STALE and WRONG: 20 x (1 - confidence).
func Claim(c report.Claim, now time.Time) *int {
	var s float64
	switch c.Verdict {
	case report.VerdictFresh:
		s = freshScore(c, now)
	case report.VerdictUnverifiable:
		s = min(max(100*(1-c.PChanged), 21), 59)
	case report.VerdictStale, report.VerdictWrong:
		s = 20 * (1 - min(max(c.Confidence, 0), 1))
	default:
		return nil
	}
	v := int(math.Round(s))
	return &v
}

func freshScore(c report.Claim, now time.Time) float64 {
	var (
		supporting int
		authority  float64
		newest     *time.Time
	)
	for _, e := range c.Evidence {
		if e.Stance != report.StanceSupports {
			continue
		}
		supporting++
		authority = max(authority, e.Authority)
		if e.PublishedDate != nil && (newest == nil || e.PublishedDate.After(*newest)) {
			newest = e.PublishedDate
		}
	}
	if supporting == 0 {
		return 60
	}

	recency := 0.5
	if hl, ok := triage.HalfLife(c.Category); !ok {
		recency = 1
	} else if newest != nil {
		age := max(now.Sub(*newest), 0)
		recency = math.Pow(0.5, float64(age)/float64(hl))
	}
	agreement := min(1, float64(supporting)/2)
	return 60 + 40*recency*authority*agreement
}

// Answer returns the mean of the scored claims weighted by P(changed), the
// plain mean when all weights are zero, or nil when no claim has a score.
func Answer(claims []report.Claim) *int {
	var sum, weights, plain float64
	n := 0
	for _, c := range claims {
		if c.Score == nil {
			continue
		}
		n++
		plain += float64(*c.Score)
		sum += float64(*c.Score) * c.PChanged
		weights += c.PChanged
	}
	if n == 0 {
		return nil
	}
	mean := plain / float64(n)
	if weights > 0 {
		mean = sum / weights
	}
	v := int(math.Round(mean))
	return &v
}
