// Command freshbench scores Freshness Guard on the curated set and measures
// what async mode adds to a request.
//
//	freshbench curated  -dataset bench/curated.json -out bench/results -name curated
//	freshbench score    -results bench/results/curated-DATE.json -dataset bench/curated.json -out bench/results
//	freshbench overhead -n 1000 -warmup 100 -model openai/fake-model -out bench/results
//
// FG_URL and FG_TOKEN (or FG_API_TOKEN) point at the plugin API; BIFROST_URL
// at the gateway.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/bench"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: freshbench curated|score|overhead [flags]")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var err error
	switch os.Args[1] {
	case "curated":
		err = curated(ctx, os.Args[2:])
	case "score":
		err = score(os.Args[2:])
	case "overhead":
		err = overhead(ctx, os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "freshbench:", err)
		os.Exit(1)
	}
}

func env(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

func loadItems(path string) ([]bench.Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return bench.LoadItems(f)
}

func curated(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("curated", flag.ExitOnError)
	dataset := fs.String("dataset", "bench/curated.json", "the curated set")
	out := fs.String("out", "bench/results", "folder for the result files")
	name := fs.String("name", "curated", "name of the run")
	concurrency := fs.Int("concurrency", 2, "checks in flight at once")
	note := fs.String("note", "", "models and setup, written into the report")
	_ = fs.Parse(args)

	items, err := loadItems(*dataset)
	if err != nil {
		return err
	}
	client := &bench.APIClient{BaseURL: env("FG_URL"), Token: env("FG_TOKEN", "FG_API_TOKEN"), PollEvery: 2 * time.Second, Deadline: 5 * time.Minute}
	if client.BaseURL == "" || client.Token == "" {
		return errors.New("set FG_URL and FG_TOKEN (or FG_API_TOKEN)")
	}
	run := bench.CuratedRun{Meta: bench.RunMeta{Name: *name, Dataset: *dataset, Note: *note, StartedAt: time.Now()}}
	if settings, err := client.Settings(ctx); err == nil {
		run.Meta.Settings = settings
	}
	fmt.Printf("checking %d items with %d at a time\n", len(items), *concurrency)
	run.Outcomes, run.Reports = bench.RunCurated(ctx, client, items, *concurrency)
	run.Meta.FinishedAt = time.Now()
	run.Summary = bench.Summarize(run.Outcomes)
	return save(*out, *name, run.Meta.StartedAt, run, bench.CuratedMarkdown(run.Meta, run.Summary, run.Outcomes))
}

func score(args []string) error {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	results := fs.String("results", "", "a curated result file (.json)")
	dataset := fs.String("dataset", "bench/curated.json", "the curated set, with corrected labels")
	out := fs.String("out", "bench/results", "folder for the result files")
	_ = fs.Parse(args)

	data, err := os.ReadFile(*results)
	if err != nil {
		return err
	}
	var run bench.CuratedRun
	if err := json.Unmarshal(data, &run); err != nil {
		return fmt.Errorf("reading %s: %w", *results, err)
	}
	items, err := loadItems(*dataset)
	if err != nil {
		return err
	}
	rescored, err := bench.Rescore(run, items)
	if err != nil {
		return err
	}
	rescored.Meta.Name = run.Meta.Name + "-rescored"
	rescored.Meta.Dataset = *dataset
	return save(*out, rescored.Meta.Name, time.Now(), rescored, bench.CuratedMarkdown(rescored.Meta, rescored.Summary, rescored.Outcomes))
}

func overhead(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("overhead", flag.ExitOnError)
	n := fs.Int("n", 1000, "timed requests, split between off and async")
	warmup := fs.Int("warmup", 100, "requests before timing starts")
	model := fs.String("model", "openai/fake-model", "model to ask")
	out := fs.String("out", "bench/results", "folder for the result files")
	name := fs.String("name", "overhead", "name of the run")
	note := fs.String("note", "", "machine and setup, written into the report")
	_ = fs.Parse(args)

	base := env("BIFROST_URL")
	if base == "" {
		return errors.New("set BIFROST_URL")
	}
	run, err := bench.RunOverhead(ctx, bench.OverheadConfig{BaseURL: base, Model: *model, Requests: *n, Warmup: *warmup})
	if err != nil {
		return err
	}
	run.Meta.Name, run.Meta.Note = *name, *note
	return save(*out, *name, run.Meta.StartedAt, run, bench.OverheadMarkdown(run))
}

func save(dir, name string, day time.Time, v any, markdown string) error {
	paths, err := writeResult(dir, name, day, v, markdown)
	if err != nil {
		return err
	}
	fmt.Println("wrote", paths.JSON)
	fmt.Println("wrote", paths.Markdown)
	return nil
}

type resultPaths struct{ JSON, Markdown string }

// writeResult writes <dir>/<name>-<date>.json and .md. An existing result is
// never overwritten: a second run on the same day gets -2, a third -3.
func writeResult(dir, name string, day time.Time, v any, markdown string) (resultPaths, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return resultPaths{}, err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return resultPaths{}, err
	}
	base := fmt.Sprintf("%s-%s", name, day.Format("2006-01-02"))
	for n := 1; ; n++ {
		stem := base
		if n > 1 {
			stem = fmt.Sprintf("%s-%d", base, n)
		}
		p := resultPaths{JSON: filepath.Join(dir, stem+".json"), Markdown: filepath.Join(dir, stem+".md")}
		if err := writeNew(p.JSON, data); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return resultPaths{}, err
		}
		if err := writeNew(p.Markdown, []byte(markdown)); err != nil {
			_ = os.Remove(p.JSON)
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return resultPaths{}, err
		}
		return p, nil
	}
}

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
