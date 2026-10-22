theory Refinement_Obligation
  imports CSP_F.CSP_F
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValMap "(val \<times> val) list"

datatype alphabet = Ev_PUT val
  | HTau

(* Internal only indexes the replicated internal choices below, via
   Rep_int_choice_f; no process performs it, so it changes no trace and no
   refusal. The injections into it are of the form \<lambda>(x, y). Internal [x, y],
   whose inj side condition is discharged by (simp add: inj_def). *)
datatype event = Alphabet alphabet
  | Internal "val list"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 は古い *)
consts atom__u24u__0_u20u_u306fu_u53e4u_u3044u :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 = 10 *)
consts atom__u24u__0_u20u_u3du_u20u10 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 = $_1 - 1 *)
consts atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 :: "val \<Rightarrow> val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 > 0 *)
consts atom__u24u__0_u20u_u3eu_u20u0 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* 鍵 *)
consts type__u9375u :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* k *)
consts type_k :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* m *)
consts type_m :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n *)
consts type_n :: "val \<Rightarrow> bool"

(* A map is a ValMap; the entry of a key is the first one holding it. *)
fun csdf_lookup :: "(val \<times> val) list \<Rightarrow> val \<Rightarrow> val"
  where "csdf_lookup [] k = ValBool False"
  | "csdf_lookup ((j, v) # kvs) k = (if j = k then v else csdf_lookup kvs k)"

definition csdf_is_map :: "val \<Rightarrow> bool"
  where "csdf_is_map m \<equiv> (case m of ValMap kvs \<Rightarrow> True | _ \<Rightarrow> False)"

definition csdf_in :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_in k m \<equiv> (case m of ValMap kvs \<Rightarrow> k \<in> set (map fst kvs) | _ \<Rightarrow> False)"

definition csdf_index :: "val \<Rightarrow> val \<Rightarrow> val"
  where "csdf_index m k \<equiv> (case m of ValMap kvs \<Rightarrow> csdf_lookup kvs k | _ \<Rightarrow> ValBool False)"

definition csdf_empty :: "val \<Rightarrow> bool"
  where "csdf_empty v_m \<equiv> csdf_is_map v_m \<and> (\<forall>(v_k::val). \<not> csdf_in v_k v_m)"

definition csdf_frame1 :: "val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_frame1 v_n v_m v_k1 \<equiv> csdf_is_map v_n \<and> csdf_is_map v_m \<and> (\<forall>(v_j::val). (\<not> (v_j = v_k1)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"

lemma csdf_lemma_1: "\<forall>(v_m::val) (v_k::val). csdf_empty v_m \<longrightarrow> (\<not> csdf_in v_k v_m)"
  by (auto simp: csdf_empty_def csdf_frame1_def)

lemma csdf_lemma_2: "\<forall>(v_n::val) (v_m::val) (v_k1::val). csdf_frame1 v_n v_m v_k1 \<longrightarrow> (\<forall>(v_j::val). (\<not> (v_j = v_k1)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"
  by (auto simp: csdf_empty_def csdf_frame1_def)

(* TODO(csdf): not formalised; this assumption is uninterpreted. *)
(* PUT は無限に起きない *)
consts assume_fair_S :: "bool"

datatype PN = S_s val (* s (m) *)
  | I_a val (* a (n) *)

primrec
  procfun :: "(PN, event) pnfun"
where
  "procfun (S_s v_m) =
     (? ev:{ev. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_PUT v_k) \<and> (\<exists>(v_m_u27u::val). type_m v_m_u27u \<and> ((\<not> csdf_in v_k v_m) \<and> csdf_frame1 v_m_u27u v_m v_k \<and> csdf_in v_k v_m_u27u)))} -> (!<(\<lambda>v_m_u27u. Internal [v_m_u27u])> v_m_u27u:{v_m_u27u. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_PUT v_k) \<and> (type_m v_m_u27u \<and> ((\<not> csdf_in v_k v_m) \<and> csdf_frame1 v_m_u27u v_m v_k \<and> csdf_in v_k v_m_u27u)))} .. $(S_s v_m_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_m_u27u::val). type_m v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))} -> (!<(\<lambda>v_m_u27u. Internal [v_m_u27u])> v_m_u27u:{v_m_u27u. ev = Alphabet HTau \<and> (type_m v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))} .. $(S_s v_m_u27u)))"
| "procfun (I_a v_n) =
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet HTau \<and> (type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))} .. $(I_a v_n_u27u)))"

overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]

definition SpecProc :: "(PN, event) proc"
  where "SpecProc \<equiv> (!<(\<lambda>v_m_u27u. Internal [v_m_u27u])> v_m_u27u:{v_m_u27u. type_m v_m_u27u \<and> (csdf_empty v_m_u27u)} .. $(S_s v_m_u27u)) -- {Alphabet HTau}"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)} .. $(I_a v_n_u27u)) -- {Alphabet HTau}"

datatype st_S = St_S_s val (* s (m) *)

definition init_S :: "st_S \<Rightarrow> bool"
  where "init_S s' \<equiv> \<exists>(v_m_u27u::val). s' = St_S_s v_m_u27u \<and> type_m v_m_u27u \<and> (csdf_empty v_m_u27u)"

definition step_S :: "st_S \<Rightarrow> st_S \<Rightarrow> bool"
  where "step_S s s' \<equiv> (\<exists>(v_m::val) (v_m_u27u::val). s = St_S_s v_m \<and> s' = St_S_s v_m_u27u \<and> type_m v_m_u27u \<and> (\<exists>(v_k::val). type_k v_k \<and> ((\<not> csdf_in v_k v_m) \<and> csdf_frame1 v_m_u27u v_m v_k \<and> csdf_in v_k v_m_u27u))) \<or> (\<exists>(v_m::val) (v_m_u27u::val). s = St_S_s v_m \<and> s' = St_S_s v_m_u27u \<and> type_m v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))"

inductive reachable_S :: "st_S \<Rightarrow> bool"
  where base: "init_S s \<Longrightarrow> reachable_S s"
  | step: "reachable_S s \<Longrightarrow> step_S s s' \<Longrightarrow> reachable_S s'"

definition tau_step_S :: "st_S \<Rightarrow> st_S \<Rightarrow> bool"
  where "tau_step_S s s' \<equiv> (\<exists>(v_m::val) (v_m_u27u::val). s = St_S_s v_m \<and> s' = St_S_s v_m_u27u \<and> type_m v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))"

theorem livelock_free_S:
  assumes "assume_fair_S"
  shows "wf_on {s. reachable_S s} {(s', s). tau_step_S s s'}"
  oops

(* Spec admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under no hypothesis: it must not rest on an assumption. *)
theorem initialisable_S: "\<exists>(v_m_u27u::val). type_m v_m_u27u \<and> (csdf_empty v_m_u27u)"
  oops

datatype st_I = St_I_a val (* a (n) *)

definition init_I :: "st_I \<Rightarrow> bool"
  where "init_I s' \<equiv> \<exists>(v_n_u27u::val). s' = St_I_a v_n_u27u \<and> type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)"

definition step_I :: "st_I \<Rightarrow> st_I \<Rightarrow> bool"
  where "step_I s s' \<equiv> (\<exists>(v_n::val) (v_n_u27u::val). s = St_I_a v_n \<and> s' = St_I_a v_n_u27u \<and> type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))"

inductive reachable_I :: "st_I \<Rightarrow> bool"
  where base: "init_I s \<Longrightarrow> reachable_I s"
  | step: "reachable_I s \<Longrightarrow> step_I s s' \<Longrightarrow> reachable_I s'"

definition tau_step_I :: "st_I \<Rightarrow> st_I \<Rightarrow> bool"
  where "tau_step_I s s' \<equiv> (\<exists>(v_n::val) (v_n_u27u::val). s = St_I_a v_n \<and> s' = St_I_a v_n_u27u \<and> type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))"

theorem livelock_free_I: "wf_on {s. reachable_I s} {(s', s). tau_step_I s s'}"
  oops

(* Impl admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under no hypothesis: it must not rest on an assumption. *)
theorem initialisable_I: "\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)"
  oops

(* CSP-Prover has no FD model, so this is the standard reduction: for
   divergence-free processes, FD refinement coincides with <=F. Divergence of
   SpecProc/ImplProc arises from an infinite HTau run of the underlying diagram
   or from a side that cannot start at all, which the well-foundedness and
   initialisability obligations above rule out respectively. *)
(* Every stable failure of the Impl diagram is one of the Spec diagram: in
   CSP-Prover P <=F Q unfolds to failures Q <= failures P, and <=F subsumes
   trace inclusion. *)
theorem refines_f: "SpecProc <=F ImplProc"
  oops

end
