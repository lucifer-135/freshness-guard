package extract_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/extract"
)

const answer = "Example Cloud Pro costs $20 per month, and its current version is 4.2."

func assertSpan(t *testing.T, wantStart, wantEnd int, wantOK bool, text, quote string) {
	t.Helper()
	start, end, ok := extract.Locate(text, quote)
	assert.Equal(t, wantStart, start, "start")
	assert.Equal(t, wantEnd, end, "end")
	assert.Equal(t, wantOK, ok, "ok")
}

func TestLocateExactASCII(t *testing.T) {
	assertSpan(t, 43, 69, true, answer, "its current version is 4.2")
}

func TestLocateUsesUTF16Offsets(t *testing.T) {
	assertSpan(t, 15, 25, true, "Preis: 20 € 😀 jetzt 25 €", "jetzt 25 €")
}

func TestLocateNormalizesWhitespaceCaseAndQuotes(t *testing.T) {
	assertSpan(t, 0, 29, true, "It’s  priced at\n$20 per MONTH.", "it's priced at $20 per month")
}

func TestLocateNormalizedMatchAfterNonASCII(t *testing.T) {
	// "😀 " is 3 UTF-16 units; the match starts after it and ends before "."
	assertSpan(t, 3, 19, true, "😀 Costs  “$25” now.", `costs "$25" now`)
}

func TestLocateNotFound(t *testing.T) {
	assertSpan(t, -1, -1, false, answer, "costs $30")
	assertSpan(t, -1, -1, false, answer, "")
}
