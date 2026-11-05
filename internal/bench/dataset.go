// Package bench scores the freshness detector on a hand-labelled set of
// claims and measures what async mode adds to a request.
package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// Label says whether the claim in an item still holds.
type Label string

// Item labels.
const (
	LabelOutdated Label = "outdated"
	LabelCurrent  Label = "current"
)

// Item is one claim of the curated set with its hand-checked label.
type Item struct {
	ID            string          `json:"id"`
	Category      report.Category `json:"category"`
	Text          string          `json:"text"`
	Label         Label           `json:"label"`
	CurrentValues []string        `json:"current_values"` // accepted spellings of today's value
	ChangedOn     string          `json:"changed_on"`     // YYYY-MM-DD of the last change; "" when unknown
	Source        string          `json:"source"`         // https URL that shows today's value
	CheckedOn     string          `json:"checked_on"`     // YYYY-MM-DD the label was checked
	Note          string          `json:"note,omitempty"`
}

// timeSensitive lists the categories a curated item may use. "static" facts
// never change, so they cannot be outdated.
var timeSensitive = map[report.Category]bool{
	report.CategoryPricing: true, report.CategoryVersion: true, report.CategoryLeadership: true,
	report.CategoryLaw: true, report.CategoryStatistic: true, report.CategoryEvent: true, report.CategoryFeature: true,
}

const dateLayout = "2006-01-02"

// LoadItems reads and validates a curated set. Every error names the item.
func LoadItems(r io.Reader) ([]Item, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var items []Item
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("reading the curated set: %w", err)
	}
	seen := map[string]bool{}
	for _, it := range items {
		if err := it.validate(); err != nil {
			return nil, fmt.Errorf("item %q: %w", it.ID, err)
		}
		if seen[it.ID] {
			return nil, fmt.Errorf("item %q: the id is used twice", it.ID)
		}
		seen[it.ID] = true
	}
	return items, nil
}

func (it Item) validate() error {
	switch {
	case strings.TrimSpace(it.ID) == "":
		return fmt.Errorf("the id is empty")
	case strings.TrimSpace(it.Text) == "":
		return fmt.Errorf("the text is empty")
	case it.Label != LabelOutdated && it.Label != LabelCurrent:
		return fmt.Errorf("the label %q must be %q or %q", it.Label, LabelOutdated, LabelCurrent)
	case !timeSensitive[it.Category]:
		return fmt.Errorf("the category %q is not a time-sensitive category", it.Category)
	case len(it.CurrentValues) == 0:
		return fmt.Errorf("current_values is empty")
	case !strings.HasPrefix(it.Source, "https://"):
		return fmt.Errorf("the source must start with https://")
	}
	if _, err := time.Parse(dateLayout, it.CheckedOn); err != nil {
		return fmt.Errorf("checked_on must be a date like 2026-11-05")
	}
	if it.ChangedOn != "" {
		if _, err := time.Parse(dateLayout, it.ChangedOn); err != nil {
			return fmt.Errorf("changed_on must be empty or a date like 2026-11-05")
		}
	}
	return nil
}
