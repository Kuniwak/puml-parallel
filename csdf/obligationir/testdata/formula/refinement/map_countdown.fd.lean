import LeanCspProver.CSP_F.CSP_F_Main

-- The guards below are propositions, so the Prop-valued procIte is used rather
-- than the Bool-valued proc.IF.
attribute [local instance] Classical.propDecidable

noncomputable section

namespace Refinement_Obligation

inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))
deriving Inhabited

inductive alphabet where
  | Ev_PUT (a1 : Val)
  | HTau
deriving Inhabited

-- Internal only indexes the replicated internal choices below, via
-- Rep_int_choice_f; no process performs it, so it changes no trace and no
-- refusal. Its injections are of the form fun (x, y) => event.Internal [x, y].
inductive event where
  | Alphabet (a : alphabet)
  | Internal (vs : List Val)
deriving Inhabited

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 は古い
opaque atom__u24u__0_u20u_u306fu_u53e4u_u3044u : Val → Prop

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
-- 鍵
opaque type__u9375u : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- k
opaque type_k : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- m
opaque type_m : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n
opaque type_n : Val → Prop

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
opaque assume_fair_S : Prop

inductive PN where
  | S_s (v_m : Val) -- s (m)
  | I_a (v_n : Val) -- a (n)
deriving Inhabited

def procfun : PN → proc PN event
  | PN.S_s v_m =>
    proc.Ext_pre_choice {ev | ∃ (v_k : Val), type_k v_k ∧ (ev = event.Alphabet (alphabet.Ev_PUT v_k) ∧ (∃ (v_m_u27u : Val), type_m v_m_u27u ∧ ((¬csdf_in v_k v_m) ∧ csdf_frame1 v_m_u27u v_m v_k ∧ csdf_in v_k v_m_u27u)))}
      (fun ev => Rep_int_choice_f (fun v_m_u27u => event.Internal [v_m_u27u]) {v_m_u27u | ∃ (v_k : Val), type_k v_k ∧ (ev = event.Alphabet (alphabet.Ev_PUT v_k) ∧ (type_m v_m_u27u ∧ ((¬csdf_in v_k v_m) ∧ csdf_frame1 v_m_u27u v_m v_k ∧ csdf_in v_k v_m_u27u)))}
        (fun v_m_u27u => proc.Proc_name (PN.S_s v_m_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet alphabet.HTau ∧ (∃ (v_m_u27u : Val), type_m v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m))}
      (fun ev => Rep_int_choice_f (fun v_m_u27u => event.Internal [v_m_u27u]) {v_m_u27u | ev = event.Alphabet alphabet.HTau ∧ (type_m v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m))}
        (fun v_m_u27u => proc.Proc_name (PN.S_s v_m_u27u)))
  | PN.I_a v_n =>
    proc.Ext_pre_choice {ev | ev = event.Alphabet alphabet.HTau ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet alphabet.HTau ∧ (type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))}
        (fun v_n_u27u => proc.Proc_name (PN.I_a v_n_u27u)))

instance Set_procfun : HasPNfun PN event where
  PNfun := procfun

instance Set_FPmode : HasFPmode where
  FPmode := fpmode.CMSmode

def SpecProc : proc PN event :=
  proc.Hiding
    (Rep_int_choice_f (fun v_m_u27u => event.Internal [v_m_u27u]) {v_m_u27u | type_m v_m_u27u ∧ (csdf_empty v_m_u27u)}
        (fun v_m_u27u => proc.Proc_name (PN.S_s v_m_u27u)))
    {event.Alphabet alphabet.HTau}

def ImplProc : proc PN event :=
  proc.Hiding
    (Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)}
        (fun v_n_u27u => proc.Proc_name (PN.I_a v_n_u27u)))
    {event.Alphabet alphabet.HTau}

inductive St_S where
  | St_S_s (v_m : Val) -- s (m)

def init_S (s' : St_S) : Prop :=
  ∃ (v_m_u27u : Val), s' = .St_S_s v_m_u27u ∧ type_m v_m_u27u ∧ (csdf_empty v_m_u27u)

def step_S (s s' : St_S) : Prop :=
  (∃ (v_m : Val) (v_m_u27u : Val), s = .St_S_s v_m ∧ s' = .St_S_s v_m_u27u ∧ type_m v_m_u27u ∧ (∃ (v_k : Val), type_k v_k ∧ ((¬csdf_in v_k v_m) ∧ csdf_frame1 v_m_u27u v_m v_k ∧ csdf_in v_k v_m_u27u))) ∨ (∃ (v_m : Val) (v_m_u27u : Val), s = .St_S_s v_m ∧ s' = .St_S_s v_m_u27u ∧ type_m v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m))

inductive Reachable_S : St_S → Prop where
  | base (s : St_S) : init_S s → Reachable_S s
  | step (s s' : St_S) : Reachable_S s → step_S s s' → Reachable_S s'

def tauStep_S (s s' : St_S) : Prop :=
  (∃ (v_m : Val) (v_m_u27u : Val), s = .St_S_s v_m ∧ s' = .St_S_s v_m_u27u ∧ type_m v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m))

theorem livelock_free_S (h_csdf_axiom_1 : csdf_axiom_1) (h_csdf_axiom_2 : csdf_axiom_2) (h_assume_fair_S : assume_fair_S) :
    WellFounded (fun s' s => Reachable_S s ∧ tauStep_S s s') := by
  sorry

-- Spec admits an initial valuation, so it does not denote DIV. A side that does
-- makes the refinement vacuous, and diverges on the empty trace, which no
-- tau-step argument can rule out.
theorem initialisable_S (h_csdf_axiom_1 : csdf_axiom_1) (h_csdf_axiom_2 : csdf_axiom_2) (h_assume_fair_S : assume_fair_S) : ∃ (v_m_u27u : Val), type_m v_m_u27u ∧ (csdf_empty v_m_u27u) := by
  sorry

inductive St_I where
  | St_I_a (v_n : Val) -- a (n)

def init_I (s' : St_I) : Prop :=
  ∃ (v_n_u27u : Val), s' = .St_I_a v_n_u27u ∧ type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)

def step_I (s s' : St_I) : Prop :=
  (∃ (v_n : Val) (v_n_u27u : Val), s = .St_I_a v_n ∧ s' = .St_I_a v_n_u27u ∧ type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))

inductive Reachable_I : St_I → Prop where
  | base (s : St_I) : init_I s → Reachable_I s
  | step (s s' : St_I) : Reachable_I s → step_I s s' → Reachable_I s'

def tauStep_I (s s' : St_I) : Prop :=
  (∃ (v_n : Val) (v_n_u27u : Val), s = .St_I_a v_n ∧ s' = .St_I_a v_n_u27u ∧ type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))

theorem livelock_free_I (h_csdf_axiom_1 : csdf_axiom_1) (h_csdf_axiom_2 : csdf_axiom_2) :
    WellFounded (fun s' s => Reachable_I s ∧ tauStep_I s s') := by
  sorry

-- Impl admits an initial valuation, so it does not denote DIV. A side that does
-- makes the refinement vacuous, and diverges on the empty trace, which no
-- tau-step argument can rule out.
theorem initialisable_I (h_csdf_axiom_1 : csdf_axiom_1) (h_csdf_axiom_2 : csdf_axiom_2) : ∃ (v_n_u27u : Val), type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u) := by
  sorry

-- lean-csp-prover has no FD model, so this is the standard reduction: for
-- divergence-free processes, FD refinement coincides with the F refinement.
-- Divergence of SpecProc/ImplProc arises from an infinite HTau run of the
-- underlying diagram or from a side that cannot start at all, which the
-- well-foundedness and initialisability obligations above rule out respectively.
-- Every stable failure of the Impl diagram is one of the Spec diagram: refF
-- P1 M1 M2 P2 unfolds to semFf P2 M2 <= semFf P1 M1, and it subsumes trace
-- inclusion.
theorem refines_f (h_csdf_axiom_1 : csdf_axiom_1) (h_csdf_axiom_2 : csdf_axiom_2) : refFfix SpecProc ImplProc := by
  sorry

end Refinement_Obligation

end
