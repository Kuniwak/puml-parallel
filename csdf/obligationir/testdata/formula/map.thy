theory Livelock_Obligation
  imports Main
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"

datatype st = St_s val (* s (m) *)

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* $_0 は古い *)
consts atom__u24u__0_u20u_u306fu_u53e4u_u3044u :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* 鍵 *)
consts type__u9375u :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* k *)
consts type_k :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* m *)
consts type_m :: "val \<Rightarrow> bool"

consts csdf_index :: "val \<Rightarrow> val \<Rightarrow> val"

consts csdf_empty :: "val \<Rightarrow> bool"

consts csdf_frame1 :: "val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> bool"

consts csdf_in :: "val \<Rightarrow> val \<Rightarrow> bool"

definition csdf_axiom_1 :: bool
  where "csdf_axiom_1 \<equiv> \<forall>(v_m::val) (v_k::val). csdf_empty v_m \<longrightarrow> (\<not> csdf_in v_k v_m)"

definition csdf_axiom_2 :: bool
  where "csdf_axiom_2 \<equiv> \<forall>(v_n::val) (v_m::val) (v_k1::val). csdf_frame1 v_n v_m v_k1 \<longrightarrow> (\<forall>(v_j::val). (\<not> (v_j = v_k1)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"

(* TODO(csdf): not formalised; this assumption is uninterpreted. *)
(* PUT は無限に起きない *)
consts assume_fair :: "bool"

definition init :: "st \<Rightarrow> bool"
  where "init s' \<equiv> \<exists>(v_m_u27u::val). s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> (csdf_empty v_m_u27u)"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> (\<exists>(v_m::val) (v_m_u27u::val). s = St_s v_m \<and> s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> (\<exists>(v_k::val). type_k v_k \<and> ((\<not> csdf_in v_k v_m) \<and> csdf_frame1 v_m_u27u v_m v_k \<and> csdf_in v_k v_m_u27u))) \<or> (\<exists>(v_m::val) (v_m_u27u::val). s = St_s v_m \<and> s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init s \<Longrightarrow> reachable s"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> (\<exists>(v_m::val) (v_m_u27u::val). s = St_s v_m \<and> s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))"

theorem livelock_free:
  assumes "csdf_axiom_1" and "csdf_axiom_2" and "assume_fair"
  shows "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
