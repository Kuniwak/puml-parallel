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
// state the first component. A float is ValFloat neg sig exp, the value
// (-1)^neg · sig · 2^exp, pure data with no arithmetic of IEEE 754: in its
// canonical form, sig odd or the zero ValFloat False 0 0, so two floats are
// one exactly when they are equal (clause.FloatParts).
const ValPrelude = `datatype val = ValInt int
  | ValFloat bool nat int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValRecord val "(string \<times> val) list"
  | ValMap "(val \<times> val) list"`

// LibraryImports are the theories of the library the model of the builtins
// rests on: Countable, to order every value. A session building an
// obligation declares LibrarySession, as Root does.
const LibraryImports = `"HOL-Library.Countable"`

// LibrarySession is the session holding LibraryImports.
const LibrarySession = "HOL-Library"

// Root is the ROOT of a session building theory on parent: it declares
// LibrarySession, without which the theory cannot import LibraryImports.
func Root(session, parent, theory string) string {
	var w strings.Builder
	w.WriteString("session " + session + " = " + parent + " +\n  sessions\n")
	w.WriteString("    \"" + LibrarySession + "\"\n")
	w.WriteString("  theories\n    " + theory + "\n")
	return w.String()
}

// mapPrelude says what a map is - a list of its entries, of distinct keys
// sorted in a linear order of every value - and what it holds, and proves
// that two maps holding the same entries are one (csdf_map_ext).
const mapPrelude = `(* Values are countable, so ordered by their numbers: a linear order of the
   library's, which the keys of a map are sorted in. *)
instance val :: countable by countable_datatype

instantiation val :: linorder
begin
definition less_eq_val :: "val \<Rightarrow> val \<Rightarrow> bool" where "less_eq_val x y \<equiv> to_nat x \<le> to_nat y"
definition less_val :: "val \<Rightarrow> val \<Rightarrow> bool" where "less_val x y \<equiv> to_nat x < to_nat y"
instance by standard (auto simp: less_eq_val_def less_val_def)
end

(* A value is well formed when every map in it has distinct keys, sorted,
   and every float in it is in its canonical form. *)
inductive csdf_wf :: "val \<Rightarrow> bool"
  where "csdf_wf (ValInt i)"
  | "odd m \<or> (\<not> s \<and> m = 0 \<and> e = 0) \<Longrightarrow> csdf_wf (ValFloat s m e)"
  | "csdf_wf (ValString s)"
  | "csdf_wf (ValBool b)"
  | "\<forall>x\<in>set xs. csdf_wf x \<Longrightarrow> csdf_wf (ValArray xs)"
  | "\<forall>p\<in>set kvs. csdf_wf (snd p) \<Longrightarrow> csdf_wf (ValDict kvs)"
  | "csdf_wf st \<Longrightarrow> \<forall>p\<in>set fs. csdf_wf (snd p) \<Longrightarrow> csdf_wf (ValRecord st fs)"
  | "distinct (map fst kvs) \<Longrightarrow> sorted (map fst kvs) \<Longrightarrow> \<forall>p\<in>set kvs. csdf_wf (fst p)
      \<Longrightarrow> \<forall>p\<in>set kvs. csdf_wf (snd p) \<Longrightarrow> csdf_wf (ValMap kvs)"

inductive_cases csdf_wf_map: "csdf_wf (ValMap kvs)"

definition csdf_is_map :: "val \<Rightarrow> bool"
  where "csdf_is_map m \<equiv> (case m of ValMap kvs \<Rightarrow> True | _ \<Rightarrow> False)"

definition csdf_in :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_in k m \<equiv> (case m of ValMap kvs \<Rightarrow> k \<in> set (map fst kvs) | _ \<Rightarrow> False)"

definition csdf_index :: "val \<Rightarrow> val \<Rightarrow> val"
  where "csdf_index m k \<equiv> (case m of ValMap kvs \<Rightarrow> (case map_of kvs k of Some v \<Rightarrow> v | None \<Rightarrow> ValBool False) | _ \<Rightarrow> ValBool False)"

(* A list of entries of distinct keys is its keys, each with its entry. *)
lemma csdf_entries:
  assumes "distinct (map fst kvs)"
  shows "kvs = zip (map fst kvs) (map (\<lambda>k. the (map_of kvs k)) (map fst kvs))"
proof -
  have "map snd kvs = map (\<lambda>k. the (map_of kvs k)) (map fst kvs)"
    using assms by (auto simp: map_of_is_SomeI)
  then show ?thesis by (metis zip_map_fst_snd)
qed

(* Two well-formed maps holding the same entries are one. *)
lemma csdf_map_ext:
  assumes "csdf_wf n" "csdf_wf m" "csdf_is_map n" "csdf_is_map m"
    and h: "\<forall>k. (csdf_in k n \<longleftrightarrow> csdf_in k m) \<and> (csdf_in k n \<longrightarrow> csdf_index n k = csdf_index m k)"
  shows "n = m"
proof -
  obtain a b where ab: "n = ValMap a" "m = ValMap b"
    using assms(3,4) by (cases n; cases m; simp add: csdf_is_map_def)
  have wa: "distinct (map fst a)" "sorted (map fst a)"
    using assms(1) unfolding ab by (auto elim: csdf_wf_map)
  have wb: "distinct (map fst b)" "sorted (map fst b)"
    using assms(2) unfolding ab by (auto elim: csdf_wf_map)
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
      by (cases "map_of a k") (simp_all add: map_of_eq_None_iff)
    from True have "k \<in> set (map fst b)"
      using spec[OF h, of k] unfolding ab by (simp add: csdf_in_def)
    then obtain w where wb': "map_of b k = Some w"
      by (cases "map_of b k") (simp_all add: map_of_eq_None_iff)
    have "csdf_index n k = csdf_index m k"
      using spec[OF h, of k] True unfolding ab by (simp add: csdf_in_def)
    then show ?thesis using va wb' unfolding ab by (simp add: csdf_index_def)
  qed
  then have "map_of a = map_of b" by (rule ext)
  moreover have "set a = set b" using \<open>map_of a = map_of b\<close> wa(1) wb(1) by (simp add: map_of_inject_set)
  moreover have "map fst a = map fst b"
    using wa wb \<open>set a = set b\<close> by (metis set_map sorted_distinct_set_unique)
  ultimately have "a = b"
    using csdf_entries[OF wa(1)] csdf_entries[OF wb(1)] \<open>map_of a = map_of b\<close> by metis
  then show ?thesis using ab by simp
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
