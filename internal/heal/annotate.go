package heal

import (
	"sort"
	"strings"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// Annotate inserts a note after each correction's span, for example
// " [outdated — now $25 per month as of 2026-09-01 (example.com)]". It is the
// fallback when an edit is rejected or cannot be made (spec 8.4).
func Annotate(original string, corrections []report.Claim) string {
	ordered := append([]report.Claim(nil), corrections...)
	// From the last span to the first, so earlier byte offsets stay valid.
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].SpanEnd > ordered[j].SpanEnd })

	out := original
	for _, c := range ordered {
		if c.SpanStart < 0 || c.SpanEnd < c.SpanStart {
			continue
		}
		at := byteOffset(original, c.SpanEnd)
		out = out[:at] + note(c) + out[at:]
	}
	return out
}

func note(c report.Claim) string {
	label := "outdated"
	if c.Verdict == report.VerdictWrong {
		label = "incorrect"
	}
	var b strings.Builder
	b.WriteString(" [" + label + " — now " + c.CurrentValue)
	if c.AsOf != "" {
		b.WriteString(" as of " + c.AsOf)
	}
	for _, e := range c.Evidence {
		if e.Stance == report.StanceSupports && e.Domain != "" {
			b.WriteString(" (" + e.Domain + ")")
			break
		}
	}
	b.WriteString("]")
	return b.String()
}
