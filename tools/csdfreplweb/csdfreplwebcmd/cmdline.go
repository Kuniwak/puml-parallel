package csdfreplwebcmd

import (
	"errors"
	"strings"
)

// SplitCommandLine splits line into words as a POSIX shell does, without
// running one and without expanding anything: blanks separate words, single
// quotes keep what they enclose as it is, double quotes keep it but for a
// backslash escaping " or \, and a backslash outside quotes keeps the
// character after it.
func SplitCommandLine(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		case r == '\\':
			if i+1 == len(runes) {
				return nil, errors.New("csdfreplwebcmd.SplitCommandLine: the command line ends in a backslash")
			}
			i++
			word.WriteRune(runes[i])
			inWord = true
		case r == '\'':
			end := indexRune(runes, i+1, '\'')
			if end < 0 {
				return nil, errors.New("csdfreplwebcmd.SplitCommandLine: a single quote is not closed")
			}
			word.WriteString(string(runes[i+1 : end]))
			i = end
			inWord = true
		case r == '"':
			i++
			for ; i < len(runes) && runes[i] != '"'; i++ {
				if runes[i] == '\\' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\') {
					i++
				}
				word.WriteRune(runes[i])
			}
			if i == len(runes) {
				return nil, errors.New("csdfreplwebcmd.SplitCommandLine: a double quote is not closed")
			}
			inWord = true
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	if len(words) == 0 {
		return nil, errors.New("csdfreplwebcmd.SplitCommandLine: the command line is empty")
	}
	return words, nil
}

func indexRune(runes []rune, from int, target rune) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == target {
			return i
		}
	}
	return -1
}
