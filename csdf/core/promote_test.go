package core

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

// The denotation of a promotion is written by hand below, independently of
// Promote and Expand (docs/CLAUSE_DESIGN.md): each instance is an LTS of its
// own, at 0 while it does not exist, which is the local start state, its events renamed e to e.i and a sync's local events to
// the sync's labels, and the instances run in alphabetised parallel
// (lts.Parallel). The labels are written as Expand writes them: the channel,
// then the IDs, then the fields, joined by ".".

// location: OPEN creates it with one item in stock of room for two; DEPART
// takes q away, ARRIVE brings q, and CLOSE deletes it once it is empty.
func location(t *testing.T) *Diagram {
	return &Diagram{
		States: []string{"none", "open"},
		Vars:   map[string][]string{"open": {"stock"}},
		Init:   "none",
		Edges: []Edge{
			{Src: "none", Dst: "open", Event: ev("OPEN"), Post: mustParse(t, "$stock' = 1")},
			{Src: "open", Dst: "open", Event: ev("DEPART", recv("q")), Guard: mustParse(t, "$stock >= $q"), Post: mustParse(t, "$stock' = $stock - $q")},
			{Src: "open", Dst: "open", Event: ev("ARRIVE", recv("q")), Guard: mustParse(t, "$stock + $q <= 2"), Post: mustParse(t, "$stock' = $stock + $q")},
			{Src: "open", Dst: "none", Event: ev("CLOSE"), Guard: mustParse(t, "$stock = 0")},
		},
	}
}

// shipment: NEW creates it waiting; it is moved, and BACK returns it to
// waiting.
func shipment(t *testing.T) *Diagram {
	return &Diagram{
		States: []string{"none", "waiting", "moved"},
		Init:   "none",
		Edges: []Edge{
			{Src: "none", Dst: "waiting", Event: ev("NEW")},
			{Src: "waiting", Dst: "moved", Event: ev("MOVE", recv("q"))},
			{Src: "moved", Dst: "waiting", Event: ev("BACK")},
		},
	}
}

// door: NEW creates it shut; it opens and shuts, coming back to shut, and
// GONE deletes it while shut, leading back to the start state.
func door() *Diagram {
	return &Diagram{
		States: []string{"none", "shut", "wide"},
		Init:   "none",
		Edges: []Edge{
			{Src: "none", Dst: "shut", Event: ev("NEW")},
			{Src: "shut", Dst: "wide", Event: ev("OPENS")},
			{Src: "wide", Dst: "shut", Event: ev("SHUTS")},
			{Src: "shut", Dst: "none", Event: ev("GONE")},
		},
	}
}

// doorInstance is the door x by hand: 0 is absent, 1 shut, 2 wide.
func doorInstance(x string) *lts.LTS {
	return handLTS(tr{0, "NEW." + x, 1}, tr{1, "OPENS." + x, 2}, tr{2, "SHUTS." + x, 1}, tr{1, "GONE." + x, 0})
}

// key: CUT creates it held, and LOST deletes it.
func key() *Diagram {
	return &Diagram{
		States: []string{"none", "held"},
		Init:   "none",
		Edges: []Edge{
			{Src: "none", Dst: "held", Event: ev("CUT")},
			{Src: "held", Dst: "none", Event: ev("LOST")},
		},
	}
}

// fittedDoor is the door x by hand when INSTALL creates it together with a
// key and SCRAP deletes it together with one: 0 is absent, 1 shut, 2 wide.
func fittedDoor(x string) *lts.LTS {
	l := handLTS(tr{1, "OPENS." + x, 2}, tr{2, "SHUTS." + x, 1})
	for _, k := range shipIDs {
		l.Add(0, "INSTALL."+x+"."+k, 1)
		l.Add(1, "SCRAP."+x+"."+k, 0)
	}
	return l
}

// fittedKey is the key k by hand: 0 is absent, 1 held.
func fittedKey(k string) *lts.LTS {
	l := lts.New(0)
	for _, x := range locIDs {
		l.Add(0, "INSTALL."+x+"."+k, 1)
		l.Add(1, "SCRAP."+x+"."+k, 0)
	}
	return l
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

// beacon is a local diagram of one state on besides its start state none,
// with edges on it.
func beacon(edges ...Edge) *Diagram {
	return &Diagram{States: []string{"none", "on"}, Init: "none",
		Edges: append([]Edge{{Src: "none", Dst: "on", Event: ev("NEW")}}, edges...)}
}

func TestPromoteRefuses(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: location(t)}
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
		{"a target taking a local internal event", promotion(Family{Map: "lamps", ID: "l", Local: lamp()}, Sync{Event: "SEE", Targets: []Target{
			{Map: "lamps", ID: "l", Event: Tau}}}), "internal"},
		{"a local edge sending what its instance does not hold", promotion(Family{Map: "beacons", ID: "b", Local: beacon(
			Edge{Src: "on", Dst: "on", Event: ev("PING", send("code"))})}), "code"},
		{"a local internal edge with fields", promotion(Family{Map: "beacons", ID: "b", Local: beacon(
			Edge{Src: "on", Dst: "on", Event: ev(Tau, recv("code"))})}), "fields"},
		{"a local diagram naming a state it does not declare", promotion(Family{Map: "beacons", ID: "b", Local: beacon(
			Edge{Src: "on", Dst: "off", Event: ev("PING")})}), "no state off"},
		{"a local clause that is not well formed", promotion(Family{Map: "beacons", ID: "b", Local: beacon(
			Edge{Src: "on", Dst: "on", Event: ev("PING"),
				Guard: clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpNe, Args: []clause.Term{clause.Free{Name: "x"}}}}}})}), "clause.WellFormed"},
		{"a sync clause that is not well formed", promotion(locations, Sync{Event: "MOVE",
			Guard:   clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpNe, Args: []clause.Term{clause.Free{Name: "from"}}}}},
			Targets: []Target{{Map: "locations", ID: "from", Event: "DEPART"}}}), "clause.WellFormed"},
		{"a start state holding variables", promotion(Family{Map: "beacons", ID: "b", Local: &Diagram{
			States: []string{"none", "on"}, Vars: map[string][]string{"none": {"code"}}, Init: "none",
			Edges: []Edge{{Src: "none", Dst: "on", Event: ev("NEW")}}}}), "holds no state variables"},
		{"a self-loop on the start state", promotion(Family{Map: "beacons", ID: "b", Local: beacon(
			Edge{Src: "none", Dst: "none", Event: ev("PING")})}), "self-loop"},
		{"a constrain matching no generated edge", Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations},
			Constrains: []Constrain{{Event: ev("NOPE", recv("loc")), Guard: mustParse(t, "$loc is a")}}}, "no generated edge"},
		{"a constrain with a sent field", Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations},
			Constrains: []Constrain{{Event: ev("DEPART", recv("loc"), send("q")), Guard: mustParse(t, "$q = 1")}}}, "every field of a global event is ?"},
		{"a constrain reading a primed name", Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations},
			Constrains: []Constrain{{Event: ev("OPEN", recv("loc")), Guard: mustParse(t, "${locations'} = 0")}}}, "primed"},
		{"a constrain with an empty guard", Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations},
			Constrains: []Constrain{{Event: ev("OPEN", recv("loc"))}}}, "empty guard"},
		{"a constrain whose channel has edges of another shape", Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations},
			Constrains: []Constrain{{Event: ev("DEPART", recv("loc")), Guard: mustParse(t, "$loc = 0")}}}, "another shape"},
		{"a constrain reading a name that is no map and no field", Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations},
			Constrains: []Constrain{{Event: ev("OPEN", recv("loc")), Guard: mustParse(t, "$z = 0")}}}, "z"},
		{"a hand-written edge into no mode", modes(locations, Edge{Src: "G", Dst: "Z", Event: ev("HALT")}), "no mode Z"},
		{"a hand-written edge on an instance's channel", modes(locations, Edge{Src: "G", Dst: "H", Event: ev("OPEN")}), "channel of its own"},
		{"a hand-written edge with a post other than the frame", modes(locations, Edge{Src: "G", Dst: "H", Event: ev("HALT"), Post: mustParse(t, "@empty(locations')")}), "other than the frame"},
		{"a hand-written edge sending a field", modes(locations, Edge{Src: "G", Dst: "H", Event: ev("HALT", send("why"))}), "holds nothing to send"},
		{"a hand-written edge reading a primed name", modes(locations, Edge{Src: "G", Dst: "H", Event: ev("HALT"), Guard: mustParse(t, "@empty(locations')")}), "primed"},
		{"a hand-written edge reading a name that is no map and no field", modes(locations, Edge{Src: "G", Dst: "H", Event: ev("HALT"), Guard: mustParse(t, "$z = 0")}), "z"},
		{"a family frozen in no mode", Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{{Map: "locations", ID: "loc", Local: location(t), Frozen: []string{"Z"}}}}, "frozen in Z"},
		{"a local diagram assuming", promotion(Family{Map: "beacons", ID: "b", Local: func() *Diagram {
			d := beacon()
			d.Assumes = []Assumption{{ID: "a1", Text: "it is lit"}}
			return d
		}()}), "about the whole system"},
		{"a global start post saying more than that every map is empty", func() Promotion {
			p := promotion(locations)
			p.InitPost = mustParse(t, "locations は空")
			return p
		}(), "global start edge"},
		{"an end edge", promotion(Family{Map: "beacons", ID: "b", Local: &Diagram{
			States: []string{"none", "on"}, Init: "none",
			Edges: []Edge{{Src: "none", Dst: "on", Event: ev("NEW")}}, Ends: []End{{Src: "on"}}}}), "end edge"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, _, err := Promote(c.p)

			// Assert
			assertErrorContains(t, err, c.want)
		})
	}
}

func TestPromoteWarnsOfAPostItDrops(t *testing.T) {
	cases := []struct {
		name  string
		local func(t *testing.T) *Diagram
		want  []Warning
	}{
		{"none", func(*testing.T) *Diagram { return door() }, nil},
		{"the post of the start edge", func(t *testing.T) *Diagram {
			d := door()
			d.InitPost = mustParse(t, "nothing is there")
			return d
		}, []Warning{{Map: "doors", Event: "[*]", Text: "the post of the start edge says nothing: the start state means that the instance does not exist"}}},
		{"the post of a deletion", func(t *testing.T) *Diagram {
			d := door()
			for i, e := range d.Edges {
				if e.Event.Channel == "GONE" {
					d.Edges[i].Post = mustParse(t, "it is gone")
				}
			}
			return d
		}, []Warning{{Map: "doors", Event: "GONE", Text: "the post of a deletion says nothing: the instance no longer exists"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			p := promotion(Family{Map: "doors", ID: "d", Local: c.local(t)})

			// Execute
			_, got, err := Promote(p)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("warnings (-want +got):\n%s", diff)
			}
		})
	}
}

// modes is a promotion of f in the modes G and H, with edges between them.
func modes(f Family, edges ...Edge) Promotion {
	return Promotion{Modes: []string{"G", "H"}, Init: "G", Families: []Family{f}, Edges: edges}
}

func TestPromoteAcceptsAGlobalStartPostSayingEveryMapIsEmpty(t *testing.T) {
	// Setup
	p := promotion(Family{Map: "doors", ID: "loc", Local: door()})
	p.InitPost = mustParse(t, "@empty(doors')")

	// Execute
	got, warnings, err := Promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if diff := cmp.Diff(mustParse(t, "@empty(doors')"), got.InitPost); diff != "" {
		t.Errorf("start post (-want +got):\n%s", diff)
	}
}

func TestPromoteReportsAFrozenFamilyAsInfo(t *testing.T) {
	// Setup
	p := Promotion{Modes: []string{"run", "maint"}, Init: "run",
		Families: []Family{{Map: "doors", ID: "loc", Local: door(), Frozen: []string{"maint"}}}}

	// Execute
	_, got, err := Promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := []Warning{{Level: Info, Map: "doors", Text: "frozen in maint: it takes no event there"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
}

func TestPromoteComparesAHandWrittenChannelWithTheGlobalOnes(t *testing.T) {
	// Setup: DEPART is taken only through the sync MOVE, so the global
	// diagram has no edge on DEPART.
	p := Promotion{Modes: []string{"G", "H"}, Init: "G",
		Families: []Family{{Map: "locations", ID: "loc", Local: location(t)}},
		Syncs: []Sync{{Event: "MOVE", Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}, {Map: "locations", ID: "to", Event: "ARRIVE"}}}},
		Edges: []Edge{{Src: "G", Dst: "H", Event: ev("DEPART")}}}

	// Execute
	_, _, err := Promote(p)

	// Assert
	if err != nil {
		t.Errorf("Promote: %v, want a hand-written edge on a channel no global edge carries", err)
	}
}

func TestPromoteCarriesTheAssumptions(t *testing.T) {
	// Setup
	p := promotion(Family{Map: "doors", ID: "loc", Local: door()})
	p.Assumes = []Assumption{{ID: "fair", Text: "every door is eventually shut"}}

	// Execute
	got, _, err := Promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(p.Assumes, got.Assumes); diff != "" {
		t.Errorf("assumptions (-want +got):\n%s", diff)
	}
}

func promotion(f Family, syncs ...Sync) Promotion {
	return Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{f}, Syncs: syncs}
}

// lamp: NEW creates it at a; from a it may go to c on GO or move to b
// internally, where DONE shows it.
func lamp() *Diagram {
	return &Diagram{
		States: []string{"none", "a", "b", "c"},
		Init:   "none",
		Edges: []Edge{
			{Src: "none", Dst: "a", Event: ev("NEW")},
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
	// Setup: one ID, absent (at the start state o) or at one of two states,
	// one holding a bit.
	f := Family{Map: "m", ID: "i", Local: &Diagram{
		States: []string{"o", "p", "q"}, Vars: map[string][]string{"q": {"bit"}}, Init: "o",
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
		States: []string{"none", "t"},
		Vars:   map[string][]string{"t": {"level"}},
		Init:   "none",
		Edges: []Edge{
			{Src: "none", Dst: "t", Event: ev("NEW"), Post: mustParse(t, "$level' = 0")},
			{Src: "t", Dst: "t", Event: ev("FILL"), Post: mustParse(t, "$level' = 1")},
			{Src: "t", Dst: "t", Event: ev("LOOK", send("level")), Post: mustParse(t, "$level' = $level")},
			{Src: "t", Dst: "t", Event: ev("SHOW", send("level")), Post: mustParse(t, "$level' = $level")},
		},
	}
}

// gauge: MAKE creates it; it receives a level on SHOW.
func gauge() *Diagram {
	return &Diagram{
		States: []string{"none", "g"},
		Init:   "none",
		Edges: []Edge{{Src: "none", Dst: "g", Event: ev("MAKE")},
			{Src: "g", Dst: "g", Event: ev("SHOW", recv("level"))}},
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
	return &Diagram{States: []string{"none", "g"}, Init: "none", Edges: []Edge{
		{Src: "none", Dst: "g", Event: ev("MAKE")},
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

// doorsWatched is doors a and b, by hand, in parallel with a watcher that
// follows every NEW and offers what it allows: the denotation of a
// constrain on NEW.
func doorsWatched(watcher *lts.LTS) *lts.LTS {
	onNew := func(label string) bool { return strings.HasPrefix(label, "NEW.") }
	return lts.Parallel([]*lts.LTS{doorInstance("a"), doorInstance("b"), watcher},
		[]func(string) bool{mentions("a"), mentions("b"), onNew})
}

// doorMonitored is doors a and b and key s1, by hand, with a monitor
// following which doors exist (bit 1 for a, 2 for b) and offering CUT only
// while none does: the denotation of a constrain on CUT reading doors.
func doorMonitored(*testing.T) *lts.LTS {
	monitor := lts.New(0)
	for set := 0; set < 4; set++ {
		for i, x := range locIDs {
			bit := 1 << i
			if set&bit == 0 {
				monitor.Add(set, "NEW."+x, set|bit)
			} else {
				monitor.Add(set, "GONE."+x, set&^bit)
			}
		}
	}
	monitor.Add(0, "CUT.s1", 0)
	follows := func(label string) bool {
		return strings.HasPrefix(label, "NEW.") || strings.HasPrefix(label, "GONE.") || strings.HasPrefix(label, "CUT.")
	}
	keyInstance := handLTS(tr{0, "CUT.s1", 1}, tr{1, "LOST.s1", 0})
	return lts.Parallel([]*lts.LTS{doorInstance("a"), doorInstance("b"), keyInstance, monitor},
		[]func(string) bool{mentions("a"), mentions("b"), mentions("s1"), follows})
}

// doorsControlled is doors a and b, by hand, in parallel with a controller
// of the modes run (0) and maint (1) that follows which doors exist (bit 1
// for a, 2 for b): HALT leaves run only while no door exists, RESUME
// returns to run, and in a mode where frozen says the doors are frozen the
// controller offers none of their events.
func doorsControlled(frozen func(mode int) bool) *lts.LTS {
	controller := lts.New(0)
	state := func(mode, set int) int { return mode*4 + set }
	for mode := 0; mode < 2; mode++ {
		for set := 0; set < 4; set++ {
			if !frozen(mode) {
				for i, x := range locIDs {
					bit := 1 << i
					if set&bit == 0 {
						controller.Add(state(mode, set), "NEW."+x, state(mode, set|bit))
					} else {
						controller.Add(state(mode, set), "GONE."+x, state(mode, set&^bit))
						controller.Add(state(mode, set), "OPENS."+x, state(mode, set))
						controller.Add(state(mode, set), "SHUTS."+x, state(mode, set))
					}
				}
			}
		}
	}
	controller.Add(state(0, 0), "HALT", state(1, 0))
	for set := 0; set < 4; set++ {
		controller.Add(state(1, set), "RESUME", state(0, set))
	}
	all := func(string) bool { return true }
	return lts.Parallel([]*lts.LTS{doorInstance("a"), doorInstance("b"), controller},
		[]func(string) bool{mentions("a"), mentions("b"), all})
}

func TestPromoteIsTheParallelOfTheInstances(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: location(t)}
	shipments := Family{Map: "shipments", ID: "s", Local: shipment(t)}
	doors := Family{Map: "doors", ID: "loc", Local: door()}
	keys := Family{Map: "keys", ID: "s", Local: key()}
	lamps := Family{Map: "lamps", ID: "l", Local: lamp()}
	tanks := Family{Map: "tanks", ID: "k", Local: tank(t)}
	gauges := Family{Map: "gauges", ID: "g", Local: gauge()}
	glancers := Family{Map: "gauges", ID: "g", Local: glancer()}
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
			name: "an edge out of the start state creates an instance, and an edge into it deletes one",
			p:    Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{doors}},
			env:  func(t *testing.T) Env { return stock(t, doors) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{doorInstance("a"), doorInstance("b")}, []func(string) bool{mentions("a"), mentions("b")})
			},
		},
		{
			name: "a sync may take a creation and a deletion",
			p: Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{doors, keys}, Syncs: []Sync{
				{Event: "INSTALL", Targets: []Target{{Map: "doors", ID: "loc", Event: "NEW"}, {Map: "keys", ID: "s", Event: "CUT"}}},
				{Event: "SCRAP", Targets: []Target{{Map: "doors", ID: "loc", Event: "GONE"}, {Map: "keys", ID: "s", Event: "LOST"}}},
			}},
			env: func(t *testing.T) Env { return stock(t, doors, keys) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{fittedDoor("a"), fittedDoor("b"), fittedKey("s1")}, []func(string) bool{mentions("a"), mentions("b"), mentions("s1")})
			},
		},
		{
			name: "a sync moves two instances of one family (#65)",
			p:    Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations, shipments}, Syncs: []Sync{{Event: "MOVE", Targets: move}}},
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
			p:    Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{lamps}},
			env:  func(t *testing.T) Env { return envWith(t, Env{Types: map[string][]Value{"l": {"x", "y"}}}, lamps) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{lampInstance("x"), lampInstance("y")}, []func(string) bool{mentions("x"), mentions("y")})
			},
		},
		{
			name: "a sync's fields merged by name: a send before a receive fixes the value",
			p:    Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{tanks, gauges}, Syncs: []Sync{{Event: "SHOW", Targets: []Target{tankTarget, gaugeTarget}}}},
			env:  func(t *testing.T) Env { return tankEnv(t, tanks, gauges) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{tankInstance(tankFirst), gaugeInstance(tankFirst)}, []func(string) bool{mentions("t1"), mentions("g1")})
			},
		},
		{
			name: "a sync's fields merged by name: a receive before a send reads it",
			p:    Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{tanks, gauges}, Syncs: []Sync{{Event: "SHOW", Targets: []Target{gaugeTarget, tankTarget}}}},
			env:  func(t *testing.T) Env { return tankEnv(t, tanks, gauges) },
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{tankInstance(gaugeFirst), gaugeInstance(gaugeFirst)}, []func(string) bool{mentions("t1"), mentions("g1")})
			},
		},
		{
			name: "two receives of one name are one value",
			p: Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{glancers}, Syncs: []Sync{{Event: "PEEK", Targets: []Target{
				{Map: "gauges", ID: "g", Event: "PEEK"}, {Map: "gauges", ID: "h", Event: "GLANCE"}}}}},
			env: func(t *testing.T) Env {
				return envWith(t, Env{Types: map[string][]Value{"g": {"g1", "g2"}, "h": {"g1", "g2"}, "level": {0, 1}}}, glancers)
			},
			want: func(*testing.T) *lts.LTS {
				return lts.Parallel([]*lts.LTS{glancerInstance("g1", "g2"), glancerInstance("g2", "g1")}, []func(string) bool{mentions("g1"), mentions("g2")})
			},
		},
		{
			name: "a constrain narrows the generated edges it matches",
			p: Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{doors},
				Constrains: []Constrain{{Event: ev("NEW", recv("loc")), Guard: mustParse(t, "$loc is a")}}},
			env: func(t *testing.T) Env {
				env := stock(t, doors)
				env.Atoms["#0 is a"] = func(a []Value) bool { return a[0] == "a" }
				return env
			},
			want: func(*testing.T) *lts.LTS { return doorsWatched(handLTS(tr{0, "NEW.a", 0})) },
		},
		{
			name: "a constrain matches by position, its field renamed to the edge's own",
			p: Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{doors},
				Constrains: []Constrain{{Event: ev("NEW", recv("x")), Guard: mustParse(t, "$x is a")}}},
			env: func(t *testing.T) Env {
				env := stock(t, doors)
				env.Atoms["#0 is a"] = func(a []Value) bool { return a[0] == "a" }
				return env
			},
			want: func(*testing.T) *lts.LTS { return doorsWatched(handLTS(tr{0, "NEW.a", 0})) },
		},
		{
			name: "a constrain's first field is the instance ID, the rest the local event's",
			p: Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{tanks},
				Constrains: []Constrain{{Event: ev("LOOK", recv("t"), recv("v")), Guard: mustParse(t, "$v = 1")}}},
			env: func(t *testing.T) Env { return tankEnv(t, tanks) },
			want: func(*testing.T) *lts.LTS {
				onLook := func(label string) bool { return strings.HasPrefix(label, "LOOK.") }
				return lts.Parallel([]*lts.LTS{tankInstance(func(l string) string { return "SHOW.t1." + l }), handLTS(tr{0, "LOOK.t1.1", 0})},
					[]func(string) bool{mentions("t1"), onLook})
			},
		},
		{
			name: "a constrain reads a map no instance of its event is in, as a monitor would",
			p: Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{doors, keys},
				Constrains: []Constrain{{Event: ev("CUT", recv("s")), Guard: mustParse(t, "@empty(doors)")}}},
			env:  func(t *testing.T) Env { return stock(t, doors, keys) },
			want: doorMonitored,
		},
		{
			name: "a hand-written global edge switches modes, reading the maps as they are",
			p: Promotion{Modes: []string{"run", "maint"}, Init: "run", Families: []Family{doors},
				Edges: []Edge{
					{Src: "run", Dst: "maint", Event: ev("HALT"), Guard: mustParse(t, "@empty(doors)")},
					{Src: "maint", Dst: "run", Event: ev("RESUME")},
				}},
			env:  func(t *testing.T) Env { return stock(t, doors) },
			want: func(*testing.T) *lts.LTS { return doorsControlled(func(int) bool { return false }) },
		},
		{
			name: "a hand-written edge may write the frame as its post",
			p: Promotion{Modes: []string{"run", "maint"}, Init: "run", Families: []Family{doors},
				Edges: []Edge{
					{Src: "run", Dst: "maint", Event: ev("HALT"), Guard: mustParse(t, "@empty(doors)"), Post: mustParse(t, "@eq(doors', doors)")},
					{Src: "maint", Dst: "run", Event: ev("RESUME"), Post: mustParse(t, "@eq(doors', doors)")},
				}},
			env:  func(t *testing.T) Env { return stock(t, doors) },
			want: func(*testing.T) *lts.LTS { return doorsControlled(func(int) bool { return false }) },
		},
		{
			name: "a frozen family takes no event in its mode",
			p: Promotion{Modes: []string{"run", "maint"}, Init: "run",
				Families: []Family{{Map: "doors", ID: "loc", Local: door(), Frozen: []string{"maint"}}},
				Edges: []Edge{
					{Src: "run", Dst: "maint", Event: ev("HALT"), Guard: mustParse(t, "@empty(doors)")},
					{Src: "maint", Dst: "run", Event: ev("RESUME")},
				}},
			env:  func(t *testing.T) Env { return stock(t, doors) },
			want: func(*testing.T) *lts.LTS { return doorsControlled(func(mode int) bool { return mode == 1 }) },
		},
		{
			name: "what a sync clause reads is carried by the event (#79)",
			p: Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations, shipments},
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
			promoted, _, err := Promote(c.p)
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
