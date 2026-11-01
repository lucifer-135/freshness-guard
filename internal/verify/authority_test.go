package verify_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/verify"
)

func TestAuthority(t *testing.T) {
	cases := []struct {
		url, entity string
		want        float64
	}{
		{"https://www.irs.gov/x", "", 1.0},
		{"https://ox.ac.uk/a", "", 1.0},
		{"https://en.wikipedia.org/wiki/X", "", 1.0},
		{"https://www.reuters.com/a", "", 1.0},
		{"https://docs.github.com/copilot", "GitHub Copilot", 1.0},
		{"https://examplecloud.io/p", "Example Cloud", 1.0},
		{"https://cloudflare.com", "Example Cloud", 0.5},
		{"https://someblog.dev/p", "GitHub Copilot", 0.5},
		{"https://notreuters.com/a", "", 0.5},
		{"https://anyone.github.io/x", "GitHub", 0.5},
		{"https://cloud.someblog.com", "Example Cloud", 0.5},
		{"::bad", "x", 0.5},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, verify.Authority(tc.url, tc.entity), tc.url)
	}
}

func TestTopicFor(t *testing.T) {
	assert.Equal(t, "news", verify.TopicFor(report.CategoryEvent))
	assert.Equal(t, "news", verify.TopicFor(report.CategoryLeadership))
	assert.Equal(t, "general", verify.TopicFor(report.CategoryPricing))
	assert.Equal(t, "general", verify.TopicFor(report.CategoryVersion))
}
