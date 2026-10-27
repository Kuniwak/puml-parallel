import LeanCspProver.CSP_F.CSP_F_Main
import Mathlib.Data.List.Sort
import Mathlib.Order.RelClasses
import Mathlib.SetTheory.Cardinal.Order

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
  | ValFloat (neg : Bool) (sig : Nat) (exp : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))
  | ValRecord (state : Val) (fields : List (String × Val))
  | ValMap (kvs : List (Val × Val))
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
-- 鍵 ⇸ 値
opaque type__u9375u_u20u_u21f8u_u20u_u5024u : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n
opaque type_n : Val → Prop

-- Values are well ordered, as every type is (the library's WellOrderingRel):
-- a linear order the keys of a map are sorted in.
noncomputable instance : LinearOrder Val := IsWellOrder.linearOrder WellOrderingRel

-- A value is well formed when every map in it has distinct keys, sorted,
-- and every float in it is in its canonical form.
inductive csdf_wf : Val → Prop where
  | int (i : Int) : csdf_wf (.ValInt i)
  | float (s : Bool) (m : Nat) (e : Int) : m % 2 = 1 ∨ (s = false ∧ m = 0 ∧ e = 0) →
      csdf_wf (.ValFloat s m e)
  | string (s : String) : csdf_wf (.ValString s)
  | bool (b : Bool) : csdf_wf (.ValBool b)
  | array (xs : List Val) : (∀ x ∈ xs, csdf_wf x) → csdf_wf (.ValArray xs)
  | dict (kvs : List (String × Val)) : (∀ p ∈ kvs, csdf_wf p.2) → csdf_wf (.ValDict kvs)
  | record (st : Val) (fs : List (String × Val)) : csdf_wf st → (∀ p ∈ fs, csdf_wf p.2) →
      csdf_wf (.ValRecord st fs)
  | map (kvs : List (Val × Val)) : (kvs.map Prod.fst).Nodup → (kvs.map Prod.fst).SortedLE →
      (∀ p ∈ kvs, csdf_wf p.1) → (∀ p ∈ kvs, csdf_wf p.2) → csdf_wf (.ValMap kvs)

def csdf_is_map : Val → Prop
  | .ValMap _ => True
  | _ => False

def csdf_in (k m : Val) : Prop :=
  match m with
  | .ValMap kvs => ∃ v, (k, v) ∈ kvs
  | _ => False

open Classical in
noncomputable def csdf_index (m k : Val) : Val :=
  match m with
  | .ValMap kvs => if h : ∃ v, (k, v) ∈ kvs then Classical.choose h else .ValBool false
  | _ => .ValBool false

theorem csdf_unique : ∀ (kvs : List (Val × Val)), (kvs.map Prod.fst).Nodup →
    ∀ k v w, (k, v) ∈ kvs → (k, w) ∈ kvs → v = w
  | [], _, _, _, _, hv, _ => by simp at hv
  | (j, x) :: rest, hn, k, v, w, hv, hw => by
    simp only [List.map_cons, List.nodup_cons, List.mem_map] at hn
    simp only [List.mem_cons, Prod.mk.injEq] at hv hw
    rcases hv with ⟨rfl, rfl⟩ | hv <;> rcases hw with ⟨hk, rfl⟩ | hw
    · rfl
    · exact absurd ⟨(k, w), hw, rfl⟩ hn.1
    · subst hk; exact absurd ⟨(k, v), hv, rfl⟩ hn.1
    · exact csdf_unique rest hn.2 k v w hv hw

theorem csdf_map_half : ∀ (a b : List (Val × Val)), (a.map Prod.fst).Nodup →
    (∀ k, (csdf_in k (.ValMap a) ↔ csdf_in k (.ValMap b)) ∧
      (csdf_in k (.ValMap a) → csdf_index (.ValMap a) k = csdf_index (.ValMap b) k)) →
    ∀ p, p ∈ a → p ∈ b := by
  intro a b na h p hp
  obtain ⟨k, v⟩ := p
  have ina : ∃ v, (k, v) ∈ a := ⟨v, hp⟩
  have inb : ∃ v, (k, v) ∈ b := (h k).1.mp ina
  have he := (h k).2 ina
  simp only [csdf_index, dif_pos ina, dif_pos inb] at he
  have v1 : Classical.choose ina = v := csdf_unique a na k _ _ (Classical.choose_spec ina) hp
  rw [← v1, he]
  exact Classical.choose_spec inb

-- Entries of distinct keys, sorted, are in the order of their keys.
theorem csdf_sorted_entries (kvs : List (Val × Val)) (n : (kvs.map Prod.fst).Nodup)
    (s : (kvs.map Prod.fst).SortedLE) : kvs.Pairwise (fun p q => p.1 < q.1) :=
  List.pairwise_map.mp (List.sortedLT_iff_nodup_and_sortedLE.mpr ⟨n, s⟩).pairwise

-- Two well-formed maps holding the same entries are one.
theorem csdf_map_ext : ∀ (n m : Val), csdf_wf n → csdf_wf m → csdf_is_map n → csdf_is_map m →
    (∀ (k : Val), (csdf_in k n ↔ csdf_in k m) ∧ (csdf_in k n → csdf_index n k = csdf_index m k)) → n = m := by
  intro n m wn wm hn hm h
  cases wn <;> cases wm <;> simp only [csdf_is_map] at hn hm
  case map.map a na sa _ _ b nb sb _ _ =>
    have perm : a.Perm b := (List.perm_ext_iff_of_nodup na.of_map nb.of_map).mpr fun p =>
      ⟨csdf_map_half a b na h p,
       csdf_map_half b a nb (fun k => ⟨(h k).1.symm, fun i => ((h k).2 ((h k).1.mpr i)).symm⟩) p⟩
    rw [perm.eq_of_pairwise (fun _ _ _ _ h1 h2 => absurd (lt_trans h1 h2) (lt_irrefl _))
      (csdf_sorted_entries a na sa) (csdf_sorted_entries b nb sb)]

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
opaque assume_fair_S : Prop

inductive PN where
  | S_s (v_m : Val) (v_memo : Val) -- s (m, memo)
  | I_a (v_n : Val) -- a (n)
deriving Inhabited

def procfun : PN → proc PN event
  | PN.S_s v_m v_memo =>
    proc.Ext_pre_choice {ev | ∃ (v_k : Val), type__u9375u v_k ∧ csdf_wf v_k ∧ (ev = event.Alphabet (alphabet.Ev_PUT v_k) ∧ (∃ (v_m_u27u : Val) (v_memo_u27u : Val), type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ ((¬csdf_in v_k v_m) ∧ csdf_frame1 v_m_u27u v_m v_k ∧ csdf_in v_k v_m_u27u ∧ v_memo_u27u = v_memo)))}
      (fun ev => Rep_int_choice_f (fun (v_m_u27u, v_memo_u27u) => event.Internal [v_m_u27u, v_memo_u27u]) {(v_m_u27u, v_memo_u27u) | ∃ (v_k : Val), type__u9375u v_k ∧ csdf_wf v_k ∧ (ev = event.Alphabet (alphabet.Ev_PUT v_k) ∧ (type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ ((¬csdf_in v_k v_m) ∧ csdf_frame1 v_m_u27u v_m v_k ∧ csdf_in v_k v_m_u27u ∧ v_memo_u27u = v_memo)))}
        (fun (v_m_u27u, v_memo_u27u) => proc.Proc_name (PN.S_s v_m_u27u v_memo_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet alphabet.HTau ∧ (∃ (v_m_u27u : Val) (v_memo_u27u : Val), type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ csdf_wf v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m ∧ v_memo_u27u = v_memo))}
      (fun ev => Rep_int_choice_f (fun (v_m_u27u, v_memo_u27u) => event.Internal [v_m_u27u, v_memo_u27u]) {(v_m_u27u, v_memo_u27u) | ev = event.Alphabet alphabet.HTau ∧ (type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ csdf_wf v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m ∧ v_memo_u27u = v_memo))}
        (fun (v_m_u27u, v_memo_u27u) => proc.Proc_name (PN.S_s v_m_u27u v_memo_u27u)))
  | PN.I_a v_n =>
    proc.Ext_pre_choice {ev | ev = event.Alphabet alphabet.HTau ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet alphabet.HTau ∧ (type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))}
        (fun v_n_u27u => proc.Proc_name (PN.I_a v_n_u27u)))

instance Set_procfun : HasPNfun PN event where
  PNfun := procfun

instance Set_FPmode : HasFPmode where
  FPmode := fpmode.CMSmode

def SpecProc : proc PN event :=
  proc.Hiding
    (Rep_int_choice_f (fun (v_m_u27u, v_memo_u27u) => event.Internal [v_m_u27u, v_memo_u27u]) {(v_m_u27u, v_memo_u27u) | type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ (csdf_empty v_m_u27u ∧ v_memo_u27u = (Val.ValInt 0))}
        (fun (v_m_u27u, v_memo_u27u) => proc.Proc_name (PN.S_s v_m_u27u v_memo_u27u)))
    {event.Alphabet alphabet.HTau}

def ImplProc : proc PN event :=
  proc.Hiding
    (Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)}
        (fun v_n_u27u => proc.Proc_name (PN.I_a v_n_u27u)))
    {event.Alphabet alphabet.HTau}

inductive St_S where
  | St_S_s (v_m : Val) (v_memo : Val) -- s (m, memo)

def init_S (s' : St_S) : Prop :=
  ∃ (v_m_u27u : Val) (v_memo_u27u : Val), s' = .St_S_s v_m_u27u v_memo_u27u ∧ type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ (csdf_empty v_m_u27u ∧ v_memo_u27u = (Val.ValInt 0))

def step_S (s s' : St_S) : Prop :=
  (∃ (v_m : Val) (v_memo : Val) (v_m_u27u : Val) (v_memo_u27u : Val), s = .St_S_s v_m v_memo ∧ s' = .St_S_s v_m_u27u v_memo_u27u ∧ type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ (∃ (v_k : Val), type__u9375u v_k ∧ csdf_wf v_k ∧ ((¬csdf_in v_k v_m) ∧ csdf_frame1 v_m_u27u v_m v_k ∧ csdf_in v_k v_m_u27u ∧ v_memo_u27u = v_memo))) ∨ (∃ (v_m : Val) (v_memo : Val) (v_m_u27u : Val) (v_memo_u27u : Val), s = .St_S_s v_m v_memo ∧ s' = .St_S_s v_m_u27u v_memo_u27u ∧ type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ csdf_wf v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m ∧ v_memo_u27u = v_memo))

inductive Reachable_S : St_S → Prop where
  | base (s : St_S) : init_S s → Reachable_S s
  | step (s s' : St_S) : Reachable_S s → step_S s s' → Reachable_S s'

def tauStep_S (s s' : St_S) : Prop :=
  (∃ (v_m : Val) (v_memo : Val) (v_m_u27u : Val) (v_memo_u27u : Val), s = .St_S_s v_m v_memo ∧ s' = .St_S_s v_m_u27u v_memo_u27u ∧ type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ ((∃ (v_k : Val), type__u9375u v_k ∧ csdf_wf v_k ∧ (csdf_in v_k v_m ∧ atom__u24u__0_u20u_u306fu_u53e4u_u3044u v_k)) ∧ v_m_u27u = v_m ∧ v_memo_u27u = v_memo))

theorem livelock_free_S (h_assume_fair_S : assume_fair_S) :
    WellFounded (fun s' s => Reachable_S s ∧ tauStep_S s s') := by
  sorry

-- Spec admits an initial valuation, so it does not denote DIV, which diverges on
-- the empty trace and, as the Impl, would make the refinement vacuous. Stated
-- under no hypothesis: it must not rest on an assumption.
theorem initialisable_S : ∃ (v_m_u27u : Val) (v_memo_u27u : Val), type__u9375u_u20u_u21f8u_u20u_u5024u v_m_u27u ∧ csdf_wf v_m_u27u ∧ (csdf_empty v_m_u27u ∧ v_memo_u27u = (Val.ValInt 0)) := by
  sorry

inductive St_I where
  | St_I_a (v_n : Val) -- a (n)

def init_I (s' : St_I) : Prop :=
  ∃ (v_n_u27u : Val), s' = .St_I_a v_n_u27u ∧ type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u)

def step_I (s s' : St_I) : Prop :=
  (∃ (v_n : Val) (v_n_u27u : Val), s = .St_I_a v_n ∧ s' = .St_I_a v_n_u27u ∧ type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))

inductive Reachable_I : St_I → Prop where
  | base (s : St_I) : init_I s → Reachable_I s
  | step (s s' : St_I) : Reachable_I s → step_I s s' → Reachable_I s'

def tauStep_I (s s' : St_I) : Prop :=
  (∃ (v_n : Val) (v_n_u27u : Val), s = .St_I_a v_n ∧ s' = .St_I_a v_n_u27u ∧ type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_n ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2du_u20u1 v_n_u27u v_n))

theorem livelock_free_I :
    WellFounded (fun s' s => Reachable_I s ∧ tauStep_I s s') := by
  sorry

-- Impl admits an initial valuation, so it does not denote DIV, which diverges on
-- the empty trace and, as the Impl, would make the refinement vacuous. Stated
-- under no hypothesis: it must not rest on an assumption.
theorem initialisable_I : ∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u10 v_n_u27u) := by
  sorry

-- lean-csp-prover has no FD model, so this is the standard reduction: for
-- divergence-free processes, FD refinement coincides with the F refinement.
-- Divergence of SpecProc/ImplProc arises from an infinite HTau run of the
-- underlying diagram or from a side that cannot start at all, which the
-- well-foundedness and initialisability obligations above rule out respectively.
-- Every stable failure of the Impl diagram is one of the Spec diagram: refF
-- P1 M1 M2 P2 unfolds to semFf P2 M2 <= semFf P1 M1, and it subsumes trace
-- inclusion.
theorem refines_f : refFfix SpecProc ImplProc := by
  sorry

end Refinement_Obligation

end
