package mcp

import (
	"fmt"
	"strings"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// Summary is the text an agent reads after check_freshness: the score, the
// counts, one line per checked claim and the report ID. Claims that were not
// checked get no line, because they carry no verdict.
func Summary(rep report.Report) string {
	var b strings.Builder
	switch {
	case rep.TotalClaims == 0 && len(rep.Claims) == 0:
		b.WriteString("No factual claims were found in the text.\n")
	case rep.Score == nil:
		b.WriteString("No claims were checked, so the text has no freshness score.\n")
	default:
		stale, wrong := 0, 0
		for _, c := range rep.Claims {
			switch c.Verdict {
			case report.VerdictStale:
				stale++
			case report.VerdictWrong:
				wrong++
			}
		}
		// The verb agrees with the number checked: "1 of 4 claims was checked".
		fmt.Fprintf(&b, "Freshness score: %d out of 100. %d of %d %s %s checked. %d %s outdated and %d %s incorrect.\n",
			*rep.Score, rep.CheckedCount, rep.TotalClaims, plural(rep.TotalClaims, "claim", "claims"),
			plural(rep.CheckedCount, "was", "were"),
			stale, plural(stale, "is", "are"), wrong, plural(wrong, "is", "are"))
		for _, c := range rep.Claims {
			if line := claimLine(c); line != "" {
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}
	fmt.Fprintf(&b, "Report ID: %s.", rep.ID)
	return b.String()
}

// PendingText is the result of a check that outlasted the API's timeout.
func PendingText(id string) string {
	return fmt.Sprintf("The check is still running. Report ID: %s. Fetch the result later from GET /freshness/reports/%s.", id, id)
}

func claimLine(c report.Claim) string {
	if !c.Checked {
		return ""
	}
	quote := fmt.Sprintf("%q", c.Quote)
	switch c.Verdict {
	case report.VerdictStale:
		return "- Outdated: " + quote + valueSentence("The current value is", c) + source(c, report.StanceContradicts)
	case report.VerdictWrong:
		return "- Incorrect: " + quote + valueSentence("The correct value is", c) + source(c, report.StanceContradicts)
	case report.VerdictFresh:
		return "- Current: " + quote + source(c, report.StanceSupports)
	case report.VerdictUnverifiable:
		line := "- Could not be verified: " + quote
		if c.Rationale != "" {
			line += " " + c.Rationale
		}
		return line
	}
	return ""
}

func valueSentence(lead string, c report.Claim) string {
	if c.CurrentValue == "" {
		return ""
	}
	s := " " + lead + " " + c.CurrentValue
	if c.AsOf != "" {
		s += " (as of " + c.AsOf + ")"
	}
	return s + "."
}

// source names the evidence behind the verdict: the first item with the
// verdict's stance, otherwise the first item that has a URL.
func source(c report.Claim, stance report.Stance) string {
	for _, e := range c.Evidence {
		if e.URL != "" && e.Stance == stance {
			return " Source: " + e.URL
		}
	}
	for _, e := range c.Evidence {
		if e.URL != "" {
			return " Source: " + e.URL
		}
	}
	return ""
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
