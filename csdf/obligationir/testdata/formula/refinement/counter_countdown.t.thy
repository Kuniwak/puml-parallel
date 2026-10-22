theory Refinement_Obligation
  imports CSP_T.CSP_T
begin

datatype scalar = SInt int
  | SString string
  | SBool bool
  | SArray "scalar list"
  | SDict "(string \<times> scalar) list"
  | SRecord scalar "(string \<times> scalar) list"

datatype val = ValScalar scalar
  | ValMap "scalar \<Rightarrow> scalar option"

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
(* $_0 = 10 *)
consts atom__u24u__0_u20u_u3du_u20u10 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 = $_1 + $_2 *)
consts atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 :: "val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 = $_1 - 1 *)
consts atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 :: "val \<Rightarrow> val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 > 0 *)
consts atom__u24u__0_u20u_u3eu_u20u0 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* k *)
consts type_k :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n *)
consts type_n :: "val \<Rightarrow> bool"

datatype PN = S_s val (* s (n) *)
  | I_a val (* a (n) *)

primrec
  procfun :: "(PN, event) pnfun"
where
  "procfun (S_s v_n) =
     (? ev:{ev. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} .. $(S_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet (Ev_show v_n) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (v_n_u27u = v_n))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet (Ev_show v_n) \<and> (type_n v_n_u27u \<and> (v_n_u27u = v_n))} .. $(S_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> (v_n_u27u = v_m)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet HTau \<and> (type_n v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> (v_n_u27u = v_m)))} .. $(S_s v_n_u27u)))"
| "procfun (I_a v_n) =
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet HTau \<and> (type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))} .. $(I_a v_n_u27u)))"

overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]

definition SpecProc :: "(PN, event) proc"
  where "SpecProc \<equiv> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)} .. $(S_s v_n_u27u)) -- {Alphabet HTau}"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)} .. $(I_a v_n_u27u)) -- {Alphabet HTau}"

(* Spec admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under no hypothesis: it must not rest on an assumption. *)
theorem initialisable_S: "\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)"
  oops

(* Impl admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under no hypothesis: it must not rest on an assumption. *)
theorem initialisable_I: "\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)"
  oops

(* Every trace of the Impl diagram is a trace of the Spec diagram: in CSP-Prover
   P <=T Q unfolds to traces Q <= traces P. *)
theorem refines_t: "SpecProc <=T ImplProc"
  oops

end
