package obligationir

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

// Each builtin's arguments, by position, as a map (m), a key (k) or an
// entry (v): what the model of the back ends holds there (CLAUSE_DESIGN.md).
// A map's keys and entries are scalars, so a map holds no map.
var positions = map[string]string{
	obligation.PredIn:     "km",
	obligation.PredEmpty:  "m",
	obligation.PredUpdate: "mmkv",
	obligation.PredFrame:  "mm", // then a key for each key
}

// checkMaps refuses what reads a map where the model holds a scalar: a map
// as the key or the entry of a map, and a map that is no variable - the
// entry of a map, or a field of an instance - which the model has no map
// for, so a builtin over it would hold of nothing and an obligation over it
// hold vacuously.
func checkMaps(fs []logic.Tree) error {
	maps := map[string]bool{}
	var scalars []logic.TermTree
	var problem error
	mapAt := func(t logic.TermTree) {
		if t.Op != logic.TermVar {
			if problem == nil {
				problem = fmt.Errorf("a map is read in %s, which is no variable but an entry or a field, and a map holds no map", spellTree(t))
			}
			return
		}
		maps[strings.TrimSuffix(t.Name, "'")] = true
	}
	var term func(t logic.TermTree)
	term = func(t logic.TermTree) {
		if t.Op == logic.TermIndex {
			mapAt(t.Args[0])
			scalars = append(scalars, t.Args[1])
		}
		for _, a := range t.Args {
			term(a)
		}
	}
	var formula func(t logic.Tree)
	formula = func(t logic.Tree) {
		if t.Op == logic.OpAtom && !t.Quoted {
			name := t.Name
			pos, ok := positions[name]
			if ok {
				for i, a := range t.Terms {
					p := byte('k')
					if i < len(pos) {
						p = pos[i]
					}
					switch p {
					case 'm':
						mapAt(a)
					default:
						scalars = append(scalars, a)
					}
				}
			}
		}
		for _, a := range t.Terms {
			term(a)
		}
		for _, o := range t.Operands {
			formula(o)
		}
	}
	for _, f := range fs {
		formula(f)
	}
	if problem != nil {
		return problem
	}
	for _, s := range scalars {
		if s.Op == logic.TermVar && maps[strings.TrimSuffix(s.Name, "'")] {
			return fmt.Errorf("the map %s is read as the key or the entry of a map, and a map holds no map", s.Name)
		}
	}
	return nil
}

func spellTree(t logic.TermTree) string {
	switch t.Op {
	case logic.TermVar:
		return t.Name
	case logic.TermIndex:
		return spellTree(t.Args[0]) + "[" + spellTree(t.Args[1]) + "]"
	case logic.TermField:
		return spellTree(t.Args[0]) + "." + t.Name
	}
	parts := make([]string, len(t.Args))
	for i, a := range t.Args {
		parts[i] = spellTree(a)
	}
	return t.Name + "(" + strings.Join(parts, ", ") + ")"
}
