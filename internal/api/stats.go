package api

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/store"
)

// Stats reads the numbers the dashboard shows; *store.SQLite implements it.
type Stats interface {
	Leaderboard(ctx context.Context, since time.Time) ([]store.LeaderboardCell, error)
	Summary(ctx context.Context, since time.Time) (store.SummaryCounts, error)
}

const (
	// minSample is the number of checked claims below which a rate is marked
	// as low confidence (spec 9.4).
	minSample = 10
	// summaryWindow is the period of the dashboard's summary row.
	summaryWindow = 24 * time.Hour
)

type counts struct {
	Checked   int `json:"checked"`
	Outdated  int `json:"outdated"`
	Incorrect int `json:"incorrect"`
}

func (c *counts) add(o counts) {
	c.Checked += o.Checked
	c.Outdated += o.Outdated
	c.Incorrect += o.Incorrect
}

type leaderboardRow struct {
	Model    string            `json:"model"`
	Provider string            `json:"provider"`
	Total    counts            `json:"total"`
	Cells    map[string]counts `json:"cells"`
}

func (h *handler) leaderboard(w http.ResponseWriter, r *http.Request) {
	days := 0
	switch r.URL.Query().Get("window") {
	case "", "7d":
		days = 7
	case "30d":
		days = 30
	default:
		writeError(w, http.StatusBadRequest, "The window must be 7d or 30d.")
		return
	}
	since := h.Now().Add(-time.Duration(days) * 24 * time.Hour).UTC()
	cells, err := h.Stats.Leaderboard(r.Context(), since)
	if err != nil {
		h.Logger.Error("The leaderboard could not be loaded.", "error", err)
		writeError(w, http.StatusInternalServerError, "The leaderboard could not be loaded.")
		return
	}

	indexOf := map[string]int{}
	rows := []leaderboardRow{}
	for _, c := range cells {
		i, ok := indexOf[c.Model]
		if !ok {
			i = len(rows)
			indexOf[c.Model] = i
			rows = append(rows, leaderboardRow{Model: c.Model, Provider: c.Provider, Cells: map[string]counts{}})
		}
		n := counts{Checked: c.Checked, Outdated: c.Outdated, Incorrect: c.Incorrect}
		rows[i].Cells[string(c.Category)] = n
		rows[i].Total.add(n)
	}
	slices.SortFunc(rows, func(a, b leaderboardRow) int {
		return cmp.Or(cmp.Compare(a.Provider, b.Provider), cmp.Compare(b.Total.Checked, a.Total.Checked), cmp.Compare(a.Model, b.Model))
	})
	writeJSON(w, http.StatusOK, map[string]any{"window_days": days, "since": since, "min_sample": minSample, "rows": rows})
}

func (h *handler) summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.Stats.Summary(r.Context(), h.Now().Add(-summaryWindow).UTC())
	if err != nil {
		h.Logger.Error("The summary could not be loaded.", "error", err)
		writeError(w, http.StatusInternalServerError, "The summary could not be loaded.")
		return
	}
	credits := map[string]int{"used": 0, "budget": 0}
	if h.Credits != nil {
		credits["used"], credits["budget"] = h.Credits.Used(), h.Credits.Daily()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"window_hours": int(summaryWindow / time.Hour), "answers": s.Answers, "checked_claims": s.CheckedClaims,
		"outdated_claims": s.OutdatedClaims, "incorrect_claims": s.IncorrectClaims, "cache_purges": s.CachePurges,
		"cache_purge_enabled": h.CachePurgeEnabled, "credits": credits,
	})
}
