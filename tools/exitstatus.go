package tools

// ExitStatusError is returned by a tool that hands its work to another
// process and wants that process's exit status as its own. Message, when not
// empty, is printed as the tool's own error; when empty the process has
// already said what went wrong, so nothing is printed.
type ExitStatusError struct {
	Status  int
	Message string
}

func (e *ExitStatusError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "exit status " + itoa(e.Status)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
