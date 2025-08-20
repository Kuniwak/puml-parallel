package nl

import (
	"io"
	"strings"
)

type Obligation []string

func (o Obligation) String() string {
	sb := &strings.Builder{}
	if _, err := o.WriteTo(sb); err != nil {
		panic(err)
	}
	return sb.String()
}

func (o Obligation) WriteTo(w io.Writer) (int64, error) {
	var n int64
	for _, s := range o {
		n2, err := io.WriteString(w, s)
		n += int64(n2)
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

const (
	andSep    = " and "
	orSep     = " or "
	notPrefix = "not "
)

func And(o1, o2 Obligation) Obligation {
	return append(append(append([]string{}, o1...), andSep), o2...)
}

func Or(o1, o2 Obligation) Obligation {
	return append(append(append([]string{}, o1...), orSep), o2...)
}

func Not(o Obligation) Obligation {
	return append([]string{notPrefix}, o...)
}
