package extract

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Locate finds quote in answer and returns its span as UTF-16 code-unit
// offsets, which match JavaScript string indexing. It tries an exact match
// first, then a match that ignores case, runs of whitespace, and typographic
// quotes and dashes. It returns (-1, -1, false) when the quote is not found.
func Locate(answer, quote string) (start, end int, ok bool) {
	if strings.TrimSpace(quote) == "" {
		return -1, -1, false
	}
	if i := strings.Index(answer, quote); i >= 0 {
		return utf16Len(answer[:i]), utf16Len(answer[:i+len(quote)]), true
	}

	normAnswer := normalize(answer)
	normQuote := normalize(quote)
	q := trimSpaces(normQuote.runes)
	if len(q) == 0 {
		return -1, -1, false
	}
	idx := indexRunes(normAnswer.runes, q)
	if idx < 0 {
		return -1, -1, false
	}
	startByte := normAnswer.starts[idx]
	endByte := normAnswer.ends[idx+len(q)-1]
	return utf16Len(answer[:startByte]), utf16Len(answer[:endByte]), true
}

// normalized is a normalized copy of a string with, for each rune, the byte
// range of the original text it came from.
type normalized struct {
	runes  []rune
	starts []int
	ends   []int
}

func normalize(s string) normalized {
	var n normalized
	for i, r := range s {
		// An invalid byte decodes as RuneError but is one byte wide.
		_, size := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			if last := len(n.runes) - 1; last >= 0 && n.runes[last] == ' ' {
				n.ends[last] = i + size
				continue
			}
			r = ' '
		}
		n.runes = append(n.runes, fold(r))
		n.starts = append(n.starts, i)
		n.ends = append(n.ends, i+size)
	}
	return n
}

func fold(r rune) rune {
	switch r {
	case '‘', '’':
		return '\''
	case '“', '”':
		return '"'
	case '–', '—':
		return '-'
	}
	return unicode.ToLower(r)
}

func trimSpaces(r []rune) []rune {
	for len(r) > 0 && r[0] == ' ' {
		r = r[1:]
	}
	for len(r) > 0 && r[len(r)-1] == ' ' {
		r = r[:len(r)-1]
	}
	return r
}

func indexRunes(haystack, needle []rune) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if l := utf16.RuneLen(r); l > 0 {
			n += l
		} else {
			n++
		}
	}
	return n
}
