package tracechk_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
)

func TestMatch(t *testing.T) {
	insert := core.Event{Channel: "insert", Fields: []core.Field{{Name: "coin"}}}
	pay := core.Event{Channel: "pay", Fields: []core.Field{{Name: "coin"}, {Send: true, Name: "change"}}}
	tick := core.Event{Channel: "tick"}
	testCases := map[string]struct {
		Match tracechk.Match
		Event core.Event
		Trace string
		Want  bool
	}{
		"exact: the label itself":                        {tracechk.MatchExact, insert, "insert?coin", true},
		"exact: a value in place of the field's name":    {tracechk.MatchExact, insert, "insert?3", true},
		"exact: a sent value in place of the sent field": {tracechk.MatchExact, pay, "pay?5!2", true},
		"exact: a field of the other direction":          {tracechk.MatchExact, insert, "insert!3", false},
		"exact: too few fields":                          {tracechk.MatchExact, pay, "pay?5", false},
		"exact: another channel":                         {tracechk.MatchExact, insert, "inserted?3", false},
		"exact: an event without fields":                 {tracechk.MatchExact, tick, "tick", true},
		"prefix: the channel alone":                      {tracechk.MatchPrefix, insert, "insert", true},
		"prefix: the first of the fields":                {tracechk.MatchPrefix, pay, "pay?5", true},
		"prefix: stops at the channel's boundary":        {tracechk.MatchPrefix, insert, "ins", false},
		"prefix: a longer channel":                       {tracechk.MatchPrefix, tick, "ticks", false},
		"prefix: every field, as exact":                  {tracechk.MatchPrefix, pay, "pay?5!2", true},
		"prefix: a field the event does not have":        {tracechk.MatchPrefix, tick, "tick?1", false},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := tc.Match(tc.Event, tc.Trace); got != tc.Want {
				t.Errorf("match(%s, %q) = %v, want %v", tracechk.Label(tc.Event), tc.Trace, got, tc.Want)
			}
		})
	}
}
