package hooks_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/hooks"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

// resultSubmitter answers each job with rep (unless never is set).
type resultSubmitter struct {
	rep   *report.Report
	never bool
	full  bool
	jobs  []engine.Job
}

func (f *resultSubmitter) Submit(job engine.Job) (<-chan report.Report, bool) {
	if f.full {
		return nil, false
	}
	f.jobs = append(f.jobs, job)
	ch := make(chan report.Report, 1)
	if f.rep != nil && !f.never {
		r := *f.rep
		r.ID = job.ReportID
		ch <- r
	}
	return ch, true
}

func intPtr(v int) *int { return &v }

func completeReport() *report.Report {
	return &report.Report{Status: report.StatusComplete, Score: intPtr(2), StaleCount: 1, CheckedCount: 2,
		TotalClaims: 2, HealedText: "healed", HealedCount: 1}
}

func syncHandler(sub hooks.Submitter, mutate func(*config.Config)) *hooks.Handler {
	cfg := config.Default()
	cfg.SyncTimeout = time.Second
	if mutate != nil {
		mutate(&cfg)
	}
	return &hooks.Handler{Config: cfg, Jobs: sub, NewID: func() string { return "r-1" },
		Now: func() time.Time { return hookNow }, Sample: func() float64 { return 0 }, Logger: slog.Default()}
}

func runSync(h *hooks.Handler, headers map[string]string) (*schemas.BifrostResponse, *schemas.BifrostError, *schemas.HTTPResponseMetadata) {
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{Headers: headers})
	h.PreLLM(ctx, chatRequest(schemas.ChatCompletionRequest))
	resp, bErr := h.PostLLM(ctx, chatResponse(&schemas.ChatMessageContent{ContentStr: str(testAnswer)}))
	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	return resp, bErr, md
}

func content(t *testing.T, resp *schemas.BifrostResponse) string {
	t.Helper()
	require.NotNil(t, resp)
	return *resp.ChatResponse.Choices[0].Message.Content.ContentStr
}

func TestSyncHealReplacesAnswer(t *testing.T) {
	sub := &resultSubmitter{rep: completeReport()}
	resp, bErr, md := runSync(syncHandler(sub, nil), map[string]string{"x-bf-freshness": "sync", "x-bf-freshness-policy": "heal"})
	require.Nil(t, bErr)
	assert.Equal(t, "healed", content(t, resp))
	assert.Equal(t, "r-1", md.Header("X-Freshness-Report-Id"))
	assert.Equal(t, "complete", md.Header("X-Freshness-Status"))
	assert.Equal(t, "2", md.Header("X-Freshness-Score"))
	assert.Equal(t, "1", md.Header("X-Freshness-Stale"))
	assert.Equal(t, "2/2", md.Header("X-Freshness-Checked"))
	assert.Equal(t, "1", md.Header("X-Freshness-Healed"))
	require.Len(t, sub.jobs, 1)
	assert.Equal(t, "sync", sub.jobs[0].Mode)
	assert.Equal(t, "heal", sub.jobs[0].Policy)
}

func TestSyncAnnotateKeepsAnswer(t *testing.T) {
	resp, bErr, md := runSync(syncHandler(&resultSubmitter{rep: completeReport()}, nil),
		map[string]string{"x-bf-freshness": "sync", "x-bf-freshness-policy": "annotate"})
	require.Nil(t, bErr)
	assert.Equal(t, testAnswer, content(t, resp))
	assert.Equal(t, "complete", md.Header("X-Freshness-Status"))
	assert.Empty(t, md.Header("X-Freshness-Healed"))
}

func TestSyncBlockReturnsError(t *testing.T) {
	rep := completeReport()
	rep.Blocked = true
	resp, bErr, _ := runSync(syncHandler(&resultSubmitter{rep: rep}, nil),
		map[string]string{"x-bf-freshness": "sync", "x-bf-freshness-policy": "block"})
	assert.Nil(t, resp)
	require.NotNil(t, bErr)
	require.NotNil(t, bErr.StatusCode)
	assert.Equal(t, 422, *bErr.StatusCode)
	require.NotNil(t, bErr.Error)
	require.NotNil(t, bErr.Error.Type)
	assert.Equal(t, "freshness_blocked", *bErr.Error.Type)
	assert.Contains(t, bErr.Error.Message, "r-1")
	assert.Contains(t, bErr.Error.Message, "outdated or incorrect claims")
	require.NotNil(t, bErr.AllowFallbacks)
	assert.False(t, *bErr.AllowFallbacks)
}

func TestSyncTimeoutReturnsPending(t *testing.T) {
	h := syncHandler(&resultSubmitter{never: true}, func(c *config.Config) { c.SyncTimeout = 20 * time.Millisecond })
	start := time.Now()
	resp, bErr, md := runSync(h, map[string]string{"x-bf-freshness": "sync"})
	assert.Less(t, time.Since(start), 500*time.Millisecond)
	require.Nil(t, bErr)
	assert.Equal(t, testAnswer, content(t, resp))
	assert.Equal(t, "pending", md.Header("X-Freshness-Status"))
	assert.Equal(t, "r-1", md.Header("X-Freshness-Report-Id"))
}

func TestSyncTimeoutWithFailClosedBlocks(t *testing.T) {
	h := syncHandler(&resultSubmitter{never: true}, func(c *config.Config) {
		c.SyncTimeout = 20 * time.Millisecond
		c.FailClosed = true
	})
	resp, bErr, _ := runSync(h, map[string]string{"x-bf-freshness": "sync", "x-bf-freshness-policy": "block"})
	assert.Nil(t, resp)
	require.NotNil(t, bErr)
	assert.Contains(t, bErr.Error.Message, "verified in time")
}

func TestSyncErrorReportPassesThrough(t *testing.T) {
	rep := completeReport()
	rep.Status = report.StatusError
	resp, bErr, md := runSync(syncHandler(&resultSubmitter{rep: rep}, nil),
		map[string]string{"x-bf-freshness": "sync", "x-bf-freshness-policy": "heal"})
	require.Nil(t, bErr)
	assert.Equal(t, testAnswer, content(t, resp))
	assert.Equal(t, "error", md.Header("X-Freshness-Status"))
	assert.Equal(t, "r-1", md.Header("X-Freshness-Report-Id"))
}

func TestSyncQueueFullIsSkipped(t *testing.T) {
	resp, bErr, md := runSync(syncHandler(&resultSubmitter{full: true}, nil), map[string]string{"x-bf-freshness": "sync"})
	require.Nil(t, bErr)
	assert.Equal(t, testAnswer, content(t, resp))
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
}
