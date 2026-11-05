package bench

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

func newClient(url string) *APIClient {
	return &APIClient{BaseURL: url, Token: "tok", HTTP: &http.Client{Timeout: 2 * time.Second}, PollEvery: 10 * time.Millisecond, Deadline: 2 * time.Second}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestCheckReturnsCompleteReport(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		var body struct{ Text string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, http.StatusOK, report.Report{ID: "r1", Status: report.StatusComplete, AnswerText: body.Text})
	}))
	defer srv.Close()
	rep, err := newClient(srv.URL).Check(context.Background(), "some text")
	require.NoError(t, err)
	assert.Equal(t, "r1", rep.ID)
	assert.Equal(t, "some text", rep.AnswerText)
	assert.Equal(t, "Bearer tok", auth)
}

func TestCheckPollsPendingReport(t *testing.T) {
	var polls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /freshness/check", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]string{"id": "r1", "status": "pending"})
	})
	mux.HandleFunc("GET /freshness/reports/r1", func(w http.ResponseWriter, _ *http.Request) {
		if polls.Add(1) <= 2 {
			writeJSON(w, http.StatusOK, report.Report{ID: "r1", Status: report.StatusPending})
			return
		}
		writeJSON(w, http.StatusOK, report.Report{ID: "r1", Status: report.StatusComplete})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	rep, err := newClient(srv.URL).Check(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, report.StatusComplete, rep.Status)
	assert.Equal(t, int32(3), polls.Load())
}

func TestCheckRetriesFullQueue(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "The verification queue is full. Try again later."})
			return
		}
		writeJSON(w, http.StatusOK, report.Report{ID: "r1", Status: report.StatusComplete})
	}))
	defer srv.Close()
	_, err := newClient(srv.URL).Check(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, int32(3), calls.Load())
}

func TestCheckGivesUpAfterFiveFullQueueAnswers(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "The verification queue is full. Try again later."})
	}))
	defer srv.Close()
	_, err := newClient(srv.URL).Check(context.Background(), "x")
	require.Error(t, err)
	assert.Equal(t, int32(5), calls.Load())
	assert.Contains(t, err.Error(), "queue is full")
}

func TestCheckGivesUpAtDeadline(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /freshness/check", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]string{"id": "r-stuck", "status": "pending"})
	})
	mux.HandleFunc("GET /freshness/reports/r-stuck", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, report.Report{ID: "r-stuck", Status: report.StatusPending})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := newClient(srv.URL)
	c.Deadline = 60 * time.Millisecond
	_, err := c.Check(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "r-stuck")
}

func TestRunCuratedKeepsOrderAndTurnsErrorsIntoOutcomes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Text string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case strings.Contains(body.Text, "broken"):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "The text field must not be empty."})
		case strings.Contains(body.Text, "outdated"):
			writeJSON(w, http.StatusOK, report.Report{ID: "r-" + body.Text, Status: report.StatusComplete,
				Claims: []report.Claim{{Checked: true, Verdict: report.VerdictStale}}})
		default:
			writeJSON(w, http.StatusOK, report.Report{ID: "r-" + body.Text, Status: report.StatusComplete,
				Claims: []report.Claim{{Checked: true, Verdict: report.VerdictFresh}}})
		}
	}))
	defer srv.Close()

	mk := func(id, text string, label Label) Item {
		return Item{ID: id, Category: report.CategoryPricing, Text: text, Label: label, CurrentValues: []string{"1"}}
	}
	items := []Item{mk("i1", "outdated one", LabelOutdated), mk("i2", "current two", LabelCurrent), mk("i3", "broken three", LabelOutdated), mk("i4", "outdated four", LabelOutdated)}
	outcomes, reports := RunCurated(context.Background(), newClient(srv.URL), items, 3)
	require.Len(t, outcomes, 4)
	require.Len(t, reports, 4)
	for i, want := range []string{"i1", "i2", "i3", "i4"} {
		assert.Equal(t, want, outcomes[i].ItemID)
	}
	assert.Equal(t, PredictedOutdated, outcomes[0].Predicted)
	assert.Equal(t, PredictedCurrent, outcomes[1].Predicted)
	assert.Equal(t, PredictedError, outcomes[2].Predicted)
	assert.Contains(t, outcomes[2].Error, "must not be empty")
	assert.Equal(t, "r-outdated one", reports[0].ID)
	assert.Equal(t, "", reports[2].ID, "a failed check has no stored report")
}

func TestRescoreUsesNewLabels(t *testing.T) {
	items := []Item{
		{ID: "a", Category: report.CategoryPricing, Label: LabelOutdated, CurrentValues: []string{"1"}},
		{ID: "b", Category: report.CategoryPricing, Label: LabelCurrent, CurrentValues: []string{"1"}},
	}
	stale := report.Report{ID: "ra", Status: report.StatusComplete, Claims: []report.Claim{{Checked: true, Verdict: report.VerdictStale}}}
	fresh := report.Report{ID: "rb", Status: report.StatusComplete, Claims: []report.Claim{{Checked: true, Verdict: report.VerdictFresh}}}
	outcomes, reports := []Outcome{Classify(items[0], stale), Classify(items[1], fresh)}, []report.Report{stale, fresh}
	run := CuratedRun{Outcomes: outcomes, Reports: reports, Summary: Summarize(outcomes)}
	require.Equal(t, Counts{TP: 1, TN: 1}, run.Summary.Counts)

	items[0].Label = LabelCurrent // the label was wrong
	rescored, err := Rescore(run, items)
	require.NoError(t, err)
	assert.Equal(t, Counts{FP: 1, TN: 1}, rescored.Summary.Counts)
	assert.Equal(t, Counts{TP: 1, TN: 1}, run.Summary.Counts, "the original run is not changed")

	missing, err := Rescore(CuratedRun{Outcomes: outcomes[:1], Reports: reports[:1]}, items)
	require.NoError(t, err)
	require.Len(t, missing.Outcomes, 2)
	assert.Equal(t, PredictedError, missing.Outcomes[1].Predicted)
	assert.Contains(t, missing.Outcomes[1].Error, "b")

	_, err = Rescore(run, items[:1])
	require.Error(t, err, "an outcome whose item is not in the dataset")
	assert.Contains(t, err.Error(), "b")
}
