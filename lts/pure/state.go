package pure

import (
	"io"
)

type State string

func (s State) String() string {
	return string(s)
}

func (s State) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(s))
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}
