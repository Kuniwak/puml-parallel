package clause

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

// Fresh is a name that avoid does not hold: hint itself if it is free,
// otherwise hint followed by the least number from 2 that makes it free.
// A hint that ends in a digit takes "_" before the number, so that x1 and
// x12 are never confused.
func Fresh(avoid map[string]bool, hint string) string {
	if !avoid[hint] {
		return hint
	}
	sep := ""
	if last, _ := utf8.DecodeLastRuneInString(hint); unicode.IsDigit(last) {
		sep = "_"
	}
	for n := 2; ; n++ {
		if name := hint + sep + strconv.Itoa(n); !avoid[name] {
			return name
		}
	}
}
