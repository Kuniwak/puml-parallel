package nl

import "io"

type Var string

func (v Var) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(v))
	return int64(n), err
}

var openParens = []byte{'('}
var closeParens = []byte{')'}
var sep = []byte{',', ' '}

func WriteVarsTo(w io.Writer, vars ...Var) (int64, error) {
	if len(vars) == 0 {
		return 0, nil
	}
	var n int64
	n2, err := w.Write(openParens)
	n += int64(n2)
	if err != nil {
		return n, err
	}

	for i, v := range vars {
		if i > 0 {
			n2, err = w.Write(sep)
			n += int64(n2)
			if err != nil {
				return n, err
			}
		}
		n3, err := v.WriteTo(w)
		n += n3
		if err != nil {
			return n, err
		}
	}
	n2, err = w.Write(closeParens)
	n += int64(n2)
	return n, err
}
