theory Livelock_Obligation
  imports Main
begin

datatype scalar = SInt int
  | SString string
  | SBool bool
  | SArray "scalar list"
  | SDict "(string \<times> scalar) list"
  | SRecord scalar "(string \<times> scalar) list"

datatype val = ValScalar scalar
  | ValMap "scalar \<Rightarrow> scalar option"

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

definition csdf_is_map :: "val \<Rightarrow> bool"
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
  assumes "assume_fair"
  shows "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
