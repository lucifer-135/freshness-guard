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
