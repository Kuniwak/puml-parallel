package nl

import (
	"io"
	"strings"
)

type StateGroup string

func (g StateGroup) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(g))
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

type State struct {
	Group StateGroup
	Vars  []Var
}

func (s State) Hash()

func (s State) WriteTo(w io.Writer) (int64, error) {
	n, err := s.Group.WriteTo(w)
	if err != nil {
		return n, err
	}
	if len(s.Vars) == 0 {
		return n, nil
	}
	n2, err := WriteVarsTo(w, s.Vars...)
	n += n2
	return n, err
}

func (s State) String() string {
	sb := &strings.Builder{}
	if _, err := s.WriteTo(sb); err != nil {
		panic(err)
	}
	return sb.String()
}
