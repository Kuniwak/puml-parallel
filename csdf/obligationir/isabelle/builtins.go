package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// mapPrelude defines a map: a ValMap, the entry of a key the first entry of
// the list holding it.
const mapPrelude = `(* A map is a ValMap; the entry of a key is the first one holding it. *)
fun csdf_lookup :: "(val \<times> val) list \<Rightarrow> val \<Rightarrow> val"
  where "csdf_lookup [] k = ValBool False"
  | "csdf_lookup ((j, v) # kvs) k = (if j = k then v else csdf_lookup kvs k)"

definition csdf_is_map :: "val \<Rightarrow> bool"
  where "csdf_is_map m \<equiv> (case m of ValMap kvs \<Rightarrow> True | _ \<Rightarrow> False)"

definition csdf_in :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_in k m \<equiv> (case m of ValMap kvs \<Rightarrow> k \<in> set (map fst kvs) | _ \<Rightarrow> False)"

definition csdf_index :: "val \<Rightarrow> val \<Rightarrow> val"
  where "csdf_index m k \<equiv> (case m of ValMap kvs \<Rightarrow> csdf_lookup kvs k | _ \<Rightarrow> ValBool False)"

`

const atDef = `(* TODO(csdf): not formalised; the state an instance is at is uninterpreted. *)
consts csdf_state :: "val \<Rightarrow> val"

definition csdf_at :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_at t s \<equiv> csdf_state t = s"

`

// WriteBuiltins defines the builtins b reads over the maps of ValMap: a
// builtin is a definition, not an assumption, so no obligation rests on an
// axiom of it. A state constant is a number of its own, so two are apart.
func WriteBuiltins(w io.Writer, b obligationir.Builtins) error {
	if b.Maps {
		io.WriteString(w, mapPrelude)
	}
	defs, err := obligationir.BuiltinDefs(Notation, b)
	if err != nil {
		return fmt.Errorf("isabelle.WriteBuiltins: %w", err)
	}
	for _, d := range defs {
		fmt.Fprintf(w, "definition %s :: \"%sbool\"\n  where \"%s %s \\<equiv> %s\"\n\n",
			d.Name, strings.Repeat(`val \<Rightarrow> `, len(d.Params)), d.Name, strings.Join(d.Params, " "), d.Body)
	}
	if b.At {
		io.WriteString(w, atDef)
	}
	for i, st := range b.States {
		fmt.Fprintf(w, "definition %s :: val\n  where \"%s \\<equiv> ValInt %d\"\n\n", st, st, i)
	}
	return nil
}

// WriteLemmas proves what the builtins mean from their definitions, so that
// the prover checks it rather than an obligation assuming it.
func WriteLemmas(w io.Writer, b obligationir.Builtins, lemmas []logic.Tree) error {
	defs, err := obligationir.BuiltinDefs(Notation, b)
	if err != nil {
		return fmt.Errorf("isabelle.WriteLemmas: %w", err)
	}
	var unfold []string
	for _, d := range defs {
		unfold = append(unfold, d.Name+"_def")
	}
	if b.At {
		unfold = append(unfold, "csdf_at_def")
	}
	for _, st := range b.States {
		unfold = append(unfold, st+"_def")
	}
	for i, l := range lemmas {
		f, err := Notation.SpellFormula(l)
		if err != nil {
			return fmt.Errorf("isabelle.WriteLemmas: %w", err)
		}
		fmt.Fprintf(w, "lemma %s: \"%s\"\n  by (auto simp: %s)\n\n", obligationir.LemmaName(i+1), f, strings.Join(unfold, " "))
	}
	return nil
}
