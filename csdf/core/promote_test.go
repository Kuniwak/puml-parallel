package core

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

// The denotation of a promotion is written by hand below, independently of
// Promote and Expand (docs/CLAUSE_DESIGN.md): each instance is an LTS of its
// own, absent at 0, its events renamed e to e.i and a sync's local events to
// the sync's labels, and the instances run in alphabetised parallel
// (lts.Parallel). The labels are written as Expand writes them: the channel,
// then the IDs, then the fields, joined by ".".

// location: OPEN creates it with one item in stock of room for two; DEPART
// takes q away, ARRIVE brings q, and CLOSE deletes it once it is empty.
func location(t *testing.T) *Diagram {
	return &Diagram{
		States:   []string{"open"},
		Vars:     map[string][]string{"open": {"stock"}},
		Init:     "open",
		InitPost: mustParse(t, "$stock' = 1"),
		Edges: []Edge{
			{Src: "open", Dst: "open", Event: ev("DEPART", recv("q")), Guard: mustParse(t, "$stock >= $q"), Post: mustParse(t, "$stock' = $stock - $q")},
			{Src: "open", Dst: "open", Event: ev("ARRIVE", recv("q")), Guard: mustParse(t, "$stock + $q <= 2"), Post: mustParse(t, "$stock' = $stock + $q")},
		},
		Ends: []End{{Src: "open", Guard: mustParse(t, "$stock = 0")}},
	}
}

// shipment: NEW creates it waiting; it is moved, and BACK returns it to
// waiting, the state it started in, where it still exists.
func shipment(t *testing.T) *Diagram {
	return &Diagram{
		States: []string{"waiting", "moved"},
		Init:   "waiting",
		Edges: []Edge{
			{Src: "waiting", Dst: "moved", Event: ev("MOVE", recv("q"))},
			{Src: "moved", Dst: "waiting", Event: ev("BACK")},
		},
	}
}

// door: NEW creates it shut; it opens and shuts, coming back to the state
// it started in, and GONE deletes it while shut.
func door() *Diagram {
	return &Diagram{
		States: []string{"shut", "wide"},
		Init:   "shut",
		Edges: []Edge{
			{Src: "shut", Dst: "wide", Event: ev("OPENS")},
			{Src: "wide", Dst: "shut", Event: ev("SHUTS")},
		},
		Ends: []End{{Src: "shut"}},
	}
}

// doorInstance is the door x by hand: 0 is absent, 1 shut, 2 wide.
func doorInstance(x string) *lts.LTS {
	return handLTS(tr{0, "NEW." + x, 1}, tr{1, "OPENS." + x, 2}, tr{2, "SHUTS." + x, 1}, tr{1, "GONE." + x, 0})
}

var (
	locIDs  = []string{"a", "b"}
	shipIDs = []string{"s1"}
)

// locationInstance is the location x by hand: 0 is absent, 1+k open with k
// in stock.
func locationInstance(x string) *lts.LTS { return readingLocation(x, noReads) }

// reads are the suffixes a label of MOVE carries for what a global clause
// reads of the instances (docs/CLAUSE_DESIGN.md): given the role the
// instance plays and its stock, the suffixes it offers.
type reads func(role string, stock int) []string

func noReads(string, int) []string { return []string{""} }

func readingLocation(x string, r reads) *lts.LTS {
	l := handLTS(tr{0, "OPEN." + x, 2}, tr{1, "CLOSE." + x, 0})
	for _, s := range shipIDs {
		for _, other := range locIDs {
			if other == x {
				continue
			}
			for k := 0; k <= 2; k++ {
				for _, suffix := range r("from", k) {
					if k >= 1 {
						l.Add(1+k, "MOVE."+s+"."+x+"."+other+".1"+suffix, k)
					}
				}
				for _, suffix := range r("to", k) {
					if k+1 <= 2 {
						l.Add(1+k, "MOVE."+s+"."+other+"."+x+".1"+suffix, 2+k)
					}
				}
			}
		}
	}
	return l
}

// shipmentInstance is the shipment s by hand: 0 is absent, 1 waiting, 2
// moved.
func shipmentInstance(s string) *lts.LTS { return readingShipment(s, []string{""}) }

func readingShipment(s string, suffixes []string) *lts.LTS {
	l := handLTS(tr{0, "NEW." + s, 1}, tr{2, "BACK." + s, 1})
	for _, from := range locIDs {
		for _, to := range locIDs {
			if from == to {
				continue
			}
			for _, suffix := range suffixes {
				l.Add(1, "MOVE."+s+"."+from+"."+to+".1"+suffix, 2)
			}
		}
	}
	return l
}

// mentions says a label names the ID x among its values.
func mentions(x string) func(string) bool {
	return func(label string) bool {
		parts := strings.Split(label, ".")
		for _, p := range parts[1:] {
			if p == x {
				return true
			}
		}
		return false
	}
}

func stock(t *testing.T, families ...Family) Env {
	t.Helper()
	types := map[string][]Value{"stock": {0, 1, 2}, "q": {1}}
	for _, id := range locIDs {
		types["loc"] = append(types["loc"], id)
	}
	types["from"], types["to"] = types["loc"], types["loc"]
	for _, id := range shipIDs {
		types["s"] = append(types["s"], id)
	}
	env := Env{
		Types: types,
		Atoms: map[string]func([]Value) bool{
			"#0 = 1":       func(a []Value) bool { return a[0] == 1 },
			"#0 = 0":       func(a []Value) bool { return a[0] == 0 },
			"#0 >= #1":     func(a []Value) bool { return a[0].(int) >= a[1].(int) },
			"#0 = #1 - #2": func(a []Value) bool { return a[0] == a[1].(int)-a[2].(int) },
			"#0 + #1 <= 2": func(a []Value) bool { return a[0].(int)+a[1].(int) <= 2 },
			"#0 = #1 + #2": func(a []Value) bool { return a[0] == a[1].(int)+a[2].(int) },
		},
	}
	return envWith(t, env, families...)
}

// envWith is env with the type of each family's map, over the IDs env gives
// the family's ID variable.
func envWith(t *testing.T, env Env, families ...Family) Env {
	t.Helper()
	for _, f := range families {
		m, err := MapType(f, env.Types[f.ID], env)
		if err != nil {
			t.Fatal(err)
		}
		env.Types[f.Map] = m
	}
	return env
}

func TestPromoteRefuses(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: location(t), Create: "OPEN", Delete: "CLOSE"}
	cases := []struct {
		name string
		p    Promotion
		want string
	}{
		{"one instance in two roles", promotion(locations, Sync{Event: "MOVE", Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}, {Map: "locations", ID: "from", Event: "ARRIVE"}}}), "twice"},
		{"one local event taken twice", promotion(locations, Sync{Event: "MOVE", Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}, {Map: "locations", ID: "to", Event: "DEPART"}}}), "DEPART"},
		{"a sync on tau", promotion(locations, Sync{Event: Tau, Targets: []Target{{Map: "locations", ID: "loc", Event: "DEPART"}}}), "tau"},
		{"a target without its local event", promotion(locations, Sync{Event: "MOVE", Targets: []Target{{Map: "locations", ID: "loc"}}}), "names no local event"},
		{"a sync clause reading an instance the sync does not bind", promotion(locations, Sync{Event: "MOVE", Guard: mustParse(t, "${locations[loc].stock} = 0"), Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}}}), "locations[loc]"},
		{"a sync clause reading a name that is no map, target or field", promotion(locations, Sync{Event: "MOVE", Guard: mustParse(t, "$z = 0"), Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}}}), "z"},
		{"a target taking a local internal event", promotion(Family{Map: "lamps", ID: "l", Local: lamp(), Create: "NEW"}, Sync{Event: "SEE", Targets: []Target{
			{Map: "lamps", ID: "l", Event: Tau}}}), "internal"},
		{"a local edge sending what its instance does not hold", promotion(Family{Map: "beacons", ID: "b", Create: "NEW", Local: &Diagram{
			States: []string{"on"}, Init: "on", Edges: []Edge{{Src: "on", Dst: "on", Event: ev("PING", send("code"))}}}}), "code"},
		{"a local internal edge with fields", promotion(Family{Map: "beacons", ID: "b", Create: "NEW", Local: &Diagram{
			States: []string{"on"}, Init: "on", Edges: []Edge{{Src: "on", Dst: "on", Event: ev(Tau, recv("code"))}}}}), "fields"},
		{"a local diagram naming a state it does not declare", promotion(Family{Map: "beacons", ID: "b", Create: "NEW", Local: &Diagram{
			States: []string{"on"}, Init: "on", Edges: []Edge{{Src: "on", Dst: "off", Event: ev("PING")}}}}), "no state off"},
		{"a local clause that is not well formed", promotion(Family{Map: "beacons", ID: "b", Create: "NEW", Local: &Diagram{
			States: []string{"on"}, Init: "on", Edges: []Edge{{Src: "on", Dst: "on", Event: ev("PING"),
				Guard: clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpNe, Args: []clause.Term{clause.Free{Name: "x"}}}}}}}}}), "clause.WellFormed"},
		{"a sync clause that is not well formed", promotion(Family{Map: "locations", ID: "loc", Local: location(t), Create: "OPEN", Delete: "CLOSE"}, Sync{Event: "MOVE",
			Guard:   clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpNe, Args: []clause.Term{clause.Free{Name: "from"}}}}},
			Targets: []Target{{Map: "locations", ID: "from", Event: "DEPART"}}}), "clause.WellFormed"},
		{"a family without its creation event", promotion(Family{Map: "locations", ID: "loc", Local: location(t), Delete: "CLOSE"}), "no creation event"},
		{"end edges without a deletion event", promotion(Family{Map: "locations", ID: "loc", Local: location(t), Create: "OPEN"}), "no deletion event"},
		{"a creation event named like a local one", promotion(Family{Map: "locations", ID: "loc", Local: location(t), Create: "DEPART", Delete: "CLOSE"}), "DEPART"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, err := Promote(c.p)

			// Assert
			assertErrorContains(t, err, c.want)
		})
	}
}

func promotion(f Family, syncs ...Sync) Promotion {
	return Promotion{State: "G", Families: []Family{f}, Syncs: syncs}
}

// lamp: NEW creates it at a; from a it may go to c on GO or move to b
// internally, where DONE shows it.
func lamp() *Diagram {
	return &Diagram{
		States: []string{"a", "b", "c"},
		Init:   "a",
		Edges: []Edge{
			{Src: "a", Dst: "b", Event: ev(Tau)},
			{Src: "a", Dst: "c", Event: ev("GO")},
			{Src: "b", Dst: "b", Event: ev("DONE")},
		},
	}
}

// lampInstance is one lamp, x, written by hand.
func lampInstance(x string) *lts.LTS {
	return handLTS(
		tr{0, "NEW." + x, 1},
		tr{1, lts.Tau, 2},
		tr{1, "GO." + x, 3},
		tr{2, "DONE." + x, 2},
	)
}

func TestMapTypeIsEveryPartialMap(t *testing.T) {
	// Setup: one ID, absent or at one of two states, one holding a bit.
	f := Family{Map: "m", ID: "i", Local: &Diagram{
		States: []string{"p", "q"}, Vars: map[string][]string{"q": {"bit"}}, Init: "p",
	}}
	env := Env{Types: map[string][]Value{"bit": {0, 1}}}

	// Execute
	got, err := MapType(f, []Value{"x"}, env)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, v := range got {
		keys[Key(v)] = true
	}
	want := []string{"{}", "{x↦⟨p|⟩}", "{x↦⟨q|bit=0⟩}", "{x↦⟨q|bit=1⟩}"}
	if len(keys) != len(want) {
		t.Errorf("MapType = %v, want %v", keys, want)
	}
	for _, k := range want {
		if !keys[k] {
			t.Errorf("MapType lacks %s: %v", k, keys)
		}
	}
}

// tank: NEW creates it empty; FILL fills it; LOOK and SHOW send its level.
func tank(t *testing.T) *Diagram {
	return &Diagram{
		States:   []string{"t"},
		Vars:     map[string][]string{"t": {"level"}},
		Init:     "t",
		InitPost: mustParse(t, "$level' = 0"),
		Edges: []Edge{
			{Src: "t", Dst: "t", Event: ev("FILL"), Post: mustParse(t, "$level' = 1")},
			{Src: "t", Dst: "t", Event: ev("LOOK", send("level")), Post: mustParse(t, "$level' = $level")},
			{Src: "t", Dst: "t", Event: ev("SHOW", send("level")), Post: mustParse(t, "$level' = $level")},
		},
	}
}

// gauge: MAKE creates it; it receives a level on SHOW.
func gauge() *Diagram {
	return &Diagram{
		States: []string{"g"},
		Init:   "g",
		Edges:  []Edge{{Src: "g", Dst: "g", Event: ev("SHOW", recv("level"))}},
	}
}

// tankInstance is the tank t1 by hand: 1 holds level 0, 2 level 1; show
// gives the label of SHOW for a level.
func tankInstance(show func(level string) string) *lts.LTS {
	return handLTS(tr{0, "NEW.t1", 1}, tr{1, "FILL.t1", 2}, tr{2, "FILL.t1", 2},
		tr{1, "LOOK.t1.0", 1}, tr{2, "LOOK.t1.1", 2}, tr{1, show("0"), 1}, tr{2, show("1"), 2})
}

// gaugeInstance is the gauge g1 by hand.
func gaugeInstance(show func(level string) string) *lts.LTS {
	return handLTS(tr{0, "MAKE.g1", 1}, tr{1, show("0"), 1}, tr{1, show("1"), 1})
}

// tankEnv gives the tank and gauge examples their types and atoms.
func tankEnv(t *testing.T, families ...Family) Env {
	return envWith(t, Env{Types: map[string][]Value{"k": {"t1"}, "g": {"g1"}, "level": {0, 1}},
		Atoms: map[string]func([]Value) bool{
			"#0 = 0":  func(a []Value) bool { return a[0] == 0 },
			"#0 = 1":  func(a []Value) bool { return a[0] == 1 },
			"#0 = #1": func(a []Value) bool { return a[0] == a[1] },
		}}, families...)
}

// glancer: a gauge receiving a level on PEEK and on GLANCE.
func glancer() *Diagram {
	return &Diagram{States: []string{"g"}, Init: "g", Edges: []Edge{
		{Src: "g", Dst: "g", Event: ev("PEEK", recv("level"))},
		{Src: "g", Dst: "g", Event: ev("GLANCE", recv("level"))},
	}}
}

// glancerInstance is the gauge x by hand, peeking with y in either role.
func glancerInstance(x, y string) *lts.LTS {
	return handLTS(tr{0, "MAKE." + x, 1},
		tr{1, "PEEK." + x + "." + y + ".0", 1}, tr{1, "PEEK." + x + "." + y + ".1", 1},
		tr{1, "PEEK." + y + "." + x + ".0", 1}, tr{1, "PEEK." + y + "." + x + ".1", 1})
}

// readingDenotation is the denotation of a MOVE taken only into an empty
// location, by hand: the instance bound to to offers MOVE carrying its
// stock, the others offer every value; the global clause takes only 0; and
// a many-to-one renaming forgets what was carried (#79).
func readingDenotation(t *testing.T) *lts.LTS {
	carry := func(k int) string { return "|to.stock=" + string(rune('0'+k)) }
	any := []string{carry(0), carry(1), carry(2)}
	r := func(role string, k int) []string {
		if role == "to" {
			return []string{carry(k)}
		}
		return any
	}
	global := lts.New(0)
	for _, from := range locIDs {
		for _, to := range locIDs {
			if from != to {
				global.Add(0, "MOVE.s1."+from+"."+to+".1"+carry(0), 0)
			}
		}
	}
	carried := func(label string) bool { return strings.Contains(label, "|") }
	joint := lts.Parallel(
		[]*lts.LTS{readingLocation("a", r), readingLocation("b", r), readingShipment("s1", any), global},
		[]func(string) bool{mentions("a"), mentions("b"), mentions("s1"), carried})
	want, err := lts.Rename(joint, func(label string) []string {
		forgotten, _, _ := strings.Cut(label, "|")
		return []string{forgotten}
	})
	if err != nil {
		t.Fatal(err)
	}
	return want
}

func TestPromoteIsTheParallelOfTheInstances(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: location(t), Create: "OPEN", Delete: "CLOSE"}
	shipments := Family{Map: "shipments", ID: "s", Local: shipment(t), Create: "NEW"}
	doors := Family{Map: "doors", ID: "loc", Local: door(), Create: "NEW", Delete: "GONE"}
	lamps := Family{Map: "lamps", ID: "l", Local: lamp(), Create: "NEW"}
	tanks := Family{Map: "tanks", ID: "k", Local: tank(t), Create: "NEW"}
	gauges := Family{Map: "gauges", ID: "g", Local: gauge(), Create: "MAKE"}
	glancers := Family{Map: "gauges", ID: "g", Local: glancer(), Create: "MAKE"}
	tankTarget, gaugeTarget := Target{Map: "tanks", ID: "k", Event: "SHOW"}, Target{Map: "gauges", ID: "g", Event: "SHOW"}
	tankFirst := func(level string) string { return "SHOW.t1.g1." + level }
	gaugeFirst := func(level string) string { return "SHOW.g1.t1." + level }
	move := []Target{
		{Map: "shipments", ID: "s", Event: "MOVE"},
		{Map: "locations", ID: "from", Event: "DEPART"},
		{Map: "locations", ID: "to", Event: "ARRIVE"},
	}
	cases := []struct {
		name string
		p    Promotion
		env  func(t *testing.T) Env
		want func(t *testing.T) *lts.LTS
	}{
		{
			name: "instances are created, return to their start state while they exist, and are deleted",
			p:    Promotion{State: "G", Families: []Family{doors}},
			env:  func(t *testing.T) Env { return stock(t, doors) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{doorInstance("a"), doorInstance("b")}, []func(string) bool{mentions("a"), mentions("b")})
			},
		},
		{
			name: "a sync moves two instances of one family (#65)",
			p:    Promotion{State: "G", Families: []Family{locations, shipments}, Syncs: []Sync{{Event: "MOVE", Targets: move}}},
			env:  func(t *testing.T) Env { return stock(t, locations, shipments) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel(
					[]*lts.LTS{locationInstance("a"), locationInstance("b"), shipmentInstance("s1")},
					[]func(string) bool{mentions("a"), mentions("b"), mentions("s1")})
			},
		},
		{
			// With x at c and y at a, only y may move to b.
			name: "an internal event moves only an instance that can take it",
			p:    Promotion{State: "G", Families: []Family{lamps}},
			env:  func(t *testing.T) Env { return envWith(t, Env{Types: map[string][]Value{"l": {"x", "y"}}}, lamps) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{lampInstance("x"), lampInstance("y")}, []func(string) bool{mentions("x"), mentions("y")})
			},
		},
		{
			name: "a sync's fields merged by name: a send before a receive fixes the value",
			p:    Promotion{State: "G", Families: []Family{tanks, gauges}, Syncs: []Sync{{Event: "SHOW", Targets: []Target{tankTarget, gaugeTarget}}}},
			env:  func(t *testing.T) Env { return tankEnv(t, tanks, gauges) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{tankInstance(tankFirst), gaugeInstance(tankFirst)}, []func(string) bool{mentions("t1"), mentions("g1")})
			},
		},
		{
			name: "a sync's fields merged by name: a receive before a send reads it",
			p:    Promotion{State: "G", Families: []Family{tanks, gauges}, Syncs: []Sync{{Event: "SHOW", Targets: []Target{gaugeTarget, tankTarget}}}},
			env:  func(t *testing.T) Env { return tankEnv(t, tanks, gauges) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{tankInstance(gaugeFirst), gaugeInstance(gaugeFirst)}, []func(string) bool{mentions("t1"), mentions("g1")})
			},
		},
		{
			name: "two receives of one name are one value",
			p: Promotion{State: "G", Families: []Family{glancers}, Syncs: []Sync{{Event: "PEEK", Targets: []Target{
				{Map: "gauges", ID: "g", Event: "PEEK"}, {Map: "gauges", ID: "h", Event: "GLANCE"}}}}},
			env: func(t *testing.T) Env {
				return envWith(t, Env{Types: map[string][]Value{"g": {"g1", "g2"}, "h": {"g1", "g2"}, "level": {0, 1}}}, glancers)
			},
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{glancerInstance("g1", "g2"), glancerInstance("g2", "g1")}, []func(string) bool{mentions("g1"), mentions("g2")})
			},
		},
		{
			name: "what a sync clause reads is carried by the event (#79)",
			p: Promotion{State: "G", Families: []Family{locations, shipments},
				Syncs: []Sync{{Event: "MOVE", Guard: mustParse(t, "${locations[to].stock} = 0"), Targets: move}}},
			env:  func(t *testing.T) Env { return stock(t, locations, shipments) },
			want: readingDenotation,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			env, want := c.env(t), c.want(t)

			// Execute
			promoted, err := Promote(c.p)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Expand(promoted, env)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if !lts.StableFailuresEqual(got, want) {
				t.Errorf("not stable-failures equivalent: got %d states, want %d", len(got.Trans), len(want.Trans))
			}
		})
	}
}
