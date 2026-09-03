package tools

import (
	"fmt"
	"strings"
)

// GeneratedBy renders the command line that produced a generated diagram, so
// that the diagram name records how to reproduce it. Arguments are quoted the
// way a POSIX shell needs them, so the rendered line can be pasted back.
func GeneratedBy(command string, args []string) string {
	var sb strings.Builder
	sb.WriteString("auto-generated-by: ")
	sb.WriteString(command)
	for _, arg := range args {
		sb.WriteString(" ")
		sb.WriteString(shellQuote(arg))
	}
	return sb.String()
}

func shellQuote(s string) string {
	if s != "" && !strings.ContainsFunc(s, needsShellQuote) {
		return s
	}
	return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", `'\''`))
}

func needsShellQuote(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	case strings.ContainsRune("_-./=:@,+", r):
		return false
	default:
		return true
	}
}
