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
  | St_a (v_n : Val) -- a (n)

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 = 10
opaque atom__u24u__0_u20u_u3du_u20u10 : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 = $_1 - 1
opaque atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 : Val → Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 > 0
opaque atom__u24u__0_u20u_u3eu_u20u0 : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n
opaque type_n : Val → Prop

def init (s' : St) : Prop :=
  ∃ (v_n_u27u : Val), s' = .St_a v_n_u27u ∧ type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)

def step (s s' : St) : Prop :=
  (∃ (v_n : Val) (v_n_u27u : Val), s = .St_a v_n ∧ s' = .St_a v_n_u27u ∧ type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))

inductive Reachable : St → Prop where
  | base (s : St) : init s → Reachable s
  | step (s s' : St) : Reachable s → step s s' → Reachable s'

def tauStep (s s' : St) : Prop :=
  (∃ (v_n : Val) (v_n_u27u : Val), s = .St_a v_n ∧ s' = .St_a v_n_u27u ∧ type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))

theorem livelock_free :
    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by
  sorry
