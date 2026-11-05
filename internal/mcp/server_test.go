package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

func connect(t *testing.T, check CheckFunc) *client.Client {
	t.Helper()
	srv := httptest.NewServer(NewHandler(check))
	t.Cleanup(srv.Close)
	c, err := client.NewStreamableHttpClient(srv.URL + "/freshness/mcp")
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	require.NoError(t, c.Start(ctx))
	_, err = c.Initialize(ctx, mcpgo.InitializeRequest{Params: mcpgo.InitializeParams{
		ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcpgo.Implementation{Name: "test", Version: "1"},
	}})
	require.NoError(t, err)
	return c
}

func call(t *testing.T, c *client.Client, text string) *mcpgo.CallToolResult {
	t.Helper()
	res, err := c.CallTool(context.Background(), mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{
		Name: "check_freshness", Arguments: map[string]any{"text": text},
	}})
	require.NoError(t, err)
	return res
}

func firstText(t *testing.T, res *mcpgo.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, res.Content)
	tc, ok := mcpgo.AsTextContent(res.Content[0])
	require.True(t, ok, "first content is text")
	return tc.Text
}

func TestListToolsHasCheckFreshness(t *testing.T) {
	c := connect(t, func(context.Context, string) (report.Report, bool, error) { return report.Report{}, true, nil })
	res, err := c.ListTools(context.Background(), mcpgo.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Tools, 1)
	tool := res.Tools[0]
	assert.Equal(t, "check_freshness", tool.Name)
	assert.Equal(t, "Checks the time-sensitive claims in a text against live web sources and returns a freshness score and a verdict for each claim.", tool.Description)
	schema, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	assert.Contains(t, string(schema), `"required":["text"]`)
}

func TestCallReturnsSummaryAndReport(t *testing.T) {
	rep := report.Report{ID: "r1", Status: report.StatusComplete, Score: intPtr(80), CheckedCount: 1, TotalClaims: 1,
		Claims: []report.Claim{{Quote: "Example Pro costs $20.", Verdict: report.VerdictFresh, Checked: true}}}
	var got string
	c := connect(t, func(_ context.Context, text string) (report.Report, bool, error) {
		got = text
		return rep, true, nil
	})
	res := call(t, c, "Example Pro costs $20.")
	assert.False(t, res.IsError)
	assert.Equal(t, Summary(rep), firstText(t, res))
	assert.Equal(t, "Example Pro costs $20.", got)
	structured, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	assert.Contains(t, string(structured), `"id":"r1"`)
}

func TestCallReturnsCheckErrorAsToolError(t *testing.T) {
	c := connect(t, func(context.Context, string) (report.Report, bool, error) {
		return report.Report{}, false, errors.New("The text field must not be empty.")
	})
	res := call(t, c, "")
	assert.True(t, res.IsError)
	assert.Equal(t, "The text field must not be empty.", firstText(t, res))
}

func TestCallSaysPendingWithReportID(t *testing.T) {
	c := connect(t, func(context.Context, string) (report.Report, bool, error) {
		return report.Report{ID: "r1", Status: report.StatusPending}, false, nil
	})
	res := call(t, c, "Some text.")
	assert.False(t, res.IsError)
	assert.Equal(t, PendingText("r1"), firstText(t, res))
}

func TestCallReportsFailedCheck(t *testing.T) {
	reason := "The gateway stopped before verification finished."
	c := connect(t, func(context.Context, string) (report.Report, bool, error) {
		return report.Report{ID: "r1", Status: report.StatusError, Reason: reason}, true, nil
	})
	res := call(t, c, "Some text.")
	assert.True(t, res.IsError)
	assert.Equal(t, "The check failed: "+reason, firstText(t, res))

	c = connect(t, func(context.Context, string) (report.Report, bool, error) {
		return report.Report{ID: "r1", Status: report.StatusError}, true, nil
	})
	assert.Equal(t, "The check failed.", firstText(t, call(t, c, "Some text.")))
}

func TestCallSurvivesPanic(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	c := connect(t, func(context.Context, string) (report.Report, bool, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			panic("boom")
		}
		return report.Report{ID: "r2", Status: report.StatusComplete}, true, nil
	})
	res, err := c.CallTool(context.Background(), mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{
		Name: "check_freshness", Arguments: map[string]any{"text": "x"},
	}})
	assert.True(t, err != nil || res.IsError, "a panic becomes an error")
	res = call(t, c, "x")
	assert.False(t, res.IsError)
	assert.Contains(t, firstText(t, res), "Report ID: r2.")
}
