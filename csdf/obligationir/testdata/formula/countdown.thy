theory Livelock_Obligation
  imports Main
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"

datatype st = St_a val (* a (n) *)

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
(* n *)
consts type_n :: "val \<Rightarrow> bool"

definition init :: "st \<Rightarrow> bool"
  where "init s' \<equiv> \<exists>(v_n_u27u::val). s' = St_a v_n_u27u \<and> type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> (\<exists>(v_n::val) (v_n_u27u::val). s = St_a v_n \<and> s' = St_a v_n_u27u \<and> type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init s \<Longrightarrow> reachable s"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> (\<exists>(v_n::val) (v_n_u27u::val). s = St_a v_n \<and> s' = St_a v_n_u27u \<and> type_n v_n_u27u \<and> (atom__u24u__0_u20u_u3eu_u20u0 v_n \<and> atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))"

theorem livelock_free: "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
