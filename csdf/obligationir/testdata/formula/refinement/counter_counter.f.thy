theory Refinement_Obligation
  imports CSP_F.CSP_F
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"

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

datatype PN = S_s val (* s (n) *)
  | I_s val (* s (n) *)

primrec
  procfun :: "(PN, event) pnfun"
where
  "procfun (S_s v_n) =
     (? ev:{ev. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} .. $(S_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet (Ev_show v_n) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (v_n_u27u = v_n))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet (Ev_show v_n) \<and> (type_n v_n_u27u \<and> (v_n_u27u = v_n))} .. $(S_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> (v_n_u27u = v_m)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet HTau \<and> (type_n v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> (v_n_u27u = v_m)))} .. $(S_s v_n_u27u)))"
| "procfun (I_s v_n) =
     (? ev:{ev. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. \<exists>(v_k::val). type_k v_k \<and> (ev = Alphabet (Ev_inc v_k) \<and> (type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_k \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))} .. $(I_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet (Ev_show v_n) \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (v_n_u27u = v_n))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet (Ev_show v_n) \<and> (type_n v_n_u27u \<and> (v_n_u27u = v_n))} .. $(I_s v_n_u27u)))
     [+]
     (? ev:{ev. ev = Alphabet HTau \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> (v_n_u27u = v_m)))} -> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. ev = Alphabet HTau \<and> (type_n v_n_u27u \<and> (\<exists>(v_m::val). type_n v_m \<and> (v_n_u27u = v_m)))} .. $(I_s v_n_u27u)))"

overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]

definition SpecProc :: "(PN, event) proc"
  where "SpecProc \<equiv> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)} .. $(S_s v_n_u27u)) -- {Alphabet HTau}"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> (!<(\<lambda>v_n_u27u. Internal [v_n_u27u])> v_n_u27u:{v_n_u27u. type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)} .. $(I_s v_n_u27u)) -- {Alphabet HTau}"

(* Spec admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under the axioms alone: it must not rest on an assumption. *)
theorem initialisable_S: "\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)"
  oops

(* Impl admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under the axioms alone: it must not rest on an assumption. *)
theorem initialisable_I: "\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)"
  oops

(* Every stable failure of the Impl diagram is one of the Spec diagram: in
   CSP-Prover P <=F Q unfolds to failures Q <= failures P, and <=F subsumes
   trace inclusion. *)
theorem refines_f: "SpecProc <=F ImplProc"
  oops

end
