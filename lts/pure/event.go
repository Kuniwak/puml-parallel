package pure

import (
	"io"
)

type Event string

var (
	EventIDTau  Event = "tau"
	EventIDTick Event = "tick"
)

func (e Event) IsTau() bool {
	return e == EventIDTau
}

func (e Event) IsTick() bool {
	return e == EventIDTick
}

func (e Event) String() string {
	return string(e)
}

func (e Event) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(e))
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}
