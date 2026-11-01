// Package triage estimates how likely each claim is to be outdated and picks
// the claims worth a Tavily search within the daily credit budget. It uses no
// LLM.
package triage

import (
	"math"
	"sort"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

const day = 24 * time.Hour

// defaultModelAge is used for models without a configured knowledge cutoff.
const defaultModelAge = 365 * day

var halfLives = map[report.Category]time.Duration{
	report.CategoryEvent:      30 * day,
	report.CategoryPricing:    60 * day,
	report.CategoryVersion:    90 * day,
	report.CategoryStatistic:  120 * day,
	report.CategoryFeature:    120 * day,
	report.CategoryLaw:        180 * day,
	report.CategoryLeadership: 365 * day,
}

// HalfLife returns the time after which a fact in category c has changed with
// a probability of one half. It returns false for facts that never change.
func HalfLife(c report.Category) (time.Duration, bool) {
	hl, ok := halfLives[c]
	return hl, ok
}

// PChanged returns 1 - 0.5^(age/halfLife). A temporal marker such as
// "currently" doubles the age. Categories without a half-life return 0.
func PChanged(c report.Category, age time.Duration, marker bool) float64 {
	hl, ok := HalfLife(c)
	if !ok || age <= 0 {
		return 0
	}
	if marker {
		age *= 2
	}
	return 1 - math.Pow(0.5, float64(age)/float64(hl))
}

// ModelAge returns the time since the model's knowledge cutoff, or 365 days
// when no cutoff is configured. It is never negative.
func ModelAge(model string, cutoffs map[string]time.Time, now time.Time) time.Duration {
	cutoff, ok := cutoffs[model]
	if !ok {
		return defaultModelAge
	}
	return max(now.Sub(cutoff), 0)
}

// Threshold returns the minimum P(changed) for a claim to be checked. It stays
// at base until half the daily budget is used, then rises linearly to 1 at the
// full budget. With no budget left it is +Inf.
func Threshold(base float64, used, budget int) float64 {
	if budget <= 0 || used >= budget {
		return math.Inf(1)
	}
	f := float64(used) / float64(budget)
	if f <= 0.5 {
		return base
	}
	return base + (1-base)*(f-0.5)/0.5
}

// Spender is the daily Tavily credit budget.
type Spender interface {
	TrySpend(n int) bool
	Used() int
	Daily() int
}

// Select marks the claims to check. Claims that are already checked (verdict
// cache hits) stay checked and cost nothing. The others are considered in
// descending P(changed) order; each needs P(changed) at or above the current
// threshold and one credit. The result keeps the input order.
func Select(claims []report.Claim, base float64, budget Spender) []report.Claim {
	out := append([]report.Claim(nil), claims...)
	order := make([]int, 0, len(out))
	for i, c := range out {
		if !c.Checked {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return out[order[a]].PChanged > out[order[b]].PChanged })
	for _, i := range order {
		if out[i].PChanged >= Threshold(base, budget.Used(), budget.Daily()) && budget.TrySpend(1) {
			out[i].Checked = true
		}
	}
	return out
}
