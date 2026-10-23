package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// mapPrelude defines what a map holds, and proves that two maps holding
// the same entries are one.
const mapPrelude = `definition csdf_is_map :: "val \<Rightarrow> bool"
  where "csdf_is_map m \<equiv> (case m of ValMap f \<Rightarrow> True | _ \<Rightarrow> False)"

definition csdf_in :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_in k m \<equiv> (case (k, m) of (ValScalar s, ValMap f) \<Rightarrow> f s \<noteq> None | _ \<Rightarrow> False)"

definition csdf_index :: "val \<Rightarrow> val \<Rightarrow> val"
  where "csdf_index m k \<equiv> (case (m, k) of (ValMap f, ValScalar s) \<Rightarrow> (case f s of Some v \<Rightarrow> ValScalar v | None \<Rightarrow> ValScalar (SBool False)) | _ \<Rightarrow> ValScalar (SBool False))"

lemma csdf_map_ext:
  assumes "csdf_is_map n" "csdf_is_map m"
    and "\<forall>k. (csdf_in k n \<longleftrightarrow> csdf_in k m) \<and> (csdf_in k n \<longrightarrow> csdf_index n k = csdf_index m k)"
  shows "n = m"
proof -
  obtain f g where nm: "n = ValMap f" "m = ValMap g"
    using assms(1,2) by (cases n; cases m; simp add: csdf_is_map_def)
  have "f s = g s" for s
    using assms(3)[rule_format, of "ValScalar s"] nm
    by (cases "f s"; cases "g s"; simp add: csdf_in_def csdf_index_def)
  then show ?thesis using nm by (auto intro: ext)
qed

`

const atDef = `definition csdf_state :: "val \<Rightarrow> val"
  where "csdf_state t \<equiv> (case t of ValScalar (SRecord s fs) \<Rightarrow> ValScalar s | _ \<Rightarrow> ValScalar (SBool False))"

definition csdf_at :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_at t s \<equiv> csdf_state t = s"

`

// WriteBuiltins defines the builtins b reads: a builtin is a definition,
// not an assumption, so no obligation rests on an axiom of it. A state
// constant is a number of its own, so two are apart.
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
		fmt.Fprintf(w, "definition %s :: val\n  where \"%s \\<equiv> ValScalar (SInt %d)\"\n\n", st, st, i)
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
