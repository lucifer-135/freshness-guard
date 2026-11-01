package engine

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// JobTimeout limits one job.
const JobTimeout = 60 * time.Second

// Runner checks one job; *Pipeline implements it.
type Runner interface {
	Run(ctx context.Context, job Job) report.Report
}

type queuedJob struct {
	job    Job
	result chan report.Report
}

// Engine runs jobs in a fixed number of workers fed by a bounded queue.
// Submitting never blocks.
type Engine struct {
	runner  Runner
	workers int
	logger  *slog.Logger
	queue   chan queuedJob
	base    context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	mu      sync.Mutex
	stopped bool
	pending map[string]struct{}
}

// New returns an engine with the given number of workers and queue size.
// Call Start to begin processing.
func New(r Runner, workers, queueSize int, logger *slog.Logger) *Engine {
	base, cancel := context.WithCancel(context.Background())
	return &Engine{
		runner: r, workers: workers, logger: logger,
		queue: make(chan queuedJob, queueSize), base: base, cancel: cancel,
		pending: map[string]struct{}{},
	}
}

// Start launches the workers.
func (e *Engine) Start() {
	for range e.workers {
		e.wg.Add(1)
		go e.work()
	}
}

// Submit queues a job without blocking. It returns false when the queue is
// full or the engine has stopped. The channel receives the finished report and
// is then closed.
func (e *Engine) Submit(job Job) (<-chan report.Report, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return nil, false
	}
	result := make(chan report.Report, 1)
	select {
	case e.queue <- queuedJob{job: job, result: result}:
		e.pending[job.ReportID] = struct{}{}
		return result, true
	default:
		return nil, false
	}
}

// IsQueued reports whether the job is waiting or running.
func (e *Engine) IsQueued(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.pending[id]
	return ok
}

// Stop rejects new jobs and waits for the queue to drain. When ctx ends first,
// it cancels the running jobs and returns ctx.Err() without waiting further.
func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	if !e.stopped {
		e.stopped = true
		close(e.queue)
	}
	e.mu.Unlock()

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		e.cancel()
		return nil
	case <-ctx.Done():
		e.cancel()
		return ctx.Err()
	}
}

func (e *Engine) work() {
	defer e.wg.Done()
	for q := range e.queue {
		r := e.run(q.job)
		e.mu.Lock()
		delete(e.pending, q.job.ReportID)
		e.mu.Unlock()
		q.result <- r
		close(q.result)
	}
}

func (e *Engine) run(job Job) (r report.Report) {
	ctx, cancel := context.WithTimeout(e.base, JobTimeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			e.logger.Error("A verification job panicked.", "report", job.ReportID, "panic", p, "stack", string(debug.Stack()))
			r = report.Report{
				ID: job.ReportID, CreatedAt: job.At, Model: job.Model, Provider: job.Provider, Mode: job.Mode,
				Status: report.StatusError, Reason: internalErrorReason,
				Prompt: job.Prompt, AnswerText: job.Answer, Claims: []report.Claim{},
			}
		}
	}()
	return e.runner.Run(ctx, job)
}
