package heal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/heal"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

func staleClaim() report.Claim {
	return report.Claim{ID: "c1", Quote: "Example Cloud Pro costs $20 per month", SpanStart: 0, SpanEnd: 37,
		Value: "$20 per month", Verdict: report.VerdictStale, CurrentValue: "$25 per month", AsOf: "2026-09-01",
		Evidence: []report.Evidence{
			{Domain: "blog.dev", Stance: report.StanceNeutral},
			{Domain: "example.com", Stance: report.StanceSupports},
		}}
}

func TestAnnotate(t *testing.T) {
	assert.Equal(t,
		"Example Cloud Pro costs $20 per month [outdated — now $25 per month as of 2026-09-01 (example.com)], and its current version is 4.2.",
		heal.Annotate(answer, []report.Claim{staleClaim()}))
}

func TestAnnotateWrongWithoutDateOrSource(t *testing.T) {
	c := staleClaim()
	c.Verdict, c.AsOf, c.Evidence = report.VerdictWrong, "", nil
	assert.Equal(t,
		"Example Cloud Pro costs $20 per month [incorrect — now $25 per month], and its current version is 4.2.",
		heal.Annotate(answer, []report.Claim{c}))
}

func TestAnnotateTwoClaimsKeepsOffsets(t *testing.T) {
	version := report.Claim{ID: "c2", Quote: "4.2", SpanStart: 66, SpanEnd: 69, Value: "4.2",
		Verdict: report.VerdictStale, CurrentValue: "4.3"}
	assert.Equal(t,
		"Example Cloud Pro costs $20 per month [outdated — now $25 per month as of 2026-09-01 (example.com)], and its current version is 4.2 [outdated — now 4.3].",
		heal.Annotate(answer, []report.Claim{staleClaim(), version}))
}
