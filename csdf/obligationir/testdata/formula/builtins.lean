inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))
  | ValMap (kvs : List (Val × Val))

instance : Inhabited Val := ⟨.ValBool false⟩

inductive St where
  | St_s (v_t : Val) (v_m : Val) -- s (t, m)

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- 鍵
opaque type__u9375u : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- m
opaque type_m : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- t
opaque type_t : Val → Prop

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

def csdf_update (v_n v_m v_k v_v : Val) : Prop :=
  csdf_is_map v_n ∧ csdf_is_map v_m ∧ (csdf_in v_k v_n ∧ csdf_index v_n v_k = v_v ∧ (∀ (v_j : Val), (¬(v_j = v_k)) → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j)))))

def csdf_frame0 (v_n v_m : Val) : Prop :=
  csdf_is_map v_n ∧ csdf_is_map v_m ∧ (∀ (v_j : Val), True → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j))))

-- TODO(csdf): not formalised; the state an instance is at is uninterpreted.
opaque csdf_state : Val → Val

def csdf_at (t s : Val) : Prop := csdf_state t = s

def state__u2euA : Val := .ValInt 0

def state__u2euB : Val := .ValInt 1

theorem csdf_lemma_1 : ∀ (v_m : Val) (v_k : Val), csdf_empty v_m → (¬csdf_in v_k v_m) :=
  fun _ k h => And.right h k

theorem csdf_lemma_2 : ∀ (v_n : Val) (v_m : Val) (v_k : Val) (v_v : Val), csdf_update v_n v_m v_k v_v → (csdf_in v_k v_n ∧ csdf_index v_n v_k = v_v ∧ (∀ (v_j : Val), (¬(v_j = v_k)) → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j))))) :=
  fun _ _ _ _ h => And.right (And.right h)

theorem csdf_lemma_3 : ∀ (v_n : Val) (v_m : Val), csdf_frame0 v_n v_m → (∀ (v_j : Val), True → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j)))) :=
  fun _ _ h => And.right (And.right h)

theorem csdf_lemma_4 : ∀ (v_t : Val), ¬(csdf_at v_t state__u2euA ∧ csdf_at v_t state__u2euB) :=
  fun _ h => absurd (Eq.trans (Eq.symm h.1) h.2) (by simp [state__u2euA, state__u2euB])

def init (s' : St) : Prop :=
  ∃ (v_t_u27u : Val) (v_m_u27u : Val), s' = .St_s v_t_u27u v_m_u27u ∧ type_t v_t_u27u ∧ type_m v_m_u27u ∧ (csdf_at v_t_u27u state__u2euA ∧ csdf_empty v_m_u27u)

def step (s s' : St) : Prop :=
  (∃ (v_t : Val) (v_m : Val) (v_t_u27u : Val) (v_m_u27u : Val), s = .St_s v_t v_m ∧ s' = .St_s v_t_u27u v_m_u27u ∧ type_t v_t_u27u ∧ type_m v_m_u27u ∧ (csdf_at v_t state__u2euA ∧ csdf_at v_t_u27u state__u2euB ∧ csdf_frame0 v_m_u27u v_m)) ∨ (∃ (v_t : Val) (v_m : Val) (v_t_u27u : Val) (v_m_u27u : Val), s = .St_s v_t v_m ∧ s' = .St_s v_t_u27u v_m_u27u ∧ type_t v_t_u27u ∧ type_m v_m_u27u ∧ (csdf_at v_t state__u2euB ∧ csdf_at v_t_u27u state__u2euA ∧ (∃ (v_k : Val), type__u9375u v_k ∧ (csdf_update v_m_u27u v_m v_k v_t))))

inductive Reachable : St → Prop where
  | base (s : St) : init s → Reachable s
  | step (s s' : St) : Reachable s → step s s' → Reachable s'

def tauStep (s s' : St) : Prop :=
  (∃ (v_t : Val) (v_m : Val) (v_t_u27u : Val) (v_m_u27u : Val), s = .St_s v_t v_m ∧ s' = .St_s v_t_u27u v_m_u27u ∧ type_t v_t_u27u ∧ type_m v_m_u27u ∧ (csdf_at v_t state__u2euA ∧ csdf_at v_t_u27u state__u2euB ∧ csdf_frame0 v_m_u27u v_m)) ∨ (∃ (v_t : Val) (v_m : Val) (v_t_u27u : Val) (v_m_u27u : Val), s = .St_s v_t v_m ∧ s' = .St_s v_t_u27u v_m_u27u ∧ type_t v_t_u27u ∧ type_m v_m_u27u ∧ (csdf_at v_t state__u2euB ∧ csdf_at v_t_u27u state__u2euA ∧ (∃ (v_k : Val), type__u9375u v_k ∧ (csdf_update v_m_u27u v_m v_k v_t))))

theorem livelock_free :
    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by
  sorry
