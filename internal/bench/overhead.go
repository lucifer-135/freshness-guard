package bench

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// OverheadConfig describes an overhead run against Bifrost.
type OverheadConfig struct {
	BaseURL, Model string
	// Requests is the total number of timed requests, split evenly between
	// the two modes; Warmup requests run first and are not timed.
	Requests, Warmup int
	HTTP             *http.Client
}

// ModeStats are the timings of one mode in microseconds.
type ModeStats struct {
	Mode                        string
	Requests                    int
	P50US, P95US, P99US, MeanUS int64
	Statuses                    map[string]int // X-Freshness-Status values seen ("" for none)
}

// OverheadRun compares requests that the plugin skips (x-bf-freshness: off)
// with async-mode requests. The Added fields are async minus off.
type OverheadRun struct {
	Meta                   RunMeta
	Off, Async             ModeStats
	AddedP50US, AddedP99US int64
}

const overheadBody = `{"model":"%s","messages":[{"role":"user","content":"What does Example Cloud Pro cost?"}]}`

// RunOverhead sends chat requests one at a time, alternating off and async,
// so that drift in the machine's speed hits both modes alike. Time is taken
// until the response body is fully read.
func RunOverhead(ctx context.Context, cfg OverheadConfig) (OverheadRun, error) {
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	send := func(mode string) (time.Duration, string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+"/v1/chat/completions",
			strings.NewReader(fmt.Sprintf(overheadBody, cfg.Model)))
		if err != nil {
			return 0, "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-bf-freshness", mode)
		start := time.Now()
		resp, err := httpClient.Do(req)
		if err != nil {
			return 0, "", err
		}
		_, err = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		elapsed := time.Since(start)
		if err != nil {
			return 0, "", err
		}
		if resp.StatusCode != http.StatusOK {
			return 0, "", fmt.Errorf("the gateway answered HTTP %d in %s mode", resp.StatusCode, mode)
		}
		return elapsed, resp.Header.Get("X-Freshness-Status"), nil
	}
	modeAt := func(i int) string {
		if i%2 == 0 {
			return "off"
		}
		return "async"
	}

	run := OverheadRun{Meta: RunMeta{Name: "overhead", StartedAt: time.Now()}}
	for i := 0; i < cfg.Warmup; i++ {
		if _, _, err := send(modeAt(i)); err != nil {
			return run, fmt.Errorf("warmup request %d: %w", i+1, err)
		}
	}
	samples := map[string][]int64{}
	statuses := map[string]map[string]int{"off": {}, "async": {}}
	for i := 0; i < cfg.Requests; i++ {
		mode := modeAt(i)
		elapsed, status, err := send(mode)
		if err != nil {
			return run, fmt.Errorf("request %d: %w", i+1, err)
		}
		samples[mode] = append(samples[mode], elapsed.Microseconds())
		statuses[mode][status]++
	}
	run.Meta.FinishedAt = time.Now()
	run.Off = stats("off", samples["off"], statuses["off"])
	run.Async = stats("async", samples["async"], statuses["async"])
	run.AddedP50US = run.Async.P50US - run.Off.P50US
	run.AddedP99US = run.Async.P99US - run.Off.P99US
	return run, nil
}

func stats(mode string, us []int64, statuses map[string]int) ModeStats {
	var sum int64
	for _, v := range us {
		sum += v
	}
	s := ModeStats{Mode: mode, Requests: len(us), Statuses: statuses,
		P50US: Percentile(us, 50), P95US: Percentile(us, 95), P99US: Percentile(us, 99)}
	if len(us) > 0 {
		s.MeanUS = sum / int64(len(us))
	}
	return s
}

func ms(us int64) string { return fmt.Sprintf("%.3f", float64(us)/1000) }

func statusList(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		name := k
		if name == "" {
			name = "(none)"
		}
		parts[i] = fmt.Sprintf("%s: %d", name, m[k])
	}
	return strings.Join(parts, ", ")
}

// OverheadMarkdown renders an overhead run as a Markdown report.
func OverheadMarkdown(r OverheadRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Async overhead: %s\n\n", r.Meta.Name)
	b.WriteString("Target: async mode adds less than 2 ms at the 99th percentile. A target is not a result; the numbers below are what this run measured.\n\n")
	b.WriteString("The baseline is `x-bf-freshness: off`: the plugin is loaded but skips the request. Both modes answer from a fake provider, one request at a time, alternating, so the numbers show what the plugin adds, not what a real model takes.\n\n")
	if !r.Meta.StartedAt.IsZero() {
		fmt.Fprintf(&b, "- Run: %s\n", r.Meta.StartedAt.UTC().Format("2006-01-02 15:04 MST"))
	}
	if r.Meta.Note != "" {
		fmt.Fprintf(&b, "- Setup: %s\n", r.Meta.Note)
	}
	b.WriteString("\n| Mode | Requests | p50 (ms) | p95 (ms) | p99 (ms) | Mean (ms) | X-Freshness-Status |\n|---|---|---|---|---|---|---|\n")
	for _, m := range []ModeStats{r.Off, r.Async} {
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %s | %s |\n", m.Mode, m.Requests, ms(m.P50US), ms(m.P95US), ms(m.P99US), ms(m.MeanUS), statusList(m.Statuses))
	}
	fmt.Fprintf(&b, "\nAsync minus off: %s ms at p50 and %s ms at p99.\n", ms(r.AddedP50US), ms(r.AddedP99US))
	return b.String()
}
