package heal_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/heal"
)

const answer = "Example Cloud Pro costs $20 per month, and its current version is 4.2."

func TestGuardAcceptsEditInsideSpan(t *testing.T) {
	ok, ratio := heal.Guard(answer, strings.Replace(answer, "$20", "$25", 1), [][2]int{{0, 37}}, []int{13})
	assert.True(t, ok)
	assert.Equal(t, 0.0, ratio)
}

func TestGuardRejectsRewrite(t *testing.T) {
	ok, _ := heal.Guard(answer, "Example Cloud is a cloud provider with many plans.", [][2]int{{0, 37}}, []int{13})
	assert.False(t, ok)
}

func TestGuardCountsOutsideChanges(t *testing.T) {
	original := strings.Repeat("a", 100)
	ok, ratio := heal.Guard(original, strings.Repeat("b", 10)+strings.Repeat("a", 90), nil, nil)
	assert.True(t, ok, "a replacement of 10 of 100 characters is exactly the limit")
	assert.InDelta(t, 0.10, ratio, 1e-9)

	ok, _ = heal.Guard(original, strings.Repeat("b", 11)+strings.Repeat("a", 89), nil, nil)
	assert.False(t, ok)

	ok, ratio = heal.Guard(original, original+strings.Repeat("c", 5), nil, nil)
	assert.True(t, ok)
	assert.InDelta(t, 0.05, ratio, 1e-9)
}

func TestGuardUsesUTF16Spans(t *testing.T) {
	ok, ratio := heal.Guard("😀 price $20 today", "😀 price $25 today", [][2]int{{9, 12}}, []int{3})
	assert.True(t, ok)
	assert.Equal(t, 0.0, ratio)
}

// An added explanation right after (or before) a corrected claim is the most
// common way a model ignores "do not add explanations".
func TestGuardLimitsInsertionsAtSpanBoundaries(t *testing.T) {
	appended := strings.Replace(answer, "$20 per month", "$25 per month (raised in September after a price review)", 1)
	ok, _ := heal.Guard(answer, appended, [][2]int{{0, 37}}, []int{len("$25 per month")})
	assert.False(t, ok, "a parenthetical after the span must count as a change outside it")

	prefixed := "Note that prices rose. " + strings.Replace(answer, "$20", "$25", 1)
	ok, _ = heal.Guard(answer, prefixed, [][2]int{{0, 37}}, []int{len("$25 per month")})
	assert.False(t, ok, "text added before the span must count as a change outside it")
}
