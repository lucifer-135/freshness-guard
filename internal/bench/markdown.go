package bench

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// RunMeta describes one benchmark run.
type RunMeta struct {
	Name       string          `json:"name"`
	Dataset    string          `json:"dataset,omitempty"`
	Note       string          `json:"note,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at"`
	Settings   json.RawMessage `json:"settings,omitempty"` // "settings" of GET /freshness/status
}

func rate(r *float64, missing string) string {
	if r == nil {
		return "n/a (" + missing + ")"
	}
	return fmt.Sprintf("%.2f", *r)
}

// CuratedMarkdown renders the results of a curated run as a Markdown report.
func CuratedMarkdown(meta RunMeta, s Summary, outcomes []Outcome) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Curated benchmark: %s\n\n", meta.Name)
	b.WriteString("The target for precision is 0.8 (specification section 15.1). A target is not a result: the numbers below are what this run measured. ")
	b.WriteString("Each item's label was set from the source named in the dataset on the date named there. Who checked the labels is described in bench/README.md.\n\n")
	if meta.Dataset != "" {
		fmt.Fprintf(&b, "- Dataset: `%s`\n", meta.Dataset)
	}
	if !meta.StartedAt.IsZero() {
		fmt.Fprintf(&b, "- Run: %s to %s\n", meta.StartedAt.UTC().Format("2006-01-02 15:04 MST"), meta.FinishedAt.UTC().Format("2006-01-02 15:04 MST"))
	}
	if meta.Note != "" {
		fmt.Fprintf(&b, "- Setup: %s\n", meta.Note)
	}
	fmt.Fprintf(&b, "- Items: %d, of which %d errored (left out of the numbers below)\n\n", s.Items, s.Errors)

	b.WriteString("| Measure | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Precision | %s |\n", rate(s.Precision, "no outdated predictions"))
	fmt.Fprintf(&b, "| Recall | %s |\n", rate(s.Recall, "no outdated items"))
	fmt.Fprintf(&b, "| F1 | %s |\n", rate(s.F1, "needs precision and recall"))
	fmt.Fprintf(&b, "| True positives | %d |\n| False positives | %d |\n| False negatives | %d |\n| True negatives | %d |\n", s.TP, s.FP, s.FN, s.TN)
	fmt.Fprintf(&b, "| Items with no checked claim | %d |\n", s.NotChecked)
	fmt.Fprintf(&b, "| Predicted current but unverifiable | %d |\n", s.Unverifiable)
	fmt.Fprintf(&b, "| Outdated items where the current value matched | %d of %d |\n", s.ValueMatches, s.TP)
	fmt.Fprintf(&b, "| Tavily credits | %d (%.2f per item) |\n", s.CreditsTotal, s.CreditsPerItem)
	fmt.Fprintf(&b, "| Latency p50 / p95 | %d ms / %d ms |\n\n", s.LatencyP50MS, s.LatencyP95MS)

	b.WriteString("## By category\n\n| Category | TP | FP | FN | TN |\n|---|---|---|---|---|\n")
	cats := make([]report.Category, 0, len(s.ByCategory))
	for c := range s.ByCategory {
		cats = append(cats, c)
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i] < cats[j] })
	for _, c := range cats {
		n := s.ByCategory[c]
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n", c, n.TP, n.FP, n.FN, n.TN)
	}

	b.WriteString("\n## Items\n\n| Item | Category | Label | Predicted | Checked claims | Verdicts | Note |\n|---|---|---|---|---|---|---|\n")
	for _, o := range outcomes {
		verdicts := make([]string, len(o.Verdicts))
		for i, v := range o.Verdicts {
			verdicts[i] = string(v)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | %s | %s |\n", o.ItemID, o.Category, o.Label, o.Predicted, o.ClaimsChecked, strings.Join(verdicts, ", "), o.Error)
	}
	return b.String()
}
