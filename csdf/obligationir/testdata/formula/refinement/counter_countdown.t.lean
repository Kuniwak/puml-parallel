import LeanCspProver.CSP_T.CSP_T_Main

-- Every name is mangled behind a prefix of its kind, and a mangled name holds
-- "__", which Mathlib's naming linter warns of.
set_option linter.style.nameCheck false

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
  | Ev_inc (a1 : Val)
  | Ev_show (a1 : Val)
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
-- $_0 = 0
opaque atom__u24u__0_u20u_u3du_u20u0 : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 = 10
opaque atom__u24u__0_u20u_u3du_u20u10 : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 = $_1 + $_2
opaque atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 : Val → Val → Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 = $_1 - 1
opaque atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 : Val → Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 > 0
opaque atom__u24u__0_u20u_u3eu_u20u0 : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- k
opaque type_k : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n
opaque type_n : Val → Prop

inductive PN where
  | S_s (v_n : Val) -- s (n)
  | I_a (v_n : Val) -- a (n)
deriving Inhabited

def procfun : PN → proc PN event
  | PN.S_s v_n =>
    proc.Ext_pre_choice {ev | ∃ (v_k : Val), type_k v_k ∧ (ev = event.Alphabet (alphabet.Ev_inc v_k) ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_k ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ∃ (v_k : Val), type_k v_k ∧ (ev = event.Alphabet (alphabet.Ev_inc v_k) ∧ (type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_k ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet (alphabet.Ev_show v_n) ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ (v_n_u27u = v_n))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet (alphabet.Ev_show v_n) ∧ (type_n v_n_u27u ∧ (v_n_u27u = v_n))}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet alphabet.HTau ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ (∃ (v_m : Val), type_n v_m ∧ (v_n_u27u = v_m)))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet alphabet.HTau ∧ (type_n v_n_u27u ∧ (∃ (v_m : Val), type_n v_m ∧ (v_n_u27u = v_m)))}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
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
    (Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
    {event.Alphabet alphabet.HTau}

def ImplProc : proc PN event :=
  proc.Hiding
    (Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)}
        (fun v_n_u27u => proc.Proc_name (PN.I_a v_n_u27u)))
    {event.Alphabet alphabet.HTau}

-- Spec admits an initial valuation, so it does not denote DIV, which diverges on
-- the empty trace and, as the Impl, would make the refinement vacuous. Stated
-- under the axioms alone: it must not rest on an assumption.
theorem initialisable_S : ∃ (v_n_u27u : Val), type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u) := by
  sorry

-- Impl admits an initial valuation, so it does not denote DIV, which diverges on
-- the empty trace and, as the Impl, would make the refinement vacuous. Stated
-- under the axioms alone: it must not rest on an assumption.
theorem initialisable_I : ∃ (v_n_u27u : Val), type_n v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u) := by
  sorry

-- Every trace of the Impl diagram is a trace of the Spec diagram: refT P1 M1 M2 P2
-- unfolds to semTf P2 M2 <= semTf P1 M1.
theorem refines_t : refTfix SpecProc ImplProc := by
  sorry

end Refinement_Obligation

end
