package clause

// Keeps is the variable b says is unchanged by the step, if it says so:
// @eq(v', v), @eq(v, v') or @frame(v', v).
func Keeps(b Builtin) (string, bool) {
	switch {
	case b.Op == OpEq, b.Op == OpFrame && len(b.Args) == 2:
	default:
		return "", false
	}
	x, okX := b.Args[0].(Free)
	y, okY := b.Args[1].(Free)
	if !okX || !okY || x.Name != y.Name || x.Primed == y.Primed {
		return "", false
	}
	if b.Op == OpFrame && !x.Primed {
		return "", false
	}
	return x.Name, true
}
