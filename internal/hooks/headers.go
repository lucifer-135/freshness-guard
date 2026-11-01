package hooks

import "github.com/maximhq/bifrost/core/schemas"

// Response header names and values set by the plugin.
const (
	HeaderStatus   = "X-Freshness-Status"
	HeaderReportID = "X-Freshness-Report-Id"
	StatusPending  = "pending"
	StatusSkipped  = "skipped"
	StatusError    = "error"
)

// SetHeader sets one response header on resp. A nil resp is ignored.
func SetHeader(resp *schemas.HTTPResponseMetadata, name, value string) {
	if resp == nil {
		return
	}
	resp.SetHeader(name, value)
}

// SetStatusHeader sets X-Freshness-Status on resp. A nil resp is ignored.
func SetStatusHeader(resp *schemas.HTTPResponseMetadata, status string) {
	SetHeader(resp, HeaderStatus, status)
}
