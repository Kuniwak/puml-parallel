# Stable Failures Refinement for State-Transition Diagrams with Natural-Language Guards — Comparison of Approaches

## 0. Position of This Document

The LTSs in this repository are defined as “state-transition diagrams with guards (Guard) and postconditions (Post) written in natural language.”
We want to make **stable failures refinement** computable between these diagrams.
The difficult point is that the meanings of transitions are confined inside **natural-language predicates**, and therefore cannot be mechanically evaluated as they are.

This document does not aim at implementation itself. Instead, it compares and organizes **four approaches (Proposal 1, Proposal 2, Proposal 3, and the hybrid approach)** for handling natural language.
It is intended to serve as the basis for the final method selection.

---

## 1. Target Model (Guarded Symbolic LTS with τ)

From the semantics in `docs/SYNTAX.md`, each diagram can be regarded as a **symbolic guarded LTS** with the following structure.

- **Control state (location)** `State`: has variables `Vars` (whose types are arbitrary strings).
- **Edge** `Src --event[Guard]/Post--> Dst`:
  - `Guard`: a natural-language predicate over the current state.
  - `Post`: a natural-language predicate relating the pre-state and the post-state (postcondition = transition relation).
- **StartEdge**: the initial state and its initial condition `Post` (a predicate constraining the initial valuation).
- **EndEdge**: an edge to a terminal state and its `Guard` (termination condition).
- **τ (internal transition)**: an internal transition occurs exactly when `event` is `tau`. → **Stability, refusals, and τ-closure are nontrivial**, and the **full FDR normalisation** is required rather than a naive subset construction.

Thus, in the ground semantics, a state is a configuration `(location ℓ, valuation v)`, and a transition `(ℓ,v) --e--> (ℓ',v')` exists when some edge `t=(ℓ,e,G,P,ℓ')` satisfies `G(v) ∧ P(v,v')`.
The essential obstacles are that **Guard / Post are natural language** and that the valuation domain may be infinite.

### Semantics of Enabledness (Assumption for the Rest of This Document)

An event `e` is **enabled** (executable) at a configuration `(ℓ,v)` only when **the guard holds and there exists a successor satisfying the postcondition**.
Even if `Guard` is true, if `Post` is unsatisfiable, the edge does not give rise to a transition and `e` can be **refused (disabled)**.

```text
Enabled_e(ℓ,v) = ⋁_{edge (ℓ,e,G,P,·)} ( G(v) ∧ ∃v'. P(v,v') )
```

- **Multiple edges with the same event** are aggregated by disjunction (if at least one is executable, then `e` is enabled).
- **Stability**: `stable(ℓ,v) ⟺ ¬Enabled_τ(ℓ,v)` (there is no executable τ-successor).
- **Acceptance set**: `enabled(ℓ,v) = { e ∈ Act | Enabled_e(ℓ,v) }` (visible events only).

> This distinction is the key soundness point for what follows.
> If `Enabled` is approximated by `Guard` alone, refusals caused by edges with unsatisfiable `Post` are missed, producing a **false PASS** (see the counterexample in §3).

---

## 2. Foundation: FDR's Stable Failures Refinement Algorithm

The details are based on the survey results (see the sources).
`Spec ⊑SF Impl` (Spec is refined by Impl) is reduced to a reachability problem in the following three stages.

1. **Normalise (determinise) the Spec** — subset construction. A state is a set of original states `U ⊆ S`.
   - Initial state: the τ-closure of the initial state of the Spec, `{s | ι =⇒ s}`.
   - Transitions: visible events `a` only. `U —a→ V`, `V = {t | ∃s∈U: s =a=> t}` (weak transition `τ*·a·τ*`).
   - The result is deterministic, τ-free, and total. When there is no destination, the state is `∅` (= that trace is absent from the Spec).
   - Each normal-form state `U` keeps the original state set `[[U]]` (needed for computing refusal sets).
2. **Explore the product `norm(Spec) ⋈ Impl` by BFS**: Impl τ-transitions proceed alone (the Spec does not move), while visible `a` transitions proceed synchronously on both sides.
3. **At each reachable configuration `(U, i)`, locally decide violation (SF-witness)**:
   - `U = ∅` (an Impl trace is absent from the Spec → trace violation), **or**
   - `stable(i) ∧ refusals(i) ⊄ refusals([[U]])`.

**Implementation-friendly local decision (acceptance-set = menu form)**:

> No violation at `(U,i)` iff `i` is unstable, or there exists at least one stable Spec state `s` in `[[U]]`
> such that `enabled(s) ⊆ enabled(i)`.

Here, `enabled(·)` is the acceptance set of visible events.
Intuition: if a stable Impl presents menu `A_i`, the Spec set `U` must contain a stable state whose menu is a subset of `A_i` (the Spec must be able to refuse at least as much).

This graph algorithm itself **does not require natural language**.
Natural language appears as **predicate decisions** about “which edges are enabled,” “which traces are feasible,” and “whether refusals can be matched.”
The four approaches differ in **where they push these natural-language predicate decisions**.

---

## 3. Proposal 1: Generation of Symbolic Proof Obligations

### Idea

Use the **control structure** of normalisation, product construction, and witness search as the skeleton, and treat Guard / Post as **opaque predicate symbols**.
Then generate **verification conditions (VCs)** stating that, if these predicates are valid, then SF refinement holds (preferably in an if-and-only-if form).
Evaluation of natural language is separated from the algorithm and pushed into the final “discharge” of obligations.

> **On finiteness and scale** (for details, see [`docs/CONCERN.md`](docs/CONCERN.md)):
> The property of “a finite number of auditable VCs” is **not obtained merely by treating Guard / Post as opaque predicates**.
> One of the following **finitisation mechanisms** must be assumed: **inductive invariants**, **finite abstraction**, or **a search-depth bound**.
> - If VCs are emitted “per path,” then **in the presence of loops there are infinitely many paths**, so enumeration is impossible (O-feasible).
>   Finiteness is obtained only when VCs are emitted as inductive VCs for each **control state (edge)** of the product, and when a **given inductive invariant `Inv` is assumed** (this is the O-trace / O-refusal scheme below).
>   O-feasible cannot be finitised and is used only for **feasibility checking and counterexample generation**.
> - Even with finite control plus inductive invariants, the number of VCs can be **`O(2^S(I+E_I))`** in the worst case due to the subset construction in normalisation (`S` = number of Spec states), and therefore can be **exponential**.
> - In symbolic normalisation that handles state variables precisely, **the number of symbolic states can be infinite even with a single control state**
>   (example: a self-loop `x' = x+1` yields `x=0,1,2,…`). A finite upper bound cannot be derived from only the numbers of control states and edges.
>
> → Countermeasures for scale and finiteness are handled by Layer 1 (on-the-fly reachable product) and Layer 3 (finitisation strategies) in §6.

### Emitted Proof Obligations (Three Families)

Let `Inv(v_q, U)` be an invariant relating an **Impl configuration `(ℓq,v_q)`** and a **normal-form Spec state `U`** (= a set of Spec configurations).
Because nondeterministic Spec processes are handled, this is **not a single valuation but a set**.
In implementation, this corresponds to a family of reachability predicates `R_p(v)` per location.

- **O-feasible (path feasibility)**: the conjunction of Guard / Post along a path is satisfiable.
  `∃v₀..vₙ. Init(v₀) ∧ ⋀ᵢ (Guard_{eᵢ}(vᵢ₋₁) ∧ Post_{eᵢ}(vᵢ₋₁,vᵢ))`
  → determines which traces are feasible (not finitisable; for counterexamples).

- **O-trace (simulation step: trace inclusion)**: for each Impl edge,
  `Inv(v_q,U) ∧ Guard^I(v_q) ∧ Post^I(v_q,v_q')`
  `⟹ ∃(ℓp,v_p)∈U. ⋁_{corresponding Spec edges} ∃v_p'. Guard^S(v_p) ∧ Post^S(v_p,v_p') ∧ Inv(v_q',U')`
  → guarantees `traces(Impl) ⊆ traces(Spec)` (`U'` is the normal-form successor of `U` after `e`).

- **O-refusal (the core of stable failures: refusal inclusion)**: for each reachable stable Impl configuration, within the normal-form set `U` there exists a stable branch whose enabled events are all also enabled on the Impl side.
  `⋁_{(ℓp,v_p)∈U} [ stable^S(ℓp,v_p) ∧ ⋀_{e∈Act} ( Enabled^S_e(ℓp,v_p) ⟹ Enabled^I_e(ℓq,v_q) ) ]`
  → guarantees `refusals(Impl) ⊆ refusals(Spec)`.
  Here `Enabled = Guard ∧ ∃v'.Post` (§1). The key point is that this is an implication between **Enabled** predicates, not merely between Guard predicates.

> **Counterexample (why Guard-only checking is unsound)**:
> Suppose the Spec has an executable `a` edge (`Guard^S_a` is true and `Post^S_a` is satisfiable), while the Impl has an `a` edge whose `Guard^I_a` is true but whose `Post^I_a` is unsatisfiable.
> Then the Impl cannot actually perform `a` and therefore **refuses** it, while the Spec does not refuse `a`; this is a **refinement violation**.
> However, `Guard^S_a ⟹ Guard^I_a` is `true ⟹ true`, so a Guard-based VC misses the violation.
> `Enabled^S_a ⟹ Enabled^I_a` is `true ⟹ false`, and therefore **correctly detects the violation**.
> Similarly, **stability** must be decided using `Enabled_τ` (the presence or absence of executable τ-successors), not the τ Guard.
> Thus **Post affects not only path reachability but also enabledness**.

### Sufficient Conditions vs Necessary and Sufficient Conditions

- **Sufficient condition**: give one inductive `Inv`; if all O-* obligations are valid, refinement holds
  (the standard proof pattern: simulation ⇒ failures refinement). `Inv` is supplied or inferred by humans.
- **Necessary and sufficient condition**: use as `Inv` the **canonical reachability invariant of symbolic normalisation**
  (the strongest postcondition for each trace = the result of symbolic subset construction).
  Since the Spec becomes deterministic by normalisation, simulation against the deterministic side is complete for trace inclusion.
  However, this requires **quantifier elimination over Post and fixed-point computation, and termination is generally not guaranteed**.

### Evaluation

| Perspective | Content |
|:--|:--|
| Soundness | ◎ If corrected to be `Enabled`-based, valid VCs imply refinement. Natural language and the algorithm are separated. |
| Completeness | △ Necessary and sufficient if a canonical `Inv` (set-valued) is used, but symbolic normalisation is generally undecidable / non-terminating. |
| NL dependency | Localized to discharging “a finite number of auditable VCs” **under a given inductive invariant**. |
| Weaknesses | Symbolic data normalisation is the hard part. Nested existential quantifiers over Post can make VCs complex. Automatic construction of canonical `Inv` may not terminate. **The number of VCs can be `O(2^S(I+E_I))` in the worst case, and data plus loops can make the symbolic state space infinite** ([`docs/CONCERN.md`](docs/CONCERN.md)). |

---

## 4. Proposal 2: On-Demand Oracle Evaluation

### Idea

Execute the ground algorithm **concretely**, and query a **pluggable evaluator (oracle)** only when a decision is required.
The evaluator can be selected from the following:

- **Human**: present VCs and obtain yes/no answers (minimal and most reliable, but labor-intensive).
- **LLM**: ask for predicate decisions directly in natural language.
- **Solver**: only when the natural language can be interpreted as a formal language.

The required queries are roughly:

- Enabledness: does `Enabled_e(v) = Guard_e(v) ∧ ∃v'.Post_e(v,v')` hold?
- Successors: what are **all** successor valuations satisfying `Post(v, ?)` (post-image for determinisation)?
- τ-closure: the **complete** closure obtained by following `Enabled_τ`.
- Refusal matching: whether there exists a stable Spec state in `[[U]]` satisfying `enabled(s) ⊆ enabled(i)`.

### Conditions for Soundness (Important)

The condition for soundness is not merely “the domain is enumerable,” but that the **oracle returns the following completely and accurately**:
all initial valuations, **all** successor valuations, the exact truth values of Guard and Post, and the **complete closure** including τ-successors.
**Pointwise queries to an LLM or a human cannot detect missed successors** (missing even one successor breaks soundness).
Caching improves **reproducibility**, but does not improve the **correctness** of answers.

### Evaluation

| Perspective | Content |
|:--|:--|
| Soundness | △ Sound only over finite domains where the oracle returns the above information completely and accurately. Pointwise queries cannot guarantee universal conditions over infinite domains (refusal matching, post-images, and closures). |
| Ease of implementation | ◎ Fits almost directly onto the FDR algorithm. Concrete **counterexample traces** are obtained naturally. |
| NL dependency | Many queries at runtime. **Nondeterminism** in LLMs threatens soundness and reproducibility → caching plus consistency checks still leaves correctness as a separate issue. |
| Positioning | Not a complete verifier by itself. Best suited for **bounded exploration, concrete simulation, counterexample-candidate generation**, and as a mechanism for discharging the VCs of Proposal 1. Results should be returned as `UNKNOWN` / `CANDIDATE_FAILURE`, not `PASS`. |

> **Constraint of the existing code**:
> `PostSolver` (`SolveJSON`) in `csdf/solver.go` receives Guard / Post as inputs, but **does not actually evaluate them**
> (it only binds input values to variables). Turning this into an oracle requires a new implementation of the evaluator.

---

## 5. Proposal 3: Prior Formalisation by LLM → Apply the Algorithm As Is

### Idea

In one pass, translate the NL-LTS into a **formal LTS** (with formal Guard / Post expressed in some decidable theory), and then apply the exact algorithm in §2 plus a solver as is.
Natural language disappears in the initial translation.

### “If the Theory Is Decidable, Then the Whole Problem Is Complete” Does Not Hold

Even if each individual Guard / Post formula is decidable, **reachability for infinite-state transition systems and fixed-point computation for normalisation are not necessarily decidable** (in general, they are undecidable).
To obtain a complete decision procedure, one must additionally have one of the following:
finite valuations, finite abstraction / finite quotient, a restricted model with decidable reachability, bounded checking, or user-provided inductive invariants.

### Practical Solution

Rather than accepting “arbitrary natural language,” define an **explicit DSL** for integers, enumerations, logical connectives, and so on.
Let the LLM generate **candidate DSL translations**, and let **a human approve them**.
This is the practical approach (the LLM is not used as a prover).

### Evaluation

| Perspective | Content |
|:--|:--|
| Soundness | ○ Sound **relative to the translation**. This is separate from soundness with respect to the intended meaning of the natural language. |
| Completeness | △ Even if individual formulae are decidable, reachability and fixed points are separate issues. Finitisation, abstraction, bounds, or invariants are required. |
| NL dependency | Concentrated in one translation step. However, **correctness depends entirely on faithful, human-reviewable translation**. |
| Weaknesses | Ambiguous NL (e.g. “the user is satisfied”) cannot be formalised. **Silent mistranslation causes confident wrong decisions**. A target theory / solver must be selected. |
| Positioning | A strong frontend. Essentially a special case of “oracle = solver.” Trust depends on review of the translation. |

---

## 6. Proposal 4 (Hybrid): Four-Layer Architecture

The three proposals are not competitors; they can be **composed**.
However, it is **not possible to perform control-level normalisation without natural language**.
Normalisation, τ-closure, stability, acceptance sets, and weak post-images all require knowing whether an edge is executable (`Enabled = Guard ∧ ∃v'.Post`).
If all syntactic edges are accepted unconditionally, false Spec transitions hide violations, and a sound `PASS` cannot be returned.
Therefore, natural language should be localized in the “semantic backend,” while the FDR core should be separated so that it consumes an **explicit LTS with resolved semantics**.

- **Layer 1: FDR core** — normalisation, product construction, and SF-witness search over finite, explicit LTSs.
  It does not touch natural language and is **fully testable**.
  (This is the correct scope of “without NL” = `docs/REFINEMENT_ALGORITHM.md`.)
  - **Scale countermeasure (Problem 1: `2^S` explosion)**:
    do not enumerate all normal-form states; instead, **generate only states reachable in the product with Impl on the fly**
    (reachable `R ≪ I·2^S`). **If the Spec is deterministic, reduce to singleton states** and avoid the blow-up.
    In the medium term, use the **antichain method** (Laveaux et al. in the sources) to compress normal-form sets.
- **Layer 2: Semantic backend** — provides `Enabled` / `Successors` / `TauClosure` / `Valid` (satisfiability).
  It passes enabledness, successors, closures, and stability to the FDR core.
  **If it cannot answer, it returns `Unknown`**.
- **Layer 3: Formalisation layer + finitisation strategies** — DSL / SMT / finite enumeration.
  **The LLM generates formalisation candidates**, but is **not treated as a prover** (human approval is required).
  The proof-obligation IR of Proposal 1 and the formaliser of Proposal 3 live here.
  - **Scale countermeasure (Problem 3: infinite data)**:
    ① restrict to **finite-domain types** (enums and bounded integers) = the simplest way to make the ground LTS finite;
    ② use **abstract interpretation + widening** to make reachability predicates `R_p(v)` converge automatically (`x=0,1,2,…` → `x≥0`);
    ③ use **predicate abstraction + CEGAR**;
    ④ use user-provided inductive invariants.
  - **Scale countermeasure (Problem 2: infinite paths)**:
    close `PASS` with per-edge inductive VCs, and handle the counterexample direction with BMC (depth bound) / k-induction
    (return `UNKNOWN` if unresolved).
- **Layer 4: Verdicts** — a **three-valued** result: `PASS / FAIL / UNKNOWN`.
  - `PASS`: only when all obligations have been **proved**.
  - `FAIL`: only when there is a concrete witness whose **feasibility has been confirmed**.
  - `UNKNOWN`: when unresolved obligations remain because the oracle / solver could not decide them.

### Why This Is Good

- The difficult graph algorithm (Layer 1) is separated from natural language and can be kept **testable by TDD**.
- The contact surface with natural language is localized to Layers 2/3 and minimized to **a finite number of auditable obligations**.
- One can start with the simplest backend and add solvers only where formalisation is possible
  — this matches “simplicity first, incremental build-up” precisely.
- Proposal 2 and Proposal 3 need not be chosen exclusively; they can coexist as interchangeable backends, making later replacement easy.
- The three-valued result structurally prevents an **unsound `PASS`** (anything not proved becomes `UNKNOWN`).

### Notes

- Automatic construction of the canonical invariant in Layer 3 (necessary and sufficient condition) involves symbolic normalisation (the hard part of Proposal 1), and is generally non-terminating.
  A practical initial approach is to limit the system to given invariants plus finite enumeration, making necessity and sufficiency depend on data assumptions.
- In layers that use LLMs, **caching (for reproducibility and cost)** should be used, but **correctness is not guaranteed by caching**.
  Important obligations should be cross-checked by another backend or by human approval.

---

## 7. Comparison Table

| Perspective | Proposal 1: Symbolic VCs | Proposal 2: Oracle Execution | Proposal 3: Prior Formalisation | Proposal 4: Hybrid |
|:--|:--|:--|:--|:--|
| Soundness | Currently × / **○ after Enabled correction** | Generally × / ○ under restrictions | ○ with respect to the translated model | Currently × / **○ after redesign** |
| Completeness | △ (necessary and sufficient with set-valued canonical Inv / non-terminating) | △ | △ (even with decidable formulae, reachability is separate) | ○–△ |
| Implementation simplicity | △ | ◎ | ○ | ○ (incremental from Layer 1) |
| NL contact surface | Finite VCs under a given Inv | Many runtime queries | One translation | Localized to Layers 2/3 + replaceable prover |
| Ease of obtaining counterexamples | △ (as failed VCs) | ◎ (concrete traces) | ○ | ◎ (FAIL only when feasibility is confirmed) |
| Main risks | Incorrect enabledness / hard symbolic normalisation / VC complexity | Unsoundness from missed successors; LLM nondeterminism | Silent mistranslation | Integration cost |
| Positioning | Correctness specification | Explorer / counterexample candidate generator (returns UNKNOWN) | Frontend | Integrated method (three-valued verdict) |

---

## 8. Recommendation

**Proposal 4 (the four-layer hybrid) is recommended**.
As described in §6, it (a) keeps the graph algorithm separated from natural language and therefore verifiable, (b) minimizes natural language to a finite number of auditable obligations, and (c) allows the system to start from a simple backend and strengthen only the parts that can be formalised.
This best fits the project policy of “simplicity first, incremental build-up.”
Proposal 1 becomes the “correctness specification,” Proposal 2 becomes a “mechanism for discharging obligations,” and Proposal 3 becomes a “frontend for solver backends” within the hybrid architecture.

> Supplement: a safe implementation staging, if this is implemented in the future, would be:
> 1. Implement the **FDR core (Layer 1) with TDD** over finite LTSs where Guard/Post≡true (fully testable).
> 2. Add a **strict backend for finite-enumeration valuations (Layer 2)**.
> 3. Add **three-valued verdicts (PASS/FAIL/UNKNOWN) + proof-obligation IR (Layers 3/4)**.
> 4. Add a **restricted formal DSL + SMT**.
> 5. Finally add **LLM-assisted formalisation**.
>
> The foundational finite-LTS theorem—that stable failures refinement is equivalent to the absence of an SF-witness in the product of the normalised Spec and Impl—is proved, and FDR3 also checks after normalising the Spec into a τ-free deterministic GLTS.
> This document is for comparing methods; implementation is not included as a goal.
>
> **Minimal configuration for scale countermeasures**:
> **on-the-fly reachable product (Problem 1) + restricting variables to finite domains (Problem 3) + UNKNOWN fallback (Problem 2)**.
> This enters a practical range while remaining sound.
> Antichains, abstract interpretation, and CEGAR should be added only after finite domains are found insufficient ([`docs/CONCERN.md`](docs/CONCERN.md)).

---

## 9. Cross-Cutting Specifications to Fix First (Prerequisites for Method Selection)

Before choosing a method, the following semantic points must be specified, because they directly affect soundness.

- How to handle multiple initial valuations generated by **StartEdge.Post** (initial states are a set of configurations).
- Whether **EndEdge** should be treated as the CSP successful-termination event `✓` (tick), or as mere stopping.
  Note: the current parallel composition does **not support EndEdge** (explicitly errors at `csdf/composition.go:56-58`).
- **Edges with unsatisfiable Post are disabled** (`Enabled = Guard ∧ ∃v'.Post` in §1).
- Enabledness for **multiple edges with the same event** is their **disjunction**.
- **Variable scopes that differ by state**, and **name collisions** during composition (handling of `ComposeStateIDs`, etc.).
- The distinction between **stable failures allowing divergence** and the **divergence-free assumption**.

## 10. Pending Items (Out of Scope Here)

- **State compression** for acceleration (compression other than normalisation).
- **Failures-divergences refinement including divergence** (requires a separate canonical form `normfdr` and post-divergence obscuring, and is complex).
  This document is limited to stable failures (under the divergence-free assumption).
- The concrete decision of the valuation domain (finite, enumerable, or infinite).
  This directly affects soundness and completeness and should be determined after the method is fixed.

---

## 11. Sources

- Gibson-Robinson et al., *FDR3 — A Modern Refinement Checker for CSP* —
  the role of normalisation and the single-threaded refinement algorithm.
- Laveaux, Groote, Willemse, *Correct and Efficient Antichain Algorithms for Refinement
  Checking* (arXiv:1902.09880) — precise definitions of refusal, stable failures, normal form, product, and SF-witnesses.
- *FDR2 User Manual: CSP Refinement* — overview of normalisation and refinement.
