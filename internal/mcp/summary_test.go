package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

func intPtr(n int) *int { return &n }

func claim(quote string, verdict report.Verdict, checked bool) report.Claim {
	return report.Claim{Quote: quote, Verdict: verdict, Checked: checked}
}

func TestSummary(t *testing.T) {
	stale := claim("Node.js 22 is the current LTS release.", report.VerdictStale, true)
	stale.CurrentValue = "Node.js 24"
	stale.AsOf = "2025-10-28"
	stale.Evidence = []report.Evidence{
		{URL: "https://example.com/neutral", Stance: report.StanceNeutral},
		{URL: "https://nodejs.org/en/about/previous-releases", Stance: report.StanceContradicts},
	}
	fresh := claim("ChatGPT Plus costs $20 per month.", report.VerdictFresh, true)
	fresh.Evidence = []report.Evidence{{URL: "https://openai.com/chatgpt/pricing", Stance: report.StanceSupports}}
	unverifiable := claim("Example Corp has 400 employees.", report.VerdictUnverifiable, true)
	unverifiable.Rationale = "No dated source mentions the claim."
	notChecked := claim("Water boils at 100 degrees Celsius.", report.VerdictNotChecked, false)

	wrong := claim("Go 1.30 is the latest release.", report.VerdictWrong, true)
	wrong.CurrentValue = "Go 1.27"
	wrong.AsOf = "2026-08-12"
	wrong.Evidence = []report.Evidence{{URL: "https://go.dev/doc/devel/release", Stance: report.StanceContradicts}}

	bare := claim("Example Cloud Pro costs $15 per month.", report.VerdictStale, true)
	noURL := claim("Example DB 3 is current.", report.VerdictFresh, true)
	noURL.Evidence = []report.Evidence{{Title: "No link"}}
	firstURL := claim("Example App 2 is current.", report.VerdictFresh, true)
	firstURL.Evidence = []report.Evidence{{URL: "https://example.com/first", Stance: report.StanceNeutral}}

	cases := []struct {
		name string
		rep  report.Report
		want string
	}{
		{
			name: "mixed verdicts",
			rep: report.Report{ID: "r1", Status: report.StatusComplete, Score: intPtr(62), StaleCount: 1, CheckedCount: 3, TotalClaims: 4,
				Claims: []report.Claim{stale, fresh, unverifiable, notChecked}},
			want: "Freshness score: 62 out of 100. 3 of 4 claims were checked. 1 is outdated and 0 are incorrect.\n" +
				"- Outdated: \"Node.js 22 is the current LTS release.\" The current value is Node.js 24 (as of 2025-10-28). Source: https://nodejs.org/en/about/previous-releases\n" +
				"- Current: \"ChatGPT Plus costs $20 per month.\" Source: https://openai.com/chatgpt/pricing\n" +
				"- Could not be verified: \"Example Corp has 400 employees.\" No dated source mentions the claim.\n" +
				"Report ID: r1.",
		},
		{
			name: "incorrect claim",
			rep: report.Report{ID: "r1", Status: report.StatusComplete, Score: intPtr(0), CheckedCount: 1, TotalClaims: 1,
				Claims: []report.Claim{wrong}},
			want: "Freshness score: 0 out of 100. 1 of 1 claim was checked. 0 are outdated and 1 is incorrect.\n" +
				"- Incorrect: \"Go 1.30 is the latest release.\" The correct value is Go 1.27 (as of 2026-08-12). Source: https://go.dev/doc/devel/release\n" +
				"Report ID: r1.",
		},
		{
			name: "missing value, date and source",
			rep: report.Report{ID: "r1", Status: report.StatusComplete, Score: intPtr(40), StaleCount: 1, CheckedCount: 1, TotalClaims: 4,
				Claims: []report.Claim{bare}},
			want: "Freshness score: 40 out of 100. 1 of 4 claims was checked. 1 is outdated and 0 are incorrect.\n" +
				"- Outdated: \"Example Cloud Pro costs $15 per month.\"\n" +
				"Report ID: r1.",
		},
		{
			name: "source falls back to the first evidence with a URL",
			rep: report.Report{ID: "r1", Status: report.StatusComplete, Score: intPtr(90), CheckedCount: 2, TotalClaims: 2,
				Claims: []report.Claim{noURL, firstURL}},
			want: "Freshness score: 90 out of 100. 2 of 2 claims were checked. 0 are outdated and 0 are incorrect.\n" +
				"- Current: \"Example DB 3 is current.\"\n" +
				"- Current: \"Example App 2 is current.\" Source: https://example.com/first\n" +
				"Report ID: r1.",
		},
		{
			name: "stale and wrong counted separately",
			rep: report.Report{ID: "r1", Status: report.StatusComplete, Score: intPtr(10), StaleCount: 2, CheckedCount: 2, TotalClaims: 2,
				Claims: []report.Claim{bare, wrong}},
			want: "Freshness score: 10 out of 100. 2 of 2 claims were checked. 1 is outdated and 1 is incorrect.\n" +
				"- Outdated: \"Example Cloud Pro costs $15 per month.\"\n" +
				"- Incorrect: \"Go 1.30 is the latest release.\" The correct value is Go 1.27 (as of 2026-08-12). Source: https://go.dev/doc/devel/release\n" +
				"Report ID: r1.",
		},
		{
			name: "no claims",
			rep:  report.Report{ID: "r1", Status: report.StatusComplete},
			want: "No factual claims were found in the text.\nReport ID: r1.",
		},
		{
			name: "claims but none checked",
			rep:  report.Report{ID: "r1", Status: report.StatusComplete, TotalClaims: 1, Claims: []report.Claim{notChecked}},
			want: "No claims were checked, so the text has no freshness score.\nReport ID: r1.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Summary(tc.rep))
		})
	}
}

func TestPendingText(t *testing.T) {
	assert.Equal(t, "The check is still running. Report ID: r1. Fetch the result later from GET /freshness/reports/r1.", PendingText("r1"))
}
