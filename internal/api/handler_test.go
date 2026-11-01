package api_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/api"
	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

type fakeReports struct {
	reports map[string]report.Report
	filter  store.ListFilter
}

func (f *fakeReports) GetReport(_ context.Context, id string) (report.Report, error) {
	r, ok := f.reports[id]
	if !ok {
		return report.Report{}, store.ErrNotFound
	}
	return r, nil
}

func (f *fakeReports) ListReports(_ context.Context, filter store.ListFilter) ([]report.Report, error) {
	f.filter = filter
	return []report.Report{{ID: "r-1", Status: report.StatusComplete}}, nil
}

type fakeJobs struct {
	queued map[string]bool
	full   bool
	result *report.Report
	job    engine.Job
}

func (f *fakeJobs) Submit(job engine.Job) (<-chan report.Report, bool) {
	f.job = job
	if f.full {
		return nil, false
	}
	ch := make(chan report.Report, 1)
	if f.result != nil {
		r := *f.result
		r.ID = job.ReportID
		ch <- r
	}
	return ch, true
}

func (f *fakeJobs) IsQueued(id string) bool { return f.queued[id] }

var fixedNow = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

func sampleReport() report.Report {
	score := 2
	return report.Report{ID: "r-1", Status: report.StatusComplete, VirtualKey: "secret-vk", Claims: []report.Claim{{
		ID: "c1", Quote: "costs $20", SpanStart: 3, SpanEnd: 12, Verdict: report.VerdictStale, Score: &score,
		Evidence: []report.Evidence{{Ref: "E1", URL: "https://example.com", Stance: report.StanceSupports}},
		Timeline: []report.TimelineEntry{},
	}}}
}

func newHandler(token string, reports *fakeReports, jobs *fakeJobs) http.Handler {
	return api.NewHandler(api.Deps{
		Token: token, Reports: reports, Jobs: jobs,
		NewID: func() string { return "r-new" }, Now: func() time.Time { return fixedNow },
		CheckTimeout: 50 * time.Millisecond, Logger: slog.Default(),
	})
}

func do(h http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body["error"]
}

func TestHealthNeedsNoToken(t *testing.T) {
	rec := do(newHandler("t", &fakeReports{}, &fakeJobs{}), http.MethodGet, "/freshness/health", "", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestProtectedRoutesNeedToken(t *testing.T) {
	h := newHandler("t", &fakeReports{reports: map[string]report.Report{"r-1": sampleReport()}}, &fakeJobs{})
	rec := do(h, http.MethodGet, "/freshness/reports/r-1", "", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
	assert.Equal(t, "A valid bearer token is required.", errorOf(t, rec))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	assert.Equal(t, http.StatusUnauthorized, do(h, http.MethodGet, "/freshness/reports/r-1", "wrong", "").Code)
	assert.Equal(t, http.StatusOK, do(h, http.MethodGet, "/freshness/reports/r-1", "t", "").Code)
}

func TestEmptyTokenDisablesProtectedRoutes(t *testing.T) {
	rec := do(newHandler("", &fakeReports{}, &fakeJobs{}), http.MethodGet, "/freshness/reports", "anything", "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, errorOf(t, rec), "api.token")
}

func TestGetReportFound(t *testing.T) {
	h := newHandler("t", &fakeReports{reports: map[string]report.Report{"r-1": sampleReport()}}, &fakeJobs{})
	rec := do(h, http.MethodGet, "/freshness/reports/r-1", "t", "")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, key := range []string{`"span_start"`, `"verdict"`, `"evidence"`, `"stance"`, `"timeline"`} {
		assert.Contains(t, body, key)
	}
	assert.NotContains(t, body, "virtual_key")
	assert.NotContains(t, body, "secret-vk")
}

func TestGetReportNotFound(t *testing.T) {
	rec := do(newHandler("t", &fakeReports{}, &fakeJobs{}), http.MethodGet, "/freshness/reports/r-9", "t", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "The report was not found.", errorOf(t, rec))
}

func TestGetReportQueued(t *testing.T) {
	rec := do(newHandler("t", &fakeReports{}, &fakeJobs{queued: map[string]bool{"r-9": true}}), http.MethodGet, "/freshness/reports/r-9", "t", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"r-9","status":"pending","claims":[]}`, rec.Body.String())
}

func TestListReportsPassesFilter(t *testing.T) {
	reports := &fakeReports{}
	rec := do(newHandler("t", reports, &fakeJobs{}), http.MethodGet, "/freshness/reports?model=groq/x&since=2026-11-01&limit=500", "t", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "groq/x", reports.filter.Model)
	assert.True(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Equal(reports.filter.Since))
	assert.Equal(t, 200, reports.filter.Limit)
	var body struct {
		Reports []map[string]any `json:"reports"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Reports, 1)
	assert.Equal(t, "r-1", body.Reports[0]["id"])

	do(newHandler("t", reports, &fakeJobs{}), http.MethodGet, "/freshness/reports?since=2026-11-01T08:30:00Z", "t", "")
	assert.True(t, time.Date(2026, 11, 1, 8, 30, 0, 0, time.UTC).Equal(reports.filter.Since))
	assert.Equal(t, 50, reports.filter.Limit)
}

func TestListReportsRejectsBadSince(t *testing.T) {
	rec := do(newHandler("t", &fakeReports{}, &fakeJobs{}), http.MethodGet, "/freshness/reports?since=yesterday", "t", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "The since parameter must be an RFC 3339 time or a YYYY-MM-DD date.", errorOf(t, rec))
}

func TestCheckReturnsCompletedReport(t *testing.T) {
	done := sampleReport()
	jobs := &fakeJobs{result: &done}
	rec := do(newHandler("t", &fakeReports{}, jobs), http.MethodPost, "/freshness/check", "t",
		`{"text":"It costs $20.","prompt":"Cost?","model":"groq/x"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, engine.Job{ReportID: "r-new", Answer: "It costs $20.", Prompt: "Cost?", Model: "groq/x", Mode: "check", At: fixedNow}, jobs.job)
	var body report.Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "r-new", body.ID)
	assert.Equal(t, report.StatusComplete, body.Status)
}

func TestCheckTimesOutWithPending(t *testing.T) {
	rec := do(newHandler("t", &fakeReports{}, &fakeJobs{}), http.MethodPost, "/freshness/check", "t", `{"text":"hello"}`)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.JSONEq(t, `{"id":"r-new","status":"pending"}`, rec.Body.String())
}

func TestCheckQueueFull(t *testing.T) {
	rec := do(newHandler("t", &fakeReports{}, &fakeJobs{full: true}), http.MethodPost, "/freshness/check", "t", `{"text":"hello"}`)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "The verification queue is full. Try again later.", errorOf(t, rec))
}

func TestCheckValidatesText(t *testing.T) {
	h := newHandler("t", &fakeReports{}, &fakeJobs{})
	assert.Equal(t, http.StatusBadRequest, do(h, http.MethodPost, "/freshness/check", "t", `{"text":"  "}`).Code)
	long, _ := json.Marshal(map[string]string{"text": strings.Repeat("é", api.MaxCheckTextRunes+1)})
	assert.Equal(t, http.StatusBadRequest, do(h, http.MethodPost, "/freshness/check", "t", string(long)).Code)
	assert.Equal(t, http.StatusBadRequest, do(h, http.MethodPost, "/freshness/check", "t", `not json`).Code)
}
