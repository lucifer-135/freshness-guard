package engine_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

type fakeRunner struct {
	started  chan string
	release  chan struct{}
	waitCtx  bool
	canceled chan struct{}
	panics   bool

	mu  sync.Mutex
	ran []string
}

func (f *fakeRunner) Run(ctx context.Context, job engine.Job) report.Report {
	if f.started != nil {
		f.started <- job.ReportID
	}
	if f.panics {
		panic("boom")
	}
	if f.waitCtx {
		<-ctx.Done()
		close(f.canceled)
	}
	if f.release != nil {
		<-f.release
	}
	f.mu.Lock()
	f.ran = append(f.ran, job.ReportID)
	f.mu.Unlock()
	return report.Report{ID: job.ReportID, Status: report.StatusComplete}
}

func job(id string) engine.Job { return engine.Job{ReportID: id, At: time.Now()} }

func receive(t *testing.T, ch <-chan report.Report) report.Report {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("no report within 2 seconds")
		return report.Report{}
	}
}

func stop(t *testing.T, e *engine.Engine) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, e.Stop(ctx))
}

func TestSubmitRunsJobAndReturnsReport(t *testing.T) {
	e := engine.New(&fakeRunner{}, 1, 4, slog.Default())
	e.Start()
	defer stop(t, e)
	ch, ok := e.Submit(job("r-1"))
	require.True(t, ok)
	assert.Equal(t, "r-1", receive(t, ch).ID)
	_, open := <-ch
	assert.False(t, open)
}

func TestSubmitFailsWhenQueueFull(t *testing.T) {
	r := &fakeRunner{started: make(chan string, 4), release: make(chan struct{})}
	e := engine.New(r, 1, 1, slog.Default())
	e.Start()
	_, ok := e.Submit(job("r-1"))
	require.True(t, ok)
	<-r.started
	_, ok = e.Submit(job("r-2"))
	require.True(t, ok)
	_, ok = e.Submit(job("r-3"))
	assert.False(t, ok)
	close(r.release)
	stop(t, e)
}

func TestIsQueuedUntilRunFinishes(t *testing.T) {
	r := &fakeRunner{started: make(chan string, 1), release: make(chan struct{})}
	e := engine.New(r, 1, 4, slog.Default())
	e.Start()
	ch, ok := e.Submit(job("r-1"))
	require.True(t, ok)
	<-r.started
	assert.True(t, e.IsQueued("r-1"))
	assert.False(t, e.IsQueued("r-2"))
	close(r.release)
	receive(t, ch)
	assert.False(t, e.IsQueued("r-1"))
	stop(t, e)
}

func TestStopDrainsQueuedJobs(t *testing.T) {
	r := &fakeRunner{}
	e := engine.New(r, 1, 10, slog.Default())
	for _, id := range []string{"r-1", "r-2", "r-3"} {
		_, ok := e.Submit(job(id))
		require.True(t, ok)
	}
	e.Start()
	stop(t, e)
	assert.ElementsMatch(t, []string{"r-1", "r-2", "r-3"}, r.ran)
	_, ok := e.Submit(job("r-4"))
	assert.False(t, ok)
}

func TestStopCancelsAfterDeadline(t *testing.T) {
	r := &fakeRunner{started: make(chan string, 1), waitCtx: true, canceled: make(chan struct{})}
	e := engine.New(r, 1, 4, slog.Default())
	e.Start()
	_, ok := e.Submit(job("r-1"))
	require.True(t, ok)
	<-r.started
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, e.Stop(ctx), context.DeadlineExceeded)
	select {
	case <-r.canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("the running job was not cancelled")
	}
}

func TestRunnerPanicBecomesErrorReport(t *testing.T) {
	e := engine.New(&fakeRunner{panics: true}, 1, 4, slog.Default())
	e.Start()
	defer stop(t, e)
	ch, ok := e.Submit(job("r-1"))
	require.True(t, ok)
	r := receive(t, ch)
	assert.Equal(t, "r-1", r.ID)
	assert.Equal(t, report.StatusError, r.Status)
	assert.Equal(t, "An internal error stopped the verification.", r.Reason)
	assert.False(t, e.IsQueued("r-1"))
}
