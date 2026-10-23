inductive Scalar where
  | SInt (i : Int)
  | SString (s : String)
  | SBool (b : Bool)
  | SArray (xs : List Scalar)
  | SDict (kvs : List (String × Scalar))
  | SRecord (state : Scalar) (fields : List (String × Scalar))
deriving Inhabited

inductive Val where
  | ValScalar (s : Scalar)
  | ValMap (f : Scalar → Option Scalar)

instance : Inhabited Val := ⟨.ValScalar (.SBool false)⟩

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

def csdf_is_map : Val → Prop
  | .ValMap _ => True
  | _ => False

def csdf_in : Val → Val → Prop
  | .ValScalar s, .ValMap f => f s ≠ none
  | _, _ => False

def csdf_index : Val → Val → Val
  | .ValMap f, .ValScalar s =>
    match f s with
    | some v => .ValScalar v
    | none => .ValScalar (.SBool false)
  | _, _ => .ValScalar (.SBool false)

theorem csdf_map_ext : ∀ (n m : Val), csdf_is_map n → csdf_is_map m →
    (∀ (k : Val), (csdf_in k n ↔ csdf_in k m) ∧ (csdf_in k n → csdf_index n k = csdf_index m k)) → n = m
  | .ValMap f, .ValMap g, _, _, h => by
    congr 1
    funext s
    have h1 := (h (.ValScalar s)).1
    have h2 := (h (.ValScalar s)).2
    cases hf : f s <;> cases hg : g s <;> simp_all [csdf_in, csdf_index]
  | .ValScalar _, _, hn, _, _ => False.elim hn
  | .ValMap _, .ValScalar _, _, hm, _ => False.elim hm

def csdf_empty (v_m : Val) : Prop :=
  csdf_is_map v_m ∧ (∀ (v_k : Val), ¬csdf_in v_k v_m)

def csdf_update (v_n v_m v_k v_v : Val) : Prop :=
  csdf_is_map v_n ∧ csdf_is_map v_m ∧ (csdf_in v_k v_n ∧ csdf_index v_n v_k = v_v ∧ (∀ (v_j : Val), (¬(v_j = v_k)) → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j)))))

def csdf_frame0 (v_n v_m : Val) : Prop :=
  csdf_is_map v_n ∧ csdf_is_map v_m ∧ (∀ (v_j : Val), True → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j))))

def csdf_at : Val → Val → Prop
  | .ValScalar (.SRecord st _), s => .ValScalar st = s
  | _, _ => False

def state__u2euA : Val := .ValScalar (.SInt 0)

def state__u2euB : Val := .ValScalar (.SInt 1)

theorem csdf_lemma_1 : ∀ (v_m : Val) (v_k : Val), csdf_empty v_m → (¬csdf_in v_k v_m) :=
  fun _ k h => And.right h k

theorem csdf_lemma_2 : ∀ (v_n : Val) (v_m : Val) (v_k : Val) (v_v : Val), csdf_update v_n v_m v_k v_v → (csdf_in v_k v_n ∧ csdf_index v_n v_k = v_v ∧ (∀ (v_j : Val), (¬(v_j = v_k)) → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j))))) :=
  fun _ _ _ _ h => And.right (And.right h)

theorem csdf_lemma_3 : ∀ (v_n : Val) (v_m : Val), csdf_frame0 v_n v_m → (∀ (v_j : Val), True → ((csdf_in v_j v_n → csdf_in v_j v_m) ∧ (csdf_in v_j v_m → (csdf_in v_j v_n ∧ csdf_index v_n v_j = csdf_index v_m v_j)))) :=
  fun _ _ h => And.right (And.right h)

theorem csdf_lemma_4 : ∀ (v_t : Val), ¬(csdf_at v_t state__u2euA ∧ csdf_at v_t state__u2euB) :=
  by
    intro t h
    cases t with
    | ValScalar s => cases s <;> simp_all [csdf_at, state__u2euA, state__u2euB]
    | ValMap _ => exact h.1

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
