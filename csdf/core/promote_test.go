package core

import (
	"slices"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

// The denotation of promotion, built independently of Promote
// (docs/CLAUSE_DESIGN.md): each instance i of a family runs its local
// diagram from its start state, its own events renamed e to e.i; a sync
// renames each local event it takes to every global event the instance can
// play its role in; and the instances run in alphabetised parallel.

type role struct {
	target int
	local  string
}

func denotation(t *testing.T, p Promotion, env Env) *lts.LTS {
	t.Helper()
	var procs []*lts.LTS
	var alphabets []func(string) bool
	for _, f := range p.Families {
		local, err := Expand(f.Local, env)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range env.Types[f.ID] {
			images := map[string][]string{}
			for label := range labels(local) {
				images[label] = renamedLabels(p, f, id, label, env)
			}
			alpha := map[string]bool{}
			for _, ims := range images {
				for _, im := range ims {
					alpha[im] = true
				}
			}
			procs = append(procs, lts.Rename(local, func(l string) []string { return images[l] }))
			alphabets = append(alphabets, func(l string) bool { return alpha[l] })
		}
	}
	return lts.Parallel(procs, alphabets)
}

func labels(l *lts.LTS) map[string]bool {
	out := map[string]bool{}
	for _, byLabel := range l.Trans {
		for label := range byLabel {
			if label != lts.Tau {
				out[label] = true
			}
		}
	}
	return out
}

func renamedLabels(p Promotion, f Family, id Value, label string, env Env) []string {
	ch, rest, _ := strings.Cut(label, ".")
	var vals []string
	if rest != "" {
		vals = strings.Split(rest, ".")
	}
	var out []string
	synced := false
	for _, s := range p.Syncs {
		for k, tg := range s.Targets {
			if tg.Map != f.Map || tg.event(s) != ch {
				continue
			}
			synced = true
			out = append(out, syncLabels(p, s, k, id, localFields(f, ch), vals, env)...)
		}
	}
	if synced {
		return out
	}
	return []string{strings.Join(append([]string{ch, Key(id)}, vals...), ".")}
}

func localFields(f Family, channel string) []string {
	for _, e := range f.Local.Edges {
		if e.Event.Channel == channel {
			var names []string
			for _, fl := range e.Event.Fields {
				names = append(names, fl.Name)
			}
			return names
		}
	}
	return nil
}

// syncLabels are the labels of the sync s in which target k is played by
// id with its fields at vals.
func syncLabels(p Promotion, s Sync, k int, id Value, fields, vals []string, env Env) []string {
	given := map[string]string{}
	for i, n := range fields {
		given[n] = vals[i]
	}
	merged := syncFields(p, s)
	var out []string
	var walk func(i int, ids []Value)
	walk = func(i int, ids []Value) {
		if i == len(s.Targets) {
			for a := range ids {
				for b := range ids {
					if a < b && s.Targets[a].Map == s.Targets[b].Map && Key(ids[a]) == Key(ids[b]) {
						return
					}
				}
			}
			parts := []string{s.Event}
			for _, v := range ids {
				parts = append(parts, Key(v))
			}
			var fill func(j int, parts []string)
			fill = func(j int, parts []string) {
				if j == len(merged) {
					out = append(out, strings.Join(parts, "."))
					return
				}
				if v, ok := given[merged[j]]; ok {
					fill(j+1, append(slices.Clone(parts), v))
					return
				}
				for _, v := range env.Types[merged[j]] {
					fill(j+1, append(slices.Clone(parts), Key(v)))
				}
			}
			fill(0, parts)
			return
		}
		if i == k {
			walk(i+1, append(slices.Clone(ids), id))
			return
		}
		fam := family(p, s.Targets[i].Map)
		for _, v := range env.Types[fam.ID] {
			walk(i+1, append(slices.Clone(ids), v))
		}
	}
	walk(0, nil)
	return out
}

func syncFields(p Promotion, s Sync) []string {
	var out []string
	for _, tg := range s.Targets {
		for _, n := range localFields(family(p, tg.Map), tg.event(s)) {
			if !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}

func family(p Promotion, m string) Family {
	for _, f := range p.Families {
		if f.Map == m {
			return f
		}
	}
	panic("no family " + m)
}

// location: absent at l0, opened with one item in stock of room for two;
// DEPART takes an item away and ARRIVE brings one.
func location(t *testing.T) *Diagram {
	return &Diagram{
		Vars: map[string][]string{"l0": nil, "open": {"stock"}},
		Init: "l0",
		Edges: []Edge{
			{Src: "l0", Dst: "open", Event: ev("OPEN"), Post: mustParse(t, "$stock' = 1")},
			{Src: "open", Dst: "open", Event: ev("DEPART", recv("q")), Guard: mustParse(t, "$stock >= $q"), Post: mustParse(t, "$stock' = $stock - $q")},
			{Src: "open", Dst: "open", Event: ev("ARRIVE", recv("q")), Guard: mustParse(t, "$stock + $q <= 2"), Post: mustParse(t, "$stock' = $stock + $q")},
		},
	}
}

// shipment: absent at s0, created waiting, and moved once.
func shipment(t *testing.T) *Diagram {
	return &Diagram{
		Vars: map[string][]string{"s0": nil, "waiting": nil, "moved": nil},
		Init: "s0",
		Edges: []Edge{
			{Src: "s0", Dst: "waiting", Event: ev("NEW")},
			{Src: "waiting", Dst: "moved", Event: ev("MOVE", recv("q"))},
		},
	}
}

func stock() Env {
	recs := func(states map[string][]string, types map[string][]Value) []Value {
		var out []Value
		for s, vars := range states {
			fields := []map[string]Value{{}}
			for _, v := range vars {
				var grown []map[string]Value
				for _, f := range fields {
					for _, val := range types[v] {
						g := map[string]Value{}
						for k, w := range f {
							g[k] = w
						}
						g[v] = val
						grown = append(grown, g)
					}
				}
				fields = grown
			}
			for _, f := range fields {
				out = append(out, Rec{State: s, Fields: f})
			}
		}
		return out
	}
	maps := func(ids []Value, recs []Value) []Value {
		out := []Map{{}}
		for _, id := range ids {
			var grown []Map
			for _, m := range out {
				for _, r := range recs {
					g := Map{}
					for k, v := range m {
						g[k] = v
					}
					g[Key(id)] = r
					grown = append(grown, g)
				}
			}
			out = grown
		}
		vs := make([]Value, len(out))
		for i, m := range out {
			vs[i] = m
		}
		return vs
	}
	types := map[string][]Value{"stock": {0, 1, 2}, "q": {1}}
	locIDs := []Value{"a", "b"}
	shipIDs := []Value{"s1"}
	types["loc"], types["from"], types["to"] = locIDs, locIDs, locIDs
	types["s"] = shipIDs
	types["locations"] = maps(locIDs, recs(map[string][]string{"l0": nil, "open": {"stock"}}, types))
	types["shipments"] = maps(shipIDs, recs(map[string][]string{"s0": nil, "waiting": nil, "moved": nil}, types))
	return Env{
		Types: types,
		Atoms: map[string]func([]Value) bool{
			"#0 = 1":       func(a []Value) bool { return a[0] == 1 },
			"#0 >= #1":     func(a []Value) bool { return a[0].(int) >= a[1].(int) },
			"#0 = #1 - #2": func(a []Value) bool { return a[0] == a[1].(int)-a[2].(int) },
			"#0 + #1 <= 2": func(a []Value) bool { return a[0].(int)+a[1].(int) <= 2 },
			"#0 = #1 + #2": func(a []Value) bool { return a[0] == a[1].(int)+a[2].(int) },
		},
	}
}

func TestPromoteIsTheParallelOfTheInstances(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: location(t)}
	shipments := Family{Map: "shipments", ID: "s", Local: shipment(t)}
	cases := []struct {
		name string
		p    Promotion
	}{
		{"instances of one family interleave", Promotion{State: "G", Families: []Family{
			{Map: "shipments", ID: "s", Local: &Diagram{
				Vars: map[string][]string{"s0": nil, "waiting": nil, "moved": nil}, Init: "s0",
				Edges: []Edge{{Src: "s0", Dst: "waiting", Event: ev("NEW")}, {Src: "waiting", Dst: "moved", Event: ev("SHIP")}}}},
		}}},
		{"a sync moves two instances of one family (#65)", Promotion{State: "G",
			Families: []Family{locations, shipments},
			Syncs: []Sync{{Event: "MOVE", Targets: []Target{
				{Map: "shipments", ID: "s"},
				{Map: "locations", ID: "from", Event: "DEPART"},
				{Map: "locations", ID: "to", Event: "ARRIVE"},
			}}},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			env := stock()
			want := denotation(t, c.p, env)

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
				t.Errorf("not stable-failures equivalent:\ngot  %d states\nwant %d states", len(got.Trans), len(want.Trans))
			}
		})
	}
}

func TestPromoteRefuses(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: location(t)}
	cases := []struct {
		name string
		sync Sync
		want string
	}{
		{"one instance in two roles", Sync{Event: "MOVE", Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}, {Map: "locations", ID: "from", Event: "ARRIVE"}}}, "twice"},
		{"one local event taken twice", Sync{Event: "MOVE", Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}, {Map: "locations", ID: "to", Event: "DEPART"}}}, "DEPART"},
		{"a sync on tau", Sync{Event: Tau, Targets: []Target{{Map: "locations", ID: "loc"}}}, "tau"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			p := Promotion{State: "G", Families: []Family{locations}, Syncs: []Sync{c.sync}}

			// Execute
			_, err := Promote(p)

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one naming %q", err, c.want)
			}
		})
	}
}
