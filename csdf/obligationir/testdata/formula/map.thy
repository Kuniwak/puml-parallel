theory Livelock_Obligation
  imports Main "HOL-Library.Countable"
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValRecord val "(string \<times> val) list"
  | ValMap "(val \<times> val) list"

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

(* Values are countable, so ordered by their numbers: a linear order of the
   library's, which the keys of a map are sorted in. *)
instance val :: countable by countable_datatype

instantiation val :: linorder
begin
definition less_eq_val :: "val \<Rightarrow> val \<Rightarrow> bool" where "less_eq_val x y \<equiv> to_nat x \<le> to_nat y"
definition less_val :: "val \<Rightarrow> val \<Rightarrow> bool" where "less_val x y \<equiv> to_nat x < to_nat y"
instance by standard (auto simp: less_eq_val_def less_val_def)
end

(* A value is well formed when every map in it has distinct keys, sorted. *)
inductive csdf_wf :: "val \<Rightarrow> bool"
  where "csdf_wf (ValInt i)"
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

definition csdf_empty :: "val \<Rightarrow> bool"
  where "csdf_empty v_m \<equiv> csdf_is_map v_m \<and> (\<forall>(v_k::val). \<not> csdf_in v_k v_m)"

definition csdf_frame1 :: "val \<Rightarrow> val \<Rightarrow> val \<Rightarrow> bool"
  where "csdf_frame1 v_n v_m v_k1 \<equiv> csdf_is_map v_n \<and> csdf_is_map v_m \<and> (\<forall>(v_j::val). (\<not> (v_j = v_k1)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"

lemma csdf_lemma_1: "\<forall>(v_m::val) (v_k::val). csdf_empty v_m \<longrightarrow> (\<not> csdf_in v_k v_m)"
  by (auto simp: csdf_empty_def csdf_frame1_def split: val.splits)

lemma csdf_lemma_2: "\<forall>(v_n::val) (v_m::val) (v_k1::val). csdf_frame1 v_n v_m v_k1 \<longrightarrow> (\<forall>(v_j::val). (\<not> (v_j = v_k1)) \<longrightarrow> ((csdf_in v_j v_n \<longrightarrow> csdf_in v_j v_m) \<and> (csdf_in v_j v_m \<longrightarrow> (csdf_in v_j v_n \<and> csdf_index v_n v_j = csdf_index v_m v_j))))"
  by (auto simp: csdf_empty_def csdf_frame1_def split: val.splits)

(* TODO(csdf): not formalised; this assumption is uninterpreted. *)
(* PUT は無限に起きない *)
consts assume_fair :: "bool"

definition init :: "st \<Rightarrow> bool"
  where "init s' \<equiv> \<exists>(v_m_u27u::val). s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> (csdf_empty v_m_u27u)"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> (\<exists>(v_m::val) (v_m_u27u::val). s = St_s v_m \<and> s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> (\<exists>(v_k::val). type_k v_k \<and> csdf_wf v_k \<and> ((\<not> csdf_in v_k v_m) \<and> csdf_frame1 v_m_u27u v_m v_k \<and> csdf_in v_k v_m_u27u))) \<or> (\<exists>(v_m::val) (v_m_u27u::val). s = St_s v_m \<and> s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> csdf_wf v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init s \<Longrightarrow> reachable s"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> (\<exists>(v_m::val) (v_m_u27u::val). s = St_s v_m \<and> s' = St_s v_m_u27u \<and> type_m v_m_u27u \<and> csdf_wf v_m_u27u \<and> ((\<exists>(v_k::val). type__u9375u v_k \<and> csdf_wf v_k \<and> (csdf_in v_k v_m \<and> atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) \<and> v_m_u27u = v_m))"

theorem livelock_free:
  assumes "assume_fair"
  shows "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
