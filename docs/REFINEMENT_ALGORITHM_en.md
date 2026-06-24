# FDR's Stable Failures Refinement Algorithm (Survey Results)

This document organizes the algorithm for deciding **stable failures refinement** in FDR (Failures-Divergences Refinement), based on primary sources (see §10).
State compression for acceleration is out of scope; this document describes the **simplest version**.
As recalled, the core contains **determinisation (= normalisation)**.

For the design-level choice of method, in particular the treatment of natural-language guards, see `PLAN.md`.
This document is limited to the underlying “control-level algorithm.”

---

## 1. Notation

- `Act`: the set of visible events. `τ`: an internal (invisible) event. `Actτ = Act ∪ {τ}`.
- `s --a--> t`: a one-step transition (`a ∈ Actτ`).
- `s =⇒ t`: a **weak transition**. From `s`, reach `t` by following τ zero or more times (`τ*`).
- `s =ρ=> t`: a **weak trace transition** (`ρ ∈ Act*`). Follow `τ*` and visible events alternately (`τ* a₁ τ* a₂ … τ*`).
- `enabled(s) = { a ∈ Actτ | ∃t. s --a--> t }`.
- `weaktraces(s) = { ρ ∈ Act* | ∃t. s =ρ=> t }`.

We call the process being refined **Spec**, and the refining process **Impl**.
The FDR direction is `Spec ⊑ Impl` (Spec is refined by Impl; equivalently, all behaviours of Impl are allowed by Spec).

---

## 2. Definition of the Stable Failures Model

- **Stable**: `stable(s) ⟺ τ ∉ enabled(s)` (the state cannot move by an internal transition; observation has settled).
- **Refusal sets**: for a stable state `s`, `refusals(s) = P(Act \ enabled(s))`.
  These are all subsets of the set of events that `s` cannot perform (downward closed).
  For a set of states `U ⊆ S`, define `refusals(U) = { X ⊆ Act | ∃s∈U. stable(s) ∧ X ∈ refusals(s) }`.
- **Stable failures**:
  `failures(s) = { (ρ, X) ∈ Act* × P(Act) | ∃t. s =ρ=> t ∧ stable(t) ∧ X ∈ refusals(t) }`.
  This is the set of observations in which, after weak trace `ρ`, a stable state is reached and `X` can be refused there.

### Refinement Relations

- **Trace refinement**: `Spec ⊑tr Impl ⟺ weaktraces(Impl) ⊆ weaktraces(Spec)`.
- **Stable failures refinement**:
  `Spec ⊑sfr Impl ⟺ failures(Impl) ⊆ failures(Spec) and weaktraces(Impl) ⊆ weaktraces(Spec)`.

> Note: the trace-inclusion clause is **mandatory**. Some implementations in the literature omit it, but this is erroneous (as pointed out by the antichain paper cited in §10).

---

## 3. Overall Algorithm (Three Stages)

Reduce `Spec ⊑sfr Impl` to a **reachability problem**.

1. **Normalise (determinise) the Spec** → `norm(Spec)` (§4).
2. **Construct and explore the product `norm(Spec) ⋈ Impl`** (§5).
3. At each reachable configuration, **locally decide violation (witness)** (§6).

Only the Spec is normalised; the Impl is inserted into the product as is.
The exploration loop itself is a simple BFS (§7).

---

## 4. Normalisation (Subset Construction = Determinisation)

Define `norm(L) = (S', ι', →')` as follows. A state is a **set of original states** `U ⊆ S`.

- **State set**: `S' = P(S)` (the powerset).
- **Initial state**: `ι' = { s ∈ S | ι =⇒ s }` (the **τ-closure** of the initial state of the Spec).
- **Transitions**: visible events `a ∈ Act` only.
  `U --a-->' V ⟺ V = { t ∈ S | ∃s ∈ U. s =a=> t }` (collecting targets by weak trace transition).

Properties: `norm(L)` is **deterministic, τ-free (concrete), and total (universal)**.
When the target set for some `a` is empty, the destination state is `∅` (`∅` is also a state).

- Meaning of `∅`: `ρ` is **not in** `weaktraces(L)` iff `∅` is reached from `ι'` by `ρ`.

> **Difference from ordinary determinisation**: normalisation can **add weak traces that were not present** in the original LTS
> because it keeps state sets for refusal information.
> Therefore, each normal-form state `U` must retain the corresponding **set of original states**
> (written as `[[U]]` in this document). This is used to compute `refusals([[U]])`.

---

## 5. Product and Exploration

Transition rules for the product `norm(Spec) ⋈ Impl`:

- Impl `τ`: `(U, i) --τ--> (U, i')` (the Spec side `U` does not move, because the normal form has no τ).
- Visible `a`: `(U, i) --a--> (U', i')` (both sides move synchronously, with `U --a-->' U'` and `i --a--> i'`).

Enumerate all configurations reachable from the initial configuration `(ι', root(Impl))` using BFS.
BFS yields a **minimum counterexample trace** when a violation is found.

---

## 6. Local Decision of Violations (SF-witnesses)

A reachable configuration `(U, i)` is an **SF-witness (= refinement violation)** iff either of the following holds:

1. `U = ∅` (an Impl trace is absent from the Spec → **trace violation**), **or**
2. `stable(i)` and `refusals(i) ⊄ refusals([[U]])` (**refusal violation**).

**Theorem**: `Spec ⊑sfr Impl ⟺ there is no reachable SF-witness in the product norm(Spec) ⋈ Impl`.
(Trace refinement is equivalent to unreachability of a “TR-witness,” namely `U=∅`.)

### Implementation-Friendly Form (Acceptance Sets = Menus)

Since refusal sets are downward closed, for condition 2 it suffices to check whether **the maximum refusal of `i`, namely `Act \ enabled(i)`, is in `refusals([[U]])`**.
By rewriting:

```
refusals(i) ⊆ refusals([[U]])
  ⟺ (Act \ enabled(i)) ∈ refusals([[U]])
  ⟺ ∃ stable s ∈ [[U]]. enabled(s) ⊆ enabled(i)
```

Therefore:

> **No violation at `(U, i)` iff `i` is unstable, or there exists at least one stable state `s` in `[[U]]`
> such that `enabled(s) ⊆ enabled(i)`.**
>
> **`(U, i)` is an SF-witness iff `U = ∅`, or
> (`i` is stable ∧ for every stable state `s` in `[[U]]`, `enabled(s) ⊄ enabled(i)`).**

Intuition: if a stable Impl state `i` presents the menu `A_i = enabled(i)`, then the Spec set `[[U]]` must contain
a stable state whose menu is a subset of `A_i` (the Spec must be able to refuse at least as much).
Here, `enabled(·)` refers only to visible events (a stable state has no τ).

> In implementation, FDR equips each normal-form state with **minimal acceptance sets** and checks whether one of them is contained in `enabled(i)`.
> This is equivalent to the formulation above.

---

## 7. Single-Threaded Pseudocode (Corresponding to FDR3, Figure 1)

```
function Refines(S, I, M):          # S = normalised Spec, I = Impl, M = semantic model
  done    = {}
  current = { (root(S), root(I)) }
  next    = {}
  while current ≠ {}:
    for (s, i) in current \ done:
      locally check whether "i refines s in the semantics of M"   # ← only this part is model-dependent (§6)
      done += (s, i)
      for (e, i') in transitions(I, i):
        if e == τ:
          next += (s, i')                 # Spec does not move
        else:
          t = transitions(S, s, e)
          if t == {}:
            Report trace error            # Spec cannot perform this e (corresponds to U=∅)
          else:
            {s'} = t                      # the normal form is deterministic, so the successor is unique
            next += (s', i')
    current = next
    next = {}
```

- `M = traces`: no local check is needed (`trace error` accounts for trace violations).
- `M = stable failures`: local check = the refusal check in §6 (for stable `i`, whether `s'` has a minimal acceptance set satisfying `enabled(s') ⊆ enabled(i)`).

---

## 8. Degenerate Case: When There Is No τ (No Hiding)

If the composed LTS has no τ (internal transitions), the above algorithm simplifies substantially.
In the model of this repository, τ arises only when `event` is exactly `tau` (`docs/SYNTAX.md`).
In a τ-free subset:

- `=⇒` is identity, `=a=>` is a one-step transition, and **all states are stable**.
- Normalisation = naive subset determinisation (no τ-closure required).
- Violation decision for `(U, i)`: `U = ∅`, or `∀ s ∈ U. enabled(s) ⊄ enabled(i)`.
- Nondeterminism arises when **the Spec diagram contains multiple edges with the same event** → hence determinisation is required.

This is the first target point when “simplicity first” is the priority; τ and divergence can be added incrementally afterward.

---

## 9. Correspondence with This Repository's Data Structures

- `csdf.Diagram` (`States`, `Edges{Src,Dst,Event}`, `StartEdge`) corresponds to an LTS.
- `ComposeParallel` is the existing path that constructs the composed LTS on the Impl side.
- Additional required concepts:
  ① normal-form states (`Set[StateID]` + minimal acceptance sets),
  ② `normalize(Diagram) → NormalForm`,
  ③ BFS for `refinesSF(spec, impl) → (ok, counterexample trace)`.
- Existing `StatePair` / `Trans` (`composition.go`) can be reused directly for product exploration.

> **Assumption**: the algorithm in this document targets an “**explicit LTS with resolved enabledness**.”
> In a symbolic model (with natural-language guards), the semantic backend must first resolve the executability of each edge,
> `Enabled_e = Guard ∧ ∃v'.Post` (edges with unsatisfiable Post are disabled), together with τ-closure and acceptance sets,
> before applying this algorithm.
> Enabledness must not be approximated by `Guard` alone (this can cause false PASS results).
> The general treatment of natural language is outside the scope of this document (see `PLAN.md`).

---

## 10. Sources

- Gibson-Robinson, Armstrong, Boulgakov, Roscoe,
  *FDR3 — A Modern Refinement Checker for CSP* — the role of normalisation and
  the single-threaded refinement algorithm (Figure 1).
- Laveaux, Groote, Willemse,
  *Correct and Efficient Antichain Algorithms for Refinement Checking* (arXiv:1902.09880) —
  precise definitions and correctness proof for refusal, stable failures, normal form, product, and SF-witnesses.
- *FDR2 User Manual: CSP Refinement* — overview of normalisation and refinement.
