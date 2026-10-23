theory Livelock_Obligation
  imports Main
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValRecord val "(string \<times> val) list"
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

(* A map is the list of its entries csdf_canon chooses among the lists of them,
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
    using spec[OF h, of k] unfolding ab
    by (cases "map_of a k"; cases "map_of b k";
        auto simp: csdf_in_def csdf_index_def map_of_eq_None_iff dest: map_of_SomeD)
  then have "map_of a = map_of b" by (rule ext)
  then have "set a = set b" using wa(1) wb(1) by (simp add: map_of_inject_set)
  then have "csdf_canon a = csdf_canon b" by (simp add: csdf_canon_def)
  then show ?thesis using ab wa(2) wb(2) by simp
qed

definition csdf_empty :: "val \<Rightarrow> bool"
  where "csdf_empty v_m \<equiv> csdf_is_map v_m \<and> (\<forall>(v_k::val). \<not> csdf_in v_k v_m)"

definition csdf_update :: "val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_update v_n v_m v_k v_v \<equiv> csdf_is_map v_n \<and> csdf_is_map v_m \<and> (csdf_in v_k v_n \<and> csdf_index v_n v_k = v_v \<and> (\<forall>(v_j::val). (\<not> (v_j = v_k)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j)))))"

definition csdf_frame0 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_frame0 v_n v_m \<equiv> csdf_is_map v_n \<and> csdf_is_map v_m \<and> (\<forall>(v_j::val). True \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"

definition csdf_at :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_at t s \<equiv> (case t of ValRecord st fs \<Rightarrow> st = s | _ \<Rightarrow> False)"

definition state__u2euA :: val
  where "state__u2euA \<equiv> ValInt 0"

definition state__u2euB :: val
  where "state__u2euB \<equiv> ValInt 1"

lemma csdf_lemma_1: "\<forall>(v_m::val) (v_k::val). csdf_empty v_m \<longrightarrow> (\<not> csdf_in v_k v_m)"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def split: val.splits)

lemma csdf_lemma_2: "\<forall>(v_n::val) (v_m::val) (v_k::val) (v_v::val). csdf_update v_n v_m v_k v_v \<longrightarrow> (csdf_in v_k v_n \<and> csdf_index v_n v_k = v_v \<and> (\<forall>(v_j::val). (\<not> (v_j = v_k)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j)))))"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def split: val.splits)

lemma csdf_lemma_3: "\<forall>(v_n::val) (v_m::val). csdf_frame0 v_n v_m \<longrightarrow> (\<forall>(v_j::val). True \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def split: val.splits)

lemma csdf_lemma_4: "\<forall>(v_t::val). \<not> (csdf_at v_t state__u2euA \<and> csdf_at v_t state__u2euB)"
  by (auto simp: csdf_empty_def csdf_update_def csdf_frame0_def csdf_at_def state__u2euA_def state__u2euB_def split: val.splits)

definition init :: "st \<Rightarrow> bool"
  where "init s' \<equiv> \<exists>(v_t_u27u::val) (v_m_u27u::val). s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> csdf_wf v_t_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> (csdf_at v_t_u27u state__u2euA \<and> csdf_empty v_m_u27u)"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> csdf_wf v_t_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> (csdf_at v_t state__u2euA \<and> csdf_at v_t_u27u state__u2euB \<and> csdf_frame0 v_m_u27u v_m)) \<or> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> csdf_wf v_t_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> (csdf_at v_t state__u2euB \<and> csdf_at v_t_u27u state__u2euA \<and> (\<exists>(v_k::val). type__u9375u v_k \<and> csdf_wf v_k \<and> (csdf_update v_m_u27u v_m v_k v_t))))"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init s \<Longrightarrow> reachable s"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> csdf_wf v_t_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> (csdf_at v_t state__u2euA \<and> csdf_at v_t_u27u state__u2euB \<and> csdf_frame0 v_m_u27u v_m)) \<or> (\<exists>(v_t::val) (v_m::val) (v_t_u27u::val) (v_m_u27u::val). s = St_s v_t v_m \<and> s' = St_s v_t_u27u v_m_u27u \<and> type_t v_t_u27u \<and> csdf_wf v_t_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> (csdf_at v_t state__u2euB \<and> csdf_at v_t_u27u state__u2euA \<and> (\<exists>(v_k::val). type__u9375u v_k \<and> csdf_wf v_k \<and> (csdf_update v_m_u27u v_m v_k v_t))))"

theorem livelock_free: "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
