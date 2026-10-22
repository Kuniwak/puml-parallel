inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))

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

opaque csdf_index : Val → Val → Val

opaque csdf_empty : Val → Prop

opaque csdf_frame1 : Val → Val → Val → Prop

opaque csdf_in : Val → Val → Prop

def csdf_axiom_1 : Prop :=
  ∀ (v_m : Val) (v_k : Val), csdf_empty v_m → (¬csdf_in v_k v_m)

def csdf_axiom_2 : Prop :=
  ∀ (v_n : Val) (v_m : Val) (v_k1 : Val), csdf_frame1 v_n v_m v_k1 → (∀ (v_j : Val), (¬(v_j = v_k1)) → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j))))

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

theorem livelock_free (h_csdf_axiom_1 : csdf_axiom_1) (h_csdf_axiom_2 : csdf_axiom_2) (h_assume_fair : assume_fair) :
    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by
  sorry
