package hooks

import "github.com/maximhq/bifrost/core/schemas"

// Response header names and values set by the plugin.
const (
	HeaderStatus  = "X-Freshness-Status"
	StatusSkipped = "skipped"
	StatusError   = "error"
)

// SetStatusHeader sets X-Freshness-Status on resp. A nil resp is ignored.
func SetStatusHeader(resp *schemas.HTTPResponseMetadata, status string) {
	if resp == nil {
		return
	}
	resp.SetHeader(HeaderStatus, status)
}
