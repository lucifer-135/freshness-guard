package bench

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validItem = `{"id":"%s","category":"version","text":"Node.js 22 is the current LTS release.","label":"outdated",
 "current_values":["24"],"changed_on":"2025-10-28","source":"https://nodejs.org/en/about/previous-releases","checked_on":"2026-11-05"}`

func twoItems() string {
	return "[" + strings.Replace(validItem, "%s", "a", 1) + "," +
		strings.Replace(strings.Replace(validItem, "%s", "b", 1), "Node.js 22 is", "Go 1.20 is", 1) + "]"
}

func TestLoadItemsReadsValidFile(t *testing.T) {
	items, err := LoadItems(strings.NewReader(twoItems()))
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "a", items[0].ID)
	assert.Equal(t, LabelOutdated, items[0].Label)
	assert.Equal(t, []string{"24"}, items[0].CurrentValues)
}

func TestLoadItemsRefusesBadItems(t *testing.T) {
	item := func(id, replaceOld, replaceNew string) string {
		return strings.Replace(strings.Replace(validItem, "%s", id, 1), replaceOld, replaceNew, 1)
	}
	cases := []struct {
		name, json, wantID string
	}{
		{"duplicate id", "[" + item("a", "", "") + "," + item("a", "Node.js 22", "Go 1.20") + "]", "a"},
		{"bad label", "[" + item("x1", `"outdated"`, `"stale"`) + "]", "x1"},
		{"unknown category", "[" + item("x2", `"version"`, `"weather"`) + "]", "x2"},
		{"static category", "[" + item("x2b", `"version"`, `"static"`) + "]", "x2b"},
		{"empty text", "[" + item("x3", `"Node.js 22 is the current LTS release."`, `"  "`) + "]", "x3"},
		{"no current values", "[" + item("x4", `["24"]`, `[]`) + "]", "x4"},
		{"source not https", "[" + item("x5", "https://nodejs.org", "http://nodejs.org") + "]", "x5"},
		{"checked_on not a date", "[" + item("x6", "2026-11-05", "5 Nov") + "]", "x6"},
		{"changed_on not a date", "[" + item("x7", "2025-10-28", "October") + "]", "x7"},
		{"unknown field", "[" + item("x8", `"label"`, `"extra":1,"label"`) + "]", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadItems(strings.NewReader(tc.json))
			require.Error(t, err)
			if tc.wantID != "" {
				assert.Contains(t, err.Error(), tc.wantID)
			}
		})
	}
}

func TestLoadItemsAllowsEmptyChangedOn(t *testing.T) {
	j := "[" + strings.Replace(strings.Replace(validItem, "%s", "a", 1), `"changed_on":"2025-10-28"`, `"changed_on":""`, 1) + "]"
	_, err := LoadItems(strings.NewReader(j))
	require.NoError(t, err)
}

func TestCuratedDatasetIsValid(t *testing.T) {
	f, err := os.Open("../../bench/curated.json")
	require.NoError(t, err)
	defer f.Close()
	items, err := LoadItems(f)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(items), 45)
	assert.LessOrEqual(t, len(items), 60)

	labels := map[Label]int{}
	categories := map[string]int{}
	texts := map[string]string{}
	for _, it := range items {
		labels[it.Label]++
		categories[string(it.Category)]++
		if other, dup := texts[it.Text]; dup {
			t.Errorf("items %q and %q share the same text", other, it.ID)
		}
		texts[it.Text] = it.ID

		checked, err := time.Parse(dateLayout, it.CheckedOn)
		require.NoError(t, err, it.ID)
		assert.False(t, checked.Before(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)), "%s: checked_on is before 2026-11-01", it.ID)

		if it.Label == LabelOutdated {
			require.NotEmpty(t, it.ChangedOn, "%s: an outdated item needs changed_on", it.ID)
			changed, err := time.Parse(dateLayout, it.ChangedOn)
			require.NoError(t, err, it.ID)
			assert.False(t, changed.Before(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), "%s: changed_on is before 2025-01-01", it.ID)
			assert.False(t, changed.After(checked), "%s: changed_on is after checked_on", it.ID)
		}
	}
	assert.GreaterOrEqual(t, labels[LabelOutdated], 20)
	assert.GreaterOrEqual(t, labels[LabelCurrent], 20)
	for _, c := range []string{"pricing", "version", "leadership"} {
		assert.GreaterOrEqual(t, categories[c], 8, "category %s", c)
	}
}
