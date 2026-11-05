package bench

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

func TestCuratedMarkdown(t *testing.T) {
	meta := RunMeta{
		Name: "curated", Dataset: "bench/curated.json", Note: "groq/openai/gpt-oss-120b judge",
		StartedAt: time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 11, 8, 10, 20, 0, 0, time.UTC),
	}
	var os []Outcome
	os = append(os, repeat(8, outcome(LabelOutdated, PredictedOutdated, report.CategoryPricing))...)
	os = append(os, repeat(2, outcome(LabelCurrent, PredictedOutdated, report.CategoryPricing))...)
	os = append(os, repeat(2, outcome(LabelOutdated, PredictedCurrent, report.CategoryVersion))...)
	os = append(os, repeat(6, outcome(LabelCurrent, PredictedCurrent, report.CategoryVersion))...)
	os[0].ItemID = "pricing-1"

	md := CuratedMarkdown(meta, Summarize(os), os)
	assert.Contains(t, md, "| Precision | 0.80 |")
	assert.Contains(t, md, "| Recall | 0.80 |")
	assert.Contains(t, md, "| F1 | 0.80 |")
	assert.Contains(t, md, "bench/curated.json")
	assert.Contains(t, md, "2026-11-08")
	assert.Contains(t, md, "| pricing |")
	assert.Contains(t, md, "| version |")
	assert.Contains(t, md, "pricing-1")
	assert.Contains(t, md, "target")
	assert.Contains(t, md, "0.8")
}

func TestCuratedMarkdownPrintsNotAvailableForMissingRates(t *testing.T) {
	os := []Outcome{outcome(LabelCurrent, PredictedCurrent, report.CategoryPricing)}
	md := CuratedMarkdown(RunMeta{Name: "curated"}, Summarize(os), os)
	assert.Contains(t, md, "n/a (no outdated predictions)")
	assert.Contains(t, md, "n/a (no outdated items)")
	assert.NotContains(t, md, "NaN")
}
