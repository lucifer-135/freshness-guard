package verify

import (
	"net/url"
	"strings"
	"unicode"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

var publishers = []string{
	"reuters.com", "apnews.com", "bbc.com", "bbc.co.uk", "nytimes.com", "wsj.com", "ft.com", "bloomberg.com",
	"theguardian.com", "economist.com", "cnbc.com", "theverge.com", "techcrunch.com", "arstechnica.com",
	"wired.com", "wikipedia.org",
}

// Authority rates a source: 1.0 for government, education and military sites,
// major publishers, and the entity's own site; 0.5 for everything else.
func Authority(rawURL, entity string) float64 {
	host := hostOf(rawURL)
	if host == "" {
		return 0.5
	}
	labels := strings.Split(host, ".")
	for _, l := range labels {
		if l == "gov" || l == "edu" || l == "mil" || l == "int" {
			return 1
		}
	}
	if len(labels) >= 2 && labels[len(labels)-2] == "ac" {
		return 1
	}
	for _, p := range publishers {
		if host == p || strings.HasSuffix(host, "."+p) {
			return 1
		}
	}

	words := strings.FieldsFunc(strings.ToLower(entity), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	joined := strings.Join(words, "")
	for _, l := range labels[:len(labels)-1] {
		if joined != "" && l == joined {
			return 1
		}
		for _, w := range words {
			if len(w) >= 4 && l == w {
				return 1
			}
		}
	}
	return 0.5
}

// TopicFor returns the Tavily topic for a claim category.
func TopicFor(c report.Category) string {
	if c == report.CategoryEvent || c == report.CategoryLeadership {
		return "news"
	}
	return "general"
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
