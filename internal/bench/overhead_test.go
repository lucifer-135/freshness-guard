package bench

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOverheadInterleavesModes(t *testing.T) {
	var mu sync.Mutex
	var modes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := r.Header.Get("x-bf-freshness")
		mu.Lock()
		modes = append(modes, mode)
		mu.Unlock()
		if mode == "async" {
			w.Header().Set("X-Freshness-Status", "pending")
		}
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	run, err := RunOverhead(context.Background(), OverheadConfig{BaseURL: srv.URL, Model: "openai/fake-model", Requests: 10, Warmup: 4, HTTP: srv.Client()})
	require.NoError(t, err)
	require.Len(t, modes, 14)
	assert.Equal(t, []string{"off", "async", "off", "async", "off", "async", "off", "async", "off", "async"}, modes[4:], "after the warmup the modes alternate, off first")
	assert.Equal(t, 5, run.Off.Requests)
	assert.Equal(t, 5, run.Async.Requests)
	assert.Equal(t, map[string]int{"": 5}, run.Off.Statuses)
	assert.Equal(t, map[string]int{"pending": 5}, run.Async.Statuses)
	assert.Equal(t, run.Async.P99US-run.Off.P99US, run.AddedP99US)
}

func TestRunOverheadFailsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := RunOverhead(context.Background(), OverheadConfig{BaseURL: srv.URL, Model: "m", Requests: 4, HTTP: srv.Client()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOverheadMarkdown(t *testing.T) {
	run := OverheadRun{
		Meta:       RunMeta{Name: "overhead", Note: "Docker Desktop, fake provider", StartedAt: time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)},
		Off:        ModeStats{Mode: "off", Requests: 500, P50US: 1500, P95US: 1800, P99US: 2000, MeanUS: 1550, Statuses: map[string]int{"": 500}},
		Async:      ModeStats{Mode: "async", Requests: 500, P50US: 1700, P95US: 2100, P99US: 2750, MeanUS: 1800, Statuses: map[string]int{"pending": 498, "skipped": 2}},
		AddedP50US: 200, AddedP99US: 750,
	}
	md := OverheadMarkdown(run)
	for _, want := range []string{"1.500", "1.800", "2.000", "1.700", "2.100", "2.750", "0.750", "0.200", "pending: 498", "skipped: 2",
		"Target: async mode adds less than 2 ms at the 99th percentile.", "x-bf-freshness: off", "fake provider", "Docker Desktop, fake provider"} {
		assert.Contains(t, md, want)
	}
}
