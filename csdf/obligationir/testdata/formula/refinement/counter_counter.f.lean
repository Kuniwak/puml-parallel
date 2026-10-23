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
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))
  | ValRecord (state : Val) (fields : List (String × Val))
  | ValMap (kvs : List (Val × Val))
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
-- $_0 = $_1 + $_2
opaque atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 : Val → Val → Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- $_0 > 0
opaque atom__u24u__0_u20u_u3eu_u20u0 : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- k
opaque type_k : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n
opaque type_n : Val → Prop

-- Values are well ordered, as every type is (the library's WellOrderingRel):
-- a linear order the keys of a map are sorted in.
noncomputable instance : LinearOrder Val := IsWellOrder.linearOrder WellOrderingRel

-- A value is well formed when every map in it has distinct keys, sorted.
inductive csdf_wf : Val → Prop where
  | int (i : Int) : csdf_wf (.ValInt i)
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

inductive PN where
  | S_s (v_n : Val) -- s (n)
  | I_s (v_n : Val) -- s (n)
deriving Inhabited

def procfun : PN → proc PN event
  | PN.S_s v_n =>
    proc.Ext_pre_choice {ev | ∃ (v_k : Val), type_k v_k ∧ csdf_wf v_k ∧ (ev = event.Alphabet (alphabet.Ev_inc v_k) ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_k ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ∃ (v_k : Val), type_k v_k ∧ csdf_wf v_k ∧ (ev = event.Alphabet (alphabet.Ev_inc v_k) ∧ (type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_k ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet (alphabet.Ev_show v_n) ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (v_n_u27u = v_n))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet (alphabet.Ev_show v_n) ∧ (type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (v_n_u27u = v_n))}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet alphabet.HTau ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (∃ (v_m : Val), type_n v_m ∧ csdf_wf v_m ∧ (v_n_u27u = v_m)))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet alphabet.HTau ∧ (type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (∃ (v_m : Val), type_n v_m ∧ csdf_wf v_m ∧ (v_n_u27u = v_m)))}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
  | PN.I_s v_n =>
    proc.Ext_pre_choice {ev | ∃ (v_k : Val), type_k v_k ∧ csdf_wf v_k ∧ (ev = event.Alphabet (alphabet.Ev_inc v_k) ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_k ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ∃ (v_k : Val), type_k v_k ∧ csdf_wf v_k ∧ (ev = event.Alphabet (alphabet.Ev_inc v_k) ∧ (type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3eu_u20u0 v_k ∧ atom__u24u__0_u20u_u3du_u20u_u24u__1_u20u_u2bu_u20u_u24u__2 v_n_u27u v_n v_k)))}
        (fun v_n_u27u => proc.Proc_name (PN.I_s v_n_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet (alphabet.Ev_show v_n) ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (v_n_u27u = v_n))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet (alphabet.Ev_show v_n) ∧ (type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (v_n_u27u = v_n))}
        (fun v_n_u27u => proc.Proc_name (PN.I_s v_n_u27u)))
    [+]
    proc.Ext_pre_choice {ev | ev = event.Alphabet alphabet.HTau ∧ (∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (∃ (v_m : Val), type_n v_m ∧ csdf_wf v_m ∧ (v_n_u27u = v_m)))}
      (fun ev => Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | ev = event.Alphabet alphabet.HTau ∧ (type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (∃ (v_m : Val), type_n v_m ∧ csdf_wf v_m ∧ (v_n_u27u = v_m)))}
        (fun v_n_u27u => proc.Proc_name (PN.I_s v_n_u27u)))

instance Set_procfun : HasPNfun PN event where
  PNfun := procfun

instance Set_FPmode : HasFPmode where
  FPmode := fpmode.CMSmode

def SpecProc : proc PN event :=
  proc.Hiding
    (Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)}
        (fun v_n_u27u => proc.Proc_name (PN.S_s v_n_u27u)))
    {event.Alphabet alphabet.HTau}

def ImplProc : proc PN event :=
  proc.Hiding
    (Rep_int_choice_f (fun v_n_u27u => event.Internal [v_n_u27u]) {v_n_u27u | type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u)}
        (fun v_n_u27u => proc.Proc_name (PN.I_s v_n_u27u)))
    {event.Alphabet alphabet.HTau}

-- Spec admits an initial valuation, so it does not denote DIV, which diverges on
-- the empty trace and, as the Impl, would make the refinement vacuous. Stated
-- under no hypothesis: it must not rest on an assumption.
theorem initialisable_S : ∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u) := by
  sorry

-- Impl admits an initial valuation, so it does not denote DIV, which diverges on
-- the empty trace and, as the Impl, would make the refinement vacuous. Stated
-- under no hypothesis: it must not rest on an assumption.
theorem initialisable_I : ∃ (v_n_u27u : Val), type_n v_n_u27u ∧ csdf_wf v_n_u27u ∧ (atom__u24u__0_u20u_u3du_u20u0 v_n_u27u) := by
  sorry

-- Every stable failure of the Impl diagram is one of the Spec diagram: refF
-- P1 M1 M2 P2 unfolds to semFf P2 M2 <= semFf P1 M1, and it subsumes trace
-- inclusion.
theorem refines_f : refFfix SpecProc ImplProc := by
  sorry

end Refinement_Obligation

end
