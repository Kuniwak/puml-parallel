theory Livelock_Obligation
  imports Main
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValMap "(val \<times> val) list"

datatype st = St_s val val (* s (t, m) *)

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* 鍵 *)
consts type__u9375u :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* m *)
consts type_m :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* t *)
consts type_t :: "val \<Rightarrow> bool"

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

definition csdf_update :: "val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_update v_n v_m v_k v_v \<equiv> csdf_is_map v_n \<and> csdf_is_map v_m \<and> (csdf_in v_k v_n \<and> csdf_index v_n v_k = v_v \<and> (\<forall>(v_j::val). (\<not> (v_j = v_k)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j)))))"

definition csdf_frame0 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_frame0 v_n v_m \<equiv> csdf_is_map v_n \<and> csdf_is_map v_m \<and> (\<forall>(v_j::val). True \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"

(* TODO(csdf): not formalised; the state an instance is at is uninterpreted. *)
consts csdf_state :: "val \<Rightarrow> val"

definition csdf_at :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_at t s \<equiv> csdf_state t = s"

definition state__u2euA :: val
  where "state__u2euA \<equiv> ValInt 0"

definition state__u2euB :: val
  where "state__u2euB \<equiv> ValInt 1"

lemma csdf_lemma_1: "\<forall>(v_m::val) (v_k::val). csdf_empty v_m \<longrightarrow> (\<not> csdf_in v_k v_m)"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def)

lemma csdf_lemma_2: "\<forall>(v_n::val) (v_m::val) (v_k::val) (v_v::val). csdf_update v_n v_m v_k v_v \<longrightarrow> (csdf_in v_k v_n \<and> csdf_index v_n v_k = v_v \<and> (\<forall>(v_j::val). (\<not> (v_j = v_k)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j)))))"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def)

lemma csdf_lemma_3: "\<forall>(v_n::val) (v_m::val). csdf_frame0 v_n v_m \<longrightarrow> (\<forall>(v_j::val). True \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def)

lemma csdf_lemma_4: "\<forall>(v_t::val). \<not> (csdf_at v_t state__u2euA \<and> csdf_at v_t state__u2euB)"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def)

definition init :: "st \<Rightarrow> bool"
  where "init s' \<equiv> \<exists>(v_t_u27u::val) (v_m_u27u::val). s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> type_m v_m_u27u \<and> (csdf_at v_t_u27u state__u2euA \<and> csdf_empty v_m_u27u)"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> type_m v_m_u27u \<and> (csdf_at v_t state__u2euA \<and> csdf_at v_t_u27u state__u2euB \<and> csdf_frame0 v_m_u27u v_m)) \<or> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> type_m v_m_u27u \<and> (csdf_at v_t state__u2euB \<and> csdf_at v_t_u27u state__u2euA \<and> (\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_update v_m_u27u v_m v_k v_t))))"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init s \<Longrightarrow> reachable s"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> type_m v_m_u27u \<and> (csdf_at v_t state__u2euA \<and> csdf_at v_t_u27u state__u2euB \<and> csdf_frame0 v_m_u27u v_m)) \<or> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> type_m v_m_u27u \<and> (csdf_at v_t state__u2euB \<and> csdf_at v_t_u27u state__u2euA \<and> (\<exists>(v_k::val). type__u9375u v_k \<and> (csdf_update v_m_u27u v_m v_k v_t))))"

theorem livelock_free: "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
