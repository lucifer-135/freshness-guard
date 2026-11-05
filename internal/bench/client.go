package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// maxQueueFullTries is how often a check is submitted while the plugin's
// queue answers that it is full.
const maxQueueFullTries = 5

// APIClient talks to the plugin API for the benchmark.
type APIClient struct {
	BaseURL, Token string
	HTTP           *http.Client
	// PollEvery is the wait between polls of a pending report, and between
	// tries while the queue is full. Deadline bounds one check.
	PollEvery, Deadline time.Duration
	// Pace is the least time between the starts of two checks, shared by all
	// goroutines. Free-tier model keys allow only a few thousand tokens a
	// minute, so a benchmark must not start checks faster than that.
	Pace time.Duration

	paceMu    sync.Mutex
	nextStart time.Time
}

// waitForTurn blocks until Pace has passed since the previous check started.
func (c *APIClient) waitForTurn(ctx context.Context) error {
	if c.Pace <= 0 {
		return nil
	}
	c.paceMu.Lock()
	now := time.Now()
	start := c.nextStart
	if start.Before(now) {
		start = now
	}
	c.nextStart = start.Add(c.Pace)
	c.paceMu.Unlock()
	select {
	case <-time.After(time.Until(start)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *APIClient) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c *APIClient) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http().Do(req)
}

func apiError(status int, body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return fmt.Sprintf("%s (HTTP %d)", e.Error, status)
	}
	return fmt.Sprintf("HTTP %d: %s", status, strings.TrimSpace(string(body)))
}

// Check sends text to POST /freshness/check and returns the finished report,
// polling the report while the API says it is still pending.
func (c *APIClient) Check(ctx context.Context, text string) (report.Report, error) {
	if err := c.waitForTurn(ctx); err != nil {
		return report.Report{}, fmt.Errorf("waiting for a turn: %w", err)
	}
	if c.Deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Deadline)
		defer cancel()
	}
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return report.Report{}, err
	}
	pendingID := ""
	fail := func(err error) (report.Report, error) {
		if pendingID != "" && ctx.Err() != nil {
			return report.Report{}, fmt.Errorf("report %s was still pending at the deadline", pendingID)
		}
		return report.Report{}, err
	}

	for try := 1; ; try++ {
		resp, err := c.do(ctx, http.MethodPost, "/freshness/check", strings.NewReader(string(payload)))
		if err != nil {
			return fail(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			var rep report.Report
			if err := json.Unmarshal(data, &rep); err != nil {
				return report.Report{}, fmt.Errorf("reading the report: %w", err)
			}
			return rep, nil
		case http.StatusAccepted:
			var p struct{ ID string }
			if err := json.Unmarshal(data, &p); err != nil || p.ID == "" {
				return report.Report{}, fmt.Errorf("the pending answer has no report id: %s", data)
			}
			pendingID = p.ID
			return c.poll(ctx, pendingID, fail)
		case http.StatusServiceUnavailable:
			if try >= maxQueueFullTries {
				return report.Report{}, fmt.Errorf("the check was refused %d times: %s", try, apiError(resp.StatusCode, data))
			}
			if err := sleep(ctx, c.PollEvery); err != nil {
				return fail(err)
			}
		default:
			return report.Report{}, fmt.Errorf("the check was refused: %s", apiError(resp.StatusCode, data))
		}
	}
}

func (c *APIClient) poll(ctx context.Context, id string, fail func(error) (report.Report, error)) (report.Report, error) {
	for {
		if err := sleep(ctx, c.PollEvery); err != nil {
			return fail(err)
		}
		resp, err := c.do(ctx, http.MethodGet, "/freshness/reports/"+id, nil)
		if err != nil {
			return fail(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return report.Report{}, fmt.Errorf("reading report %s: %s", id, apiError(resp.StatusCode, data))
		}
		var rep report.Report
		if err := json.Unmarshal(data, &rep); err != nil {
			return report.Report{}, fmt.Errorf("reading report %s: %w", id, err)
		}
		if rep.Status != report.StatusPending {
			return rep, nil
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Settings returns the "settings" object of GET /freshness/status, which a
// result file keeps so that a run records how the plugin was configured.
func (c *APIClient) Settings(ctx context.Context) (json.RawMessage, error) {
	resp, err := c.do(ctx, http.MethodGet, "/freshness/status", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading the status: %s", apiError(resp.StatusCode, data))
	}
	var s struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return s.Settings, nil
}

// RunCurated checks every item and returns outcomes and reports in item
// order. A failed check becomes an error outcome and an empty report; it never
// stops the run.
func RunCurated(ctx context.Context, c *APIClient, items []Item, concurrency int) ([]Outcome, []report.Report) {
	if concurrency < 1 {
		concurrency = 1
	}
	outcomes := make([]Outcome, len(items))
	reports := make([]report.Report, len(items))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			rep, err := c.Check(ctx, item.Text)
			if err != nil {
				outcomes[i] = errorOutcome(item, err.Error())
				return
			}
			outcomes[i] = Classify(item, rep)
			reports[i] = rep
		}()
	}
	wg.Wait()
	return outcomes, reports
}

func errorOutcome(item Item, message string) Outcome {
	return Outcome{ItemID: item.ID, Category: item.Category, Label: item.Label, Predicted: PredictedError, Error: message}
}

// CuratedRun is a finished curated run. Reports are kept so that corrected
// labels can be scored again without spending credits.
type CuratedRun struct {
	Meta     RunMeta         `json:"meta"`
	Summary  Summary         `json:"summary"`
	Outcomes []Outcome       `json:"outcomes"`
	Reports  []report.Report `json:"reports"`
}

// Rescore classifies the stored reports again against items, matching each
// outcome's ItemID to an item and its ReportID to a stored report. An item the
// run never checked becomes an error outcome; an outcome whose item is not in
// items is an error. The run passed in is not changed.
func Rescore(run CuratedRun, items []Item) (CuratedRun, error) {
	known := make(map[string]Item, len(items))
	for _, it := range items {
		known[it.ID] = it
	}
	byItem := make(map[string]Outcome, len(run.Outcomes))
	for _, o := range run.Outcomes {
		if _, ok := known[o.ItemID]; !ok {
			return CuratedRun{}, fmt.Errorf("the run has a result for item %q, which is not in the dataset", o.ItemID)
		}
		byItem[o.ItemID] = o
	}
	byReport := make(map[string]report.Report, len(run.Reports))
	for _, r := range run.Reports {
		if r.ID != "" {
			byReport[r.ID] = r
		}
	}

	out := run
	out.Outcomes = make([]Outcome, 0, len(items))
	for _, item := range items {
		old, ok := byItem[item.ID]
		switch rep, stored := byReport[old.ReportID]; {
		case !ok:
			out.Outcomes = append(out.Outcomes, errorOutcome(item, fmt.Sprintf("item %s was not in the run", item.ID)))
		case old.ReportID != "" && stored:
			out.Outcomes = append(out.Outcomes, Classify(item, rep))
		case old.ReportID != "":
			out.Outcomes = append(out.Outcomes, errorOutcome(item, fmt.Sprintf("the stored report %s is missing", old.ReportID)))
		default:
			out.Outcomes = append(out.Outcomes, errorOutcome(item, old.Error))
		}
	}
	out.Summary = Summarize(out.Outcomes)
	return out, nil
}
