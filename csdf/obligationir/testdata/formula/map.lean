inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))
  | ValMap (kvs : List (Val × Val))

instance : Inhabited Val := ⟨.ValBool false⟩

inductive St where
  | St_s (v_m : Val) -- s (m)

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 は古い
opaque atom__u24u__0_u20u_u306fu_u53e4u_u3044u : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- 鍵
opaque type__u9375u : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- k
opaque type_k : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- m
opaque type_m : Val → Prop

-- A map is a ValMap; the entry of a key is the first one holding it.
open Classical in
noncomputable def csdf_lookup : List (Val × Val) → Val → Val
  | [], _ => .ValBool false
  | (j, v) :: kvs, k => if j = k then v else csdf_lookup kvs k

def csdf_is_map (m : Val) : Prop :=
  match m with
  | .ValMap _ => True
  | _ => False

def csdf_in (k m : Val) : Prop :=
  match m with
  | .ValMap kvs => k ∈ kvs.map Prod.fst
  | _ => False

noncomputable def csdf_index (m k : Val) : Val :=
  match m with
  | .ValMap kvs => csdf_lookup kvs k
  | _ => .ValBool false

def csdf_empty (v_m : Val) : Prop :=
  csdf_is_map v_m ∧ (∀ (v_k : Val), ¬csdf_in v_k v_m)

def csdf_frame1 (v_n v_m v_k1 : Val) : Prop :=
  csdf_is_map v_n ∧ csdf_is_map v_m ∧ (∀ (v_j : Val), (¬(v_j = v_k1)) → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j))))

theorem csdf_lemma_1 : ∀ (v_m : Val) (v_k : Val), csdf_empty v_m → (¬csdf_in v_k v_m) :=
  fun _ k h => And.right h k

theorem csdf_lemma_2 : ∀ (v_n : Val) (v_m : Val) (v_k1 : Val), csdf_frame1 v_n v_m v_k1 → (∀ (v_j : Val), (¬(v_j = v_k1)) → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j)))) :=
  fun _ _ _ h => And.right (And.right h)

-- TODO(csdf): not formalised; this assumption is uninterpreted.
-- PUT は無限に起きない
opaque assume_fair : Prop

def init (s' : St) : Prop :=
  ∃ (v_m_u27u : Val), s' = .St_s v_m_u27u ∧ type_m v_m_u27u ∧ (csdf_empty v_m_u27u)

def step (s s' : St) : Prop :=
  (∃ (v_m : Val) (v_m_u27u : Val), s = .St_s v_m ∧ s' = .St_s v_m_u27u ∧ type_m v_m_u27u ∧ (∃ (v_k : Val), type_k v_k ∧ ((¬csdf_in v_k v_m) ∧ csdf_frame1 v_m_u27u v_m v_k ∧ csdf_in v_k v_m_u27u))) ∨ (∃ (v_m : Val) (v_m_u27u : Val), s = .St_s v_m ∧ s' = .St_s v_m_u27u ∧ type_m v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m))

inductive Reachable : St → Prop where
  | base (s : St) : init s → Reachable s
  | step (s s' : St) : Reachable s → step s s' → Reachable s'

def tauStep (s s' : St) : Prop :=
  (∃ (v_m : Val) (v_m_u27u : Val), s = .St_s v_m ∧ s' = .St_s v_m_u27u ∧ type_m v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m))

theorem livelock_free (h_assume_fair : assume_fair) :
    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by
  sorry
