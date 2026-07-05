theory Livelock_Obligation imports Main begin
(* structurally_livelock_free: false *)
datatype json = JSONInt int | JSONString string | JSONBool bool | JSONArray "json list" | JSONDict "(string \<times> json) list"
datatype st =
    a json (* declared: Nat *)

(* "n > 0" *)
definition guard_L5 :: "json \<Rightarrow> bool"
  where "guard_L5 x \<equiv> case x
    of JSONInt n \<Rightarrow> n > 0
     | _ \<Rightarrow> False"

(* "n' = n - 1" *)
definition post_L5 :: "json \<Rightarrow> json \<Rightarrow> bool"
  where "post_L5 x x' \<equiv> case x
    of JSONInt n \<Rightarrow> x' = JSONInt (n - 1)
     | _ \<Rightarrow> False"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool" where
  "tau_step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_L5 n \<and> post_L5 n n'"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
unfolding tau_step_def guard_L5_def post_L5_def proof -
  have eq: "{(s', s). \<exists>n n'. s = a n \<and> s' = a n' \<and> (case n of JSONInt n \<Rightarrow> 0 < n | _ \<Rightarrow> False) \<and> (case n of JSONInt n \<Rightarrow> n' = JSONInt (n - 1) | _ \<Rightarrow> False)} = (map_prod (a \<circ> JSONInt) (a \<circ> JSONInt) \<circ> (\<lambda>n. (n - 1, n))) ` Collect ((<) 0)" proof auto
    fix n n'
    assume 1: "case n of JSONInt x \<Rightarrow> 0 < x | _ \<Rightarrow> False"
      and 2: "case n of JSONInt n \<Rightarrow> n' = JSONInt (n - 1) | _ \<Rightarrow> False"
    obtain n1 where eq1: "n = JSONInt n1" and gt: "0 < n1" using 1 by (cases n; simp)
    have eq2: "n' = JSONInt (n1 - 1)" using 2 unfolding eq1 by simp
    show "(a n', a n) \<in> (\<lambda>x. (a (JSONInt (x - 1)), a (JSONInt x))) ` Collect ((<) 0)" unfolding eq2 using eq1 gt by blast
  qed
  show "wf {(s', s). \<exists>n n'. s = a n \<and> s' = a n' \<and> (case n of JSONInt n \<Rightarrow> 0 < n | _ \<Rightarrow> False) \<and> (case n of JSONInt n \<Rightarrow> n' = JSONInt (n - 1) | _ \<Rightarrow> False)}" unfolding eq proof -
    have 1: "wf (map_prod (a \<circ> JSONInt) (a \<circ> JSONInt) ` (((\<lambda>n :: int. (n - 1, n)) ` Collect ((<) 0))))" proof (rule wf_map_prod_image)
      show "wf ((\<lambda>n :: int. (n - 1, n)) ` Collect ((<) 0))" unfolding wf_def proof auto
        fix P x
        assume 1[rule_format]: "\<forall>x. (\<forall>y. (y, x) \<in> (\<lambda>n :: int. (n - 1, n)) ` Collect ((<) 0) \<longrightarrow> P y) \<longrightarrow> P x"
        show "P x" proof (induct x)
          case (nonneg n)
          show ?case proof (induct n)
            case 0
            show ?case using 1 by auto
          next
            case (Suc n)
            show ?case proof (rule 1)
              fix y
              assume "(y, int (Suc n)) \<in> (\<lambda>n. (n - 1, n)) ` Collect ((<) 0)"
              thus "P y" using Suc by fastforce
            qed
          qed
        next
          case (neg n)
          show "P (- int (Suc n))" proof (rule 1)
            fix y
            assume "(y, - int (Suc n)) \<in> (\<lambda>n. (n - 1, n)) ` Collect ((<) 0)"
            hence False by fastforce
            thus "P y" by simp
          qed
        qed
      qed
    next
      show "inj (a \<circ> JSONInt)" by (simp add: linorder_inj_onI')
    qed
    show "wf ((map_prod (a \<circ> JSONInt) (a \<circ> JSONInt) \<circ> (\<lambda>n. (n - 1, n))) ` Collect ((<) 0))" using 1 by (simp add: image_comp)
  qed
qed
end
