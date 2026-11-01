package hooks_test

import (
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/hooks"
)

func TestSetStatusHeader(t *testing.T) {
	md := &schemas.HTTPResponseMetadata{}
	hooks.SetStatusHeader(md, hooks.StatusSkipped)
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
}

func TestSetStatusHeaderNilMetadataDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { hooks.SetStatusHeader(nil, hooks.StatusSkipped) })
}

func TestSetHeaderSetsValue(t *testing.T) {
	md := &schemas.HTTPResponseMetadata{}
	hooks.SetHeader(md, hooks.HeaderReportID, "r-1")
	assert.Equal(t, "r-1", md.Header("X-Freshness-Report-Id"))
}

func TestSetHeaderNilSafe(t *testing.T) {
	assert.NotPanics(t, func() { hooks.SetHeader(nil, "A", "b") })
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "pending", hooks.StatusPending)
	assert.Equal(t, "skipped", hooks.StatusSkipped)
	assert.Equal(t, "error", hooks.StatusError)
}
