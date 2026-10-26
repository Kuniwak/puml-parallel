theory Refinement_Obligation
  imports CSP_F.CSP_F "HOL-Library.Countable"
begin

datatype val = ValInt int
  | ValFloat bool nat int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValRecord val "(string \<times> val) list"
  | ValMap "(val \<times> val) list"

datatype alphabet = Ev_inc val
  | Ev_show val
  | HTau

(* Internal only indexes the replicated internal choices below, via
   Rep_int_choice_f; no process performs it, so it changes no trace and no
   refusal. The injections into it are of the form \<lambda>(x, y). Internal [x, y],
   whose inj side condition is discharged by (simp add: inj_def). *)
datatype event = Alphabet alphabet
  | Internal "val list"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 = 0 *)
consts atom__u24u__0_u20u_u3du_u20u0 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 = $_1 + $_2 *)
consts atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 :: "val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 > 0 *)
consts atom__u24u__0_u20u_u3eu_u20u0 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* k *)
consts type_k :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n *)
consts type_n :: "val \<Rightarrow> bool"

(* Values are countable, so ordered by their numbers: a linear order of the
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

datatype PN = S_s val (* s (n) *)
  | I_s val (* s (n) *)

primrec
  procfun :: "(PN, event) pnfun"
where
  "procfun (S_s v_n) =
     (? ev:{ev. \<exists>(v_k::val). type_k v_k \<and> csdf_wf v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. \<exists>(v_k::val). type_k v_k \<and> csdf_wf v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} .. $(S_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet (Ev_show v_n) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (v_n_u27u = v_n))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet (Ev_show v_n) \<and> (type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (v_n_u27u = v_n))} .. $(S_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> csdf_wf v_m \<and> (v_n_u27u = v_m)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet HTau \<and> (type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> csdf_wf v_m \<and> (v_n_u27u = v_m)))} .. $(S_s v_n_u27u)))"
| "procfun (I_s v_n) =
     (? ev:{ev. \<exists>(v_k::val). type_k v_k \<and> csdf_wf v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. \<exists>(v_k::val). type_k v_k \<and> csdf_wf v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} .. $(I_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet (Ev_show v_n) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (v_n_u27u = v_n))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet (Ev_show v_n) \<and> (type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (v_n_u27u = v_n))} .. $(I_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> csdf_wf v_m \<and> (v_n_u27u = v_m)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet HTau \<and> (type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> csdf_wf v_m \<and> (v_n_u27u = v_m)))} .. $(I_s v_n_u27u)))"

overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]

definition SpecProc :: "(PN, event) proc"
  where "SpecProc \<equiv> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)} .. $(S_s v_n_u27u)) -- {Alphabet HTau}"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)} .. $(I_s v_n_u27u)) -- {Alphabet HTau}"

(* Spec admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under no hypothesis: it must not rest on an assumption. *)
theorem initialisable_S: "\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)"
  oops

(* Impl admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under no hypothesis: it must not rest on an assumption. *)
theorem initialisable_I: "\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)"
  oops

(* Every stable failure of the Impl diagram is one of the Spec diagram: in
   CSP-Prover P <=F Q unfolds to failures Q <= failures P, and <=F subsumes
   trace inclusion. *)
theorem refines_f: "SpecProc <=F ImplProc"
  oops

end
