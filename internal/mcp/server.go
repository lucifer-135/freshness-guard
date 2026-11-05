// Package mcp serves the check_freshness tool over the Model Context Protocol
// (Streamable HTTP), so agents can check their own answers. Bifrost connects to
// it as an MCP client and offers the tool through its /mcp gateway.
package mcp

import (
	"context"
	"net/http"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// Path is where the MCP endpoint is served on the plugin API.
const Path = "/freshness/mcp"

const serverVersion = "0.4.0"

// CheckFunc checks text the way POST /freshness/check does. done is false
// when the check outlasted the timeout; rep then carries only the ID. A
// returned error's message is shown to the agent as written.
type CheckFunc func(ctx context.Context, text string) (rep report.Report, done bool, err error)

// NewHandler returns the MCP endpoint with the check_freshness tool.
func NewHandler(check CheckFunc) http.Handler {
	s := server.NewMCPServer("freshness-guard", serverVersion,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	tool := mcpgo.NewTool("check_freshness",
		mcpgo.WithDescription("Checks the time-sensitive claims in a text against live web sources and returns a freshness score and a verdict for each claim."),
		mcpgo.WithString("text", mcpgo.Required(), mcpgo.Description("The answer text to check, up to 20000 characters.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		rep, done, err := check(ctx, req.GetString("text", ""))
		switch {
		case err != nil:
			return mcpgo.NewToolResultError(err.Error()), nil
		case !done:
			return mcpgo.NewToolResultText(PendingText(rep.ID)), nil
		case rep.Status == report.StatusError:
			msg := "The check failed."
			if rep.Reason != "" {
				msg = "The check failed: " + rep.Reason
			}
			return mcpgo.NewToolResultError(msg), nil
		}
		return mcpgo.NewToolResultStructured(rep, Summary(rep)), nil
	})
	// Stateless: Bifrost connects afresh for each call by default, so no
	// session needs to outlive a request.
	return server.NewStreamableHTTPServer(s, server.WithEndpointPath(Path), server.WithStateLess(true))
}
