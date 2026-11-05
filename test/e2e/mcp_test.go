//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeURL() string { return os.Getenv("FAKE_URL") }

// gatewayMCP connects to Bifrost's MCP gateway, which offers the tools of the
// MCP clients Bifrost is connected to, including the plugin's.
func gatewayMCP(t *testing.T) *mcpclient.Client {
	t.Helper()
	waitReady(t)
	c, err := mcpclient.NewStreamableHttpClient(bifrostURL() + "/mcp")
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, c.Start(ctx))
	_, err = c.Initialize(ctx, mcpgo.InitializeRequest{Params: mcpgo.InitializeParams{
		ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcpgo.Implementation{Name: "freshness-e2e", Version: "1"},
	}})
	require.NoError(t, err)
	return c
}

// checkFreshnessTool returns the name Bifrost lists the plugin's tool under.
// Bifrost lists tools only after it has connected to the MCP server, so the
// first seconds after startup may show none.
func checkFreshnessTool(t *testing.T, c *mcpclient.Client) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		res, err := c.ListTools(context.Background(), mcpgo.ListToolsRequest{})
		require.NoError(t, err)
		var names []string
		for _, tool := range res.Tools {
			names = append(names, tool.Name)
			if strings.HasSuffix(tool.Name, "check_freshness") {
				return tool.Name
			}
		}
		require.True(t, time.Now().Before(deadline), "Bifrost lists no check_freshness tool; tools: %v", names)
		time.Sleep(time.Second)
	}
}

func TestMCPGatewayListsCheckFreshness(t *testing.T) {
	name := checkFreshnessTool(t, gatewayMCP(t))
	t.Logf("Bifrost lists the tool as %q", name)
}

func TestMCPGatewayChecksText(t *testing.T) {
	c := gatewayMCP(t)
	name := checkFreshnessTool(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := c.CallTool(ctx, mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{
		Name: name, Arguments: map[string]any{"text": answer},
	}})
	require.NoError(t, err)
	require.NotEmpty(t, res.Content)
	text, ok := mcpgo.AsTextContent(res.Content[0])
	require.True(t, ok)
	require.False(t, res.IsError, text.Text)
	m := regexp.MustCompile(`Report ID: (\S+)\.`).FindStringSubmatch(text.Text)
	require.Len(t, m, 2, text.Text)
	assert.Equal(t, "complete", getReport(t, m[1]).Status)
}

// Bifrost must not add MCP tools to chat requests, including the plugin's own
// extract and judge calls, whose JSON answers the pipeline parses.
func TestChatRequestsCarryNoMCPTools(t *testing.T) {
	checkFreshnessTool(t, gatewayMCP(t)) // the MCP client is connected

	resp := postChat(t, `{"model":"openai/fake-model","messages":[{"role":"user","content":"What does Example Cloud Pro cost?"}]}`, nil)
	readBody(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	chatID := resp.Header.Get("X-Freshness-Report-Id")
	require.NotEmpty(t, chatID)

	body, _ := json.Marshal(map[string]string{"text": answer})
	checkResp := pluginRequest(t, http.MethodPost, "/freshness/check", string(body), true)
	var checked freshnessReport
	require.NoError(t, json.Unmarshal([]byte(readBody(t, checkResp)), &checked))
	assert.Equal(t, "complete", getReport(t, chatID).Status)
	assert.Equal(t, "complete", getReport(t, checked.ID).Status)

	countResp, err := http.Get(fakeURL() + "/debug/tool-requests")
	require.NoError(t, err)
	assert.JSONEq(t, `{"count":0}`, readBody(t, countResp))
}
