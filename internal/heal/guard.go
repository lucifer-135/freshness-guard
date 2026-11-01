package heal

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// MaxOutsideChangeRatio is the largest share of the characters outside the
// corrected claims that an edit may change (spec 8.4).
const MaxOutsideChangeRatio = 0.10

// Guard checks that edited changes original only inside the given spans
// (UTF-16 offsets into original). It returns whether the edit is acceptable
// and the share of characters outside the spans that were changed. Each run
// of adjacent deletions and insertions counts as max(deleted, inserted), so a
// replaced character counts once.
func Guard(original, edited string, spans [][2]int) (ok bool, ratio float64) {
	runeSpans := toRuneSpans(original, spans)
	inSpan := func(pos int) bool {
		for _, s := range runeSpans {
			if pos >= s[0] && pos < s[1] {
				return true
			}
		}
		return false
	}
	// An insertion right at a span boundary belongs to the span.
	insertInSpan := func(pos int) bool {
		for _, s := range runeSpans {
			if pos >= s[0] && pos <= s[1] {
				return true
			}
		}
		return false
	}

	total := utf8.RuneCountInString(original)
	outside := 0
	for pos := range total {
		if !inSpan(pos) {
			outside++
		}
	}

	changed, deleted, inserted, pos := 0, 0, 0, 0
	flush := func() {
		changed += max(deleted, inserted)
		deleted, inserted = 0, 0
	}
	for _, d := range diffmatchpatch.New().DiffMain(original, edited, false) {
		n := utf8.RuneCountInString(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			flush()
			pos += n
		case diffmatchpatch.DiffDelete:
			for i := range n {
				if !inSpan(pos + i) {
					deleted++
				}
			}
			pos += n
		case diffmatchpatch.DiffInsert:
			if !insertInSpan(pos) {
				inserted += n
			}
		}
	}
	flush()

	ratio = float64(changed) / float64(max(1, outside))
	return ratio <= MaxOutsideChangeRatio, ratio
}

// toRuneSpans converts UTF-16 offsets into rune offsets.
func toRuneSpans(s string, spans [][2]int) [][2]int {
	// boundaries[i] is the UTF-16 offset where rune i starts.
	boundaries := []int{0}
	units := 0
	for _, r := range s {
		if l := utf16.RuneLen(r); l > 0 {
			units += l
		} else {
			units++
		}
		boundaries = append(boundaries, units)
	}
	runeIndex := func(offset int) int {
		for i, b := range boundaries {
			if b >= offset {
				return i
			}
		}
		return len(boundaries) - 1
	}
	out := make([][2]int, 0, len(spans))
	for _, sp := range spans {
		out = append(out, [2]int{runeIndex(sp[0]), runeIndex(sp[1])})
	}
	return out
}

// byteOffset converts a UTF-16 offset into a byte offset in s.
func byteOffset(s string, offset int) int {
	units := 0
	for i, r := range s {
		if units >= offset {
			return i
		}
		if l := utf16.RuneLen(r); l > 0 {
			units += l
		} else {
			units++
		}
	}
	return len(s)
}
