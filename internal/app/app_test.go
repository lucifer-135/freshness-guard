package app_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/app"
	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

const answer = "Example Cloud Pro costs $20 per month, and its current version is 4.2."

const extraction = `{"claims":[
 {"quote":"Example Cloud Pro costs $20 per month","entity":"Example Cloud Pro","attribute":"monthly price","value":"$20 per month",
  "category":"pricing","temporal_marker":"","search_query":"Example Cloud Pro monthly price"},
 {"quote":"its current version is 4.2","entity":"Example Cloud Pro","attribute":"current version","value":"4.2",
  "category":"version","temporal_marker":"current","search_query":"Example Cloud Pro latest version"}]}`

type chatBody struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	ResponseFormat struct {
		JSONSchema struct {
			Name string `json:"name"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

func judgeReply(user string) string {
	var in struct {
		Claims []struct {
			ID        string `json:"id"`
			Attribute string `json:"attribute"`
			Evidence  []struct {
				Ref string `json:"ref"`
			} `json:"evidence"`
		} `json:"claims"`
	}
	_ = json.Unmarshal([]byte(user), &in)
	results := []map[string]any{}
	for _, c := range in.Claims {
		refs := []string{}
		for _, e := range c.Evidence {
			refs = append(refs, e.Ref)
		}
		r := map[string]any{"claim_id": c.ID, "verdict": "FRESH", "current_value": "4.2", "as_of": "2026-10-15",
			"supporting_evidence_ids": refs, "contradicting_evidence_ids": []string{}, "timeline": []any{},
			"confidence": 0.85, "rationale": "Confirmed."}
		if strings.Contains(c.Attribute, "price") {
			r["verdict"], r["current_value"], r["as_of"] = "STALE", "$25 per month", "2026-09-01"
			r["supporting_evidence_ids"], r["confidence"] = refs[:1], 0.9
		}
		results = append(results, r)
	}
	out, _ := json.Marshal(map[string]any{"results": results})
	return string(out)
}

func fakeLLM(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		content := extraction
		user := body.Messages[len(body.Messages)-1].Content
		switch body.ResponseFormat.JSONSchema.Name {
		case "verdicts":
			content = judgeReply(user)
		case "healed_answer":
			var in struct {
				Answer string `json:"answer"`
			}
			_ = json.Unmarshal([]byte(user), &in)
			out, _ := json.Marshal(map[string]string{"text": strings.Replace(in.Answer, "$20", "$25", 1)})
			content = string(out)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fakeTavily(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{
			map[string]any{"title": "Example", "url": "https://example.com/a", "content": "Pro is $25.", "published_date": "2026-10-01"},
			map[string]any{"title": "Blog", "url": "https://blog.dev/b", "content": "Pro is $25.", "published_date": "2026-10-02"},
		}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

func testConfig(t *testing.T) config.Config {
	cfg := config.Default()
	cfg.DBPath = filepath.Join(t.TempDir(), "freshness.db")
	cfg.BifrostBaseURL = fakeLLM(t).URL
	cfg.TavilyBaseURL = fakeTavily(t).URL
	cfg.TavilyAPIKey = "k"
	cfg.APIHost = "127.0.0.1"
	cfg.APIPort = freePort(t)
	cfg.APIToken = "t"
	return cfg
}

func TestAppChecksTextEndToEnd(t *testing.T) {
	cfg := testConfig(t)
	a, err := app.New(cfg, slog.Default())
	require.NoError(t, err)
	require.NoError(t, a.Start())

	body, _ := json.Marshal(map[string]string{"text": answer})
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/freshness/check", cfg.APIPort), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var rep report.Report
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rep))
	assert.Equal(t, report.StatusComplete, rep.Status, rep.Reason)
	require.Len(t, rep.Claims, 2)
	assert.Equal(t, report.VerdictStale, rep.Claims[0].Verdict)
	assert.Equal(t, "$25 per month", rep.Claims[0].CurrentValue)
	assert.Equal(t, report.VerdictFresh, rep.Claims[1].Verdict)
	assert.Equal(t, 2, rep.CheckedCount)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	assert.NoError(t, a.Stop(ctx))
}

func TestNewFailsLeftoverPendingReports(t *testing.T) {
	cfg := testConfig(t)
	st, err := store.OpenSQLite(cfg.DBPath)
	require.NoError(t, err)
	require.NoError(t, st.SaveReport(context.Background(), report.Report{ID: "r-old", CreatedAt: time.Now(), Status: report.StatusPending}))
	require.NoError(t, st.Close())

	a, err := app.New(cfg, slog.Default())
	require.NoError(t, err)
	defer a.Stop(context.Background())

	st, err = store.OpenSQLite(cfg.DBPath)
	require.NoError(t, err)
	defer st.Close()
	got, err := st.GetReport(context.Background(), "r-old")
	require.NoError(t, err)
	assert.Equal(t, report.StatusError, got.Status)
	assert.Equal(t, "The gateway stopped before verification finished.", got.Reason)
}

func TestNewFailsWhenDatabaseCannotOpen(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	cfg := testConfig(t)
	cfg.DBPath = filepath.Join(file, "freshness.db")
	_, err := app.New(cfg, slog.Default())
	assert.Error(t, err)
}

func TestStartReportsBusyPort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	cfg := testConfig(t)
	cfg.APIPort = l.Addr().(*net.TCPAddr).Port
	a, err := app.New(cfg, slog.Default())
	require.NoError(t, err)
	assert.Error(t, a.Start())
	assert.NoError(t, a.Stop(context.Background()))
}

func authorized(t *testing.T, method, url, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer t")
	return req
}

func TestAppStreamsEventsForACheck(t *testing.T) {
	cfg := testConfig(t)
	a, err := app.New(cfg, slog.Default())
	require.NoError(t, err)
	require.NoError(t, a.Start())
	defer a.Stop(context.Background())
	base := fmt.Sprintf("http://127.0.0.1:%d", cfg.APIPort)

	stream, err := http.DefaultClient.Do(authorized(t, http.MethodGet, base+"/freshness/events", ""))
	require.NoError(t, err)
	defer stream.Body.Close()
	lines := bufio.NewReader(stream.Body)
	first, err := lines.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, ": connected\n", first)

	body, _ := json.Marshal(map[string]string{"text": answer})
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(authorized(t, http.MethodPost, base+"/freshness/check", string(body)))
	require.NoError(t, err)
	var rep report.Report
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rep))
	resp.Body.Close()

	seen := map[string]int{}
	deadline := time.After(10 * time.Second)
	got := make(chan string)
	go func() {
		for {
			line, err := lines.ReadString('\n')
			if err != nil {
				close(got)
				return
			}
			got <- line
		}
	}()
	for seen["report.complete"] == 0 {
		select {
		case line, ok := <-got:
			require.True(t, ok, "stream ended early")
			if typ, found := strings.CutPrefix(strings.TrimSpace(line), "event: "); found {
				seen[typ]++
			}
		case <-deadline:
			t.Fatalf("no report.complete within 10 seconds; seen %v", seen)
		}
	}
	assert.Equal(t, 1, seen["report.started"])
	assert.Equal(t, 1, seen["report.claims"])
	assert.Equal(t, 2, seen["claim.verdict"])
	assert.NotEmpty(t, rep.ID)
}

func TestAppStatusReportsCredits(t *testing.T) {
	cfg := testConfig(t)
	a, err := app.New(cfg, slog.Default())
	require.NoError(t, err)
	require.NoError(t, a.Start())
	defer a.Stop(context.Background())
	resp, err := http.DefaultClient.Do(authorized(t, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/freshness/status", cfg.APIPort), ""))
	require.NoError(t, err)
	defer resp.Body.Close()
	var status struct {
		Credits struct {
			Budget int `json:"budget"`
		} `json:"credits"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&status))
	assert.Equal(t, cfg.DailyCreditBudget, status.Credits.Budget)
}
