package bench

import (
	"math"
	"sort"
	"strings"

	"github.com/lucifer-135/freshness-guard/internal/facts"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

// Prediction is what the detector said about an item's text.
type Prediction string

// Predictions. A text is outdated when any checked claim is STALE or WRONG.
const (
	PredictedOutdated Prediction = "outdated"
	PredictedCurrent  Prediction = "current"
	PredictedError    Prediction = "error"
)

// Outcome is the result of checking one item.
type Outcome struct {
	ItemID        string           `json:"item_id"`
	Category      report.Category  `json:"category"`
	Label         Label            `json:"label"`
	Predicted     Prediction       `json:"predicted"`
	ReportID      string           `json:"report_id"`
	Status        string           `json:"status"`
	Error         string           `json:"error,omitempty"`
	ClaimsFound   int              `json:"claims_found"`
	ClaimsChecked int              `json:"claims_checked"`
	Verdicts      []report.Verdict `json:"verdicts"`
	// Unverifiable: predicted current while a checked claim was UNVERIFIABLE,
	// so "current" may only mean "nothing found".
	Unverifiable bool `json:"unverifiable"`
	// ValueMatched: predicted outdated and an outdated claim's current value
	// contains one of the item's accepted values.
	ValueMatched bool  `json:"value_matched"`
	CreditsUsed  int   `json:"credits_used"`
	LatencyMS    int64 `json:"latency_ms"`
}

// Classify turns a finished report into an outcome for item.
func Classify(item Item, rep report.Report) Outcome {
	o := Outcome{
		ItemID: item.ID, Category: item.Category, Label: item.Label,
		ReportID: rep.ID, Status: string(rep.Status),
		ClaimsFound: len(rep.Claims), CreditsUsed: rep.CreditsUsed, LatencyMS: rep.LatencyMS,
	}
	switch rep.Status {
	case report.StatusComplete:
	case report.StatusError:
		o.Predicted = PredictedError
		o.Error = rep.Reason
		if o.Error == "" {
			o.Error = "error"
		}
		return o
	default:
		o.Predicted = PredictedError
		o.Error = string(rep.Status)
		return o
	}

	outdated, unverifiable := false, false
	for _, c := range rep.Claims {
		if !c.Checked {
			continue
		}
		o.ClaimsChecked++
		o.Verdicts = append(o.Verdicts, c.Verdict)
		switch c.Verdict {
		case report.VerdictStale, report.VerdictWrong:
			outdated = true
			if mentions(c.CurrentValue, item.CurrentValues) {
				o.ValueMatched = true
			}
		case report.VerdictUnverifiable:
			unverifiable = true
		}
	}
	if outdated {
		o.Predicted = PredictedOutdated
		return o
	}
	o.Predicted = PredictedCurrent
	o.Unverifiable = unverifiable
	return o
}

// mentions reports whether value contains one of the accepted values as whole
// words, after both are normalized the way the fact cache normalizes values.
func mentions(value string, accepted []string) bool {
	padded := " " + facts.NormalizeValue(value) + " "
	for _, a := range accepted {
		if n := facts.NormalizeValue(a); n != "" && strings.Contains(padded, " "+n+" ") {
			return true
		}
	}
	return false
}

// Counts is a confusion matrix; "positive" means outdated.
type Counts struct{ TP, FP, FN, TN int }

// Summary is the scored result of a run. Items that errored are counted in
// Errors and left out of every other number.
type Summary struct {
	Items, Errors int
	Counts
	Precision, Recall, F1      *float64 // nil when the denominator is 0
	Unverifiable, NotChecked   int
	ValueMatches               int
	CreditsTotal               int
	CreditsPerItem             float64
	LatencyP50MS, LatencyP95MS int64
	ByCategory                 map[report.Category]Counts
}

// Summarize scores outcomes.
func Summarize(outcomes []Outcome) Summary {
	s := Summary{Items: len(outcomes), ByCategory: map[report.Category]Counts{}}
	var latencies []int64
	scored := 0
	for _, o := range outcomes {
		if o.Predicted == PredictedError {
			s.Errors++
			continue
		}
		scored++
		c := s.ByCategory[o.Category]
		switch {
		case o.Label == LabelOutdated && o.Predicted == PredictedOutdated:
			s.TP++
			c.TP++
			if o.ValueMatched {
				s.ValueMatches++
			}
		case o.Label == LabelCurrent && o.Predicted == PredictedOutdated:
			s.FP++
			c.FP++
		case o.Label == LabelOutdated && o.Predicted == PredictedCurrent:
			s.FN++
			c.FN++
		default:
			s.TN++
			c.TN++
		}
		s.ByCategory[o.Category] = c
		if o.Unverifiable {
			s.Unverifiable++
		}
		if o.ClaimsChecked == 0 {
			s.NotChecked++
		}
		s.CreditsTotal += o.CreditsUsed
		latencies = append(latencies, o.LatencyMS)
	}
	if scored > 0 {
		s.CreditsPerItem = float64(s.CreditsTotal) / float64(scored)
	}
	s.Precision = ratio(s.TP, s.TP+s.FP)
	s.Recall = ratio(s.TP, s.TP+s.FN)
	if s.Precision != nil && s.Recall != nil && *s.Precision+*s.Recall > 0 {
		f1 := 2 * *s.Precision * *s.Recall / (*s.Precision + *s.Recall)
		s.F1 = &f1
	}
	s.LatencyP50MS = Percentile(latencies, 50)
	s.LatencyP95MS = Percentile(latencies, 95)
	return s
}

func ratio(num, den int) *float64 {
	if den == 0 {
		return nil
	}
	r := float64(num) / float64(den)
	return &r
}

// Percentile returns the nearest-rank percentile of values (0 for none). The
// input is not reordered.
func Percentile(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
