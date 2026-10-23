package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// ValPrelude declares val, the type of every value (docs/CLAUSE_DESIGN.md).
// A map is a ValMap, a list of its entries; an instance is a ValRecord, its
// state the first component.
const ValPrelude = `datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValRecord val "(string \<times> val) list"
  | ValMap "(val \<times> val) list"`

// mapPrelude says what a map is - a list of its entries, of distinct keys,
// the one list of its entries csdf_canon chooses - and what it holds, and
// proves that two maps holding the same entries are one (csdf_map_ext).
const mapPrelude = `(* A map is the list of its entries csdf_canon chooses among the lists of them,
   so that two maps holding the same entries are one. *)
definition csdf_canon :: "(val \<times> val) list \<Rightarrow> (val \<times> val) list"
  where "csdf_canon kvs \<equiv> SOME l. set l = set kvs"

(* A value is well formed when every map in it is canonical, of distinct keys. *)
inductive csdf_wf :: "val \<Rightarrow> bool"
  where "csdf_wf (ValInt i)"
  | "csdf_wf (ValString s)"
  | "csdf_wf (ValBool b)"
  | "\<forall>x\<in>set xs. csdf_wf x \<Longrightarrow> csdf_wf (ValArray xs)"
  | "\<forall>p\<in>set kvs. csdf_wf (snd p) \<Longrightarrow> csdf_wf (ValDict kvs)"
  | "csdf_wf st \<Longrightarrow> \<forall>p\<in>set fs. csdf_wf (snd p) \<Longrightarrow> csdf_wf (ValRecord st fs)"
  | "distinct (map fst kvs) \<Longrightarrow> kvs = csdf_canon kvs \<Longrightarrow> \<forall>p\<in>set kvs. csdf_wf (fst p)
      \<Longrightarrow> \<forall>p\<in>set kvs. csdf_wf (snd p) \<Longrightarrow> csdf_wf (ValMap kvs)"

definition csdf_is_map :: "val \<Rightarrow> bool"
  where "csdf_is_map m \<equiv> (case m of ValMap kvs \<Rightarrow> True | _ \<Rightarrow> False)"

definition csdf_in :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_in k m \<equiv> (case m of ValMap kvs \<Rightarrow> k \<in> set (map fst kvs) | _ \<Rightarrow> False)"

definition csdf_index :: "val \<Rightarrow> val \<Rightarrow> val"
  where "csdf_index m k \<equiv> (case m of ValMap kvs \<Rightarrow> (case map_of kvs k of Some v \<Rightarrow> v | None \<Rightarrow> ValBool False) | _ \<Rightarrow> ValBool False)"

(* Two well-formed maps holding the same entries are one. *)
lemma csdf_map_ext:
  assumes "csdf_wf n" "csdf_wf m" "csdf_is_map n" "csdf_is_map m"
    and h: "\<forall>k. (csdf_in k n \<longleftrightarrow> csdf_in k m) \<and> (csdf_in k n \<longrightarrow> csdf_index n k = csdf_index m k)"
  shows "n = m"
proof -
  obtain a b where ab: "n = ValMap a" "m = ValMap b"
    using assms(3,4) by (cases n; cases m; simp add: csdf_is_map_def)
  have wa: "distinct (map fst a)" "a = csdf_canon a"
    using assms(1) unfolding ab by (auto elim: csdf_wf.cases)
  have wb: "distinct (map fst b)" "b = csdf_canon b"
    using assms(2) unfolding ab by (auto elim: csdf_wf.cases)
  have "map_of a k = map_of b k" for k
  proof (cases "k \<in> set (map fst a)")
    case False
    then have "k \<notin> set (map fst b)"
      using spec[OF h, of k] unfolding ab by (simp add: csdf_in_def)
    then have "map_of b k = None" by (simp add: map_of_eq_None_iff)
    moreover have "map_of a k = None" using False by (simp add: map_of_eq_None_iff)
    ultimately show ?thesis by simp
  next
    case True
    then obtain v where va: "map_of a k = Some v"
      by (metis map_of_eq_None_iff not_None_eq)
    from True have "k \<in> set (map fst b)"
      using spec[OF h, of k] unfolding ab by (simp add: csdf_in_def)
    then obtain w where wb: "map_of b k = Some w"
      by (metis map_of_eq_None_iff not_None_eq)
    have "csdf_index n k = csdf_index m k"
      using spec[OF h, of k] True unfolding ab by (simp add: csdf_in_def)
    then have "v = w" using va wb unfolding ab by (simp add: csdf_index_def)
    then show ?thesis using va wb by simp
  qed
  then have "map_of a = map_of b" by (rule ext)
  then have "set a = set b" using wa(1) wb(1) by (simp add: map_of_inject_set)
  then have "csdf_canon a = csdf_canon b" by (simp add: csdf_canon_def)
  then show ?thesis using ab wa(2) wb(2) by simp
qed

`

// atDef defines that an instance, a record, is at a state: its state is it.
// A value that is no record is at no state.
const atDef = `definition csdf_at :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_at t s \<equiv> (case t of ValRecord st fs \<Rightarrow> st = s | _ \<Rightarrow> False)"

`

// WriteBuiltins defines the builtins b reads: a builtin is a definition,
// not an assumption, so no obligation rests on an axiom of it. What a map
// is comes first in every obligation, for a quantifier over a type is
// restricted to well-formed values. A state constant is a number of its
// own, so two are apart.
func WriteBuiltins(w io.Writer, b obligationir.Builtins) error {
	io.WriteString(w, mapPrelude)
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
		fmt.Fprintf(w, "lemma %s: \"%s\"\n  by (auto simp: %s split: val.splits)\n\n", obligationir.LemmaName(i+1), f, strings.Join(unfold, " "))
	}
	return nil
}
