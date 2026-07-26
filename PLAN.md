# csdfrefinement Implementation Plan

## 1. Goal

Add a tool `csdfrefinement` that, like `csdflivelockfree`, **emits proof obligations
instead of deciding verdicts**:

```
csdfrefinement -m <t|trace|f|stable-failure|fd|failures-divergence> \
               [-target <ir-json|isabelle>] \
               path/to/abs.puml path/to/detail.puml
```

- `abs.puml` is the **Spec** (the refined/abstract side), `detail.puml` is the
  **Impl** (the refining/concrete side). The obligation states `Spec ⊑ Impl`
  in the selected model (FDR direction: every behaviour of Impl is allowed by Spec).
- Modes: `t`/`trace` = trace refinement, `f`/`stable-failure` = stable failures
  refinement, `fd`/`failures-divergence` = failures-divergences refinement.
  `-m` is mandatory (no default).
- Targets: `ir-json` (default, prover-agnostic IR) and `isabelle`
  (a theory importing **CSP-Prover**'s `CSP_T`/`CSP_F`).
  `lean` is **deferred** until the in-house Lean translation of CSP-Prover is
  published; until then `-target lean` is rejected with a message saying so.
- Exit status never encodes the verdict. Whether refinement holds depends on the
  natural-language Guard/Post predicates, which stay opaque (same policy as
  `csdflivelockfree`).

Background documents: `docs/REFINEMENT_ALGORITHM_en.md` (control-level FDR
algorithm), and the approach comparison formerly in this file (see git history of
`PLAN.md`; its recommendation evolved into the design below).

## 2. Chosen Approach: CSP-Prover as the Proven Metatheory

Two obligation shapes were considered:

1. **Semantic-direct**: deep-embed both step relations, define traces/failures in
   the theory, state the inclusion theorem. Small tool, but the prover must
   re-invent the whole simulation metatheory per obligation — practically
   undischargeable for non-trivial diagrams.
2. **Simulation VCs**: the tool normalises the Spec and emits per-edge lemmas over
   a placeholder invariant. Easy to discharge, but the VC family's soundness
   becomes a hand-rolled meta-argument inside the Go tool.

**Decision: encode both diagrams as CSP-Prover process terms and state the
obligation as a one-line refinement (`<=T` / `<=F`).** This combines the good
halves: the emitted artifact is tiny and regeneration-stable (like 1), the proof
decomposes into a candidate relation plus small per-edge goals via CSP-Prover's
fixed-point-induction tactics (like 2), and the metatheory is not ours to get
wrong — it is CSP-Prover's machine-checked theory.

Facts verified about CSP-Prover (github.com/yoshinao-isobe/CSP-Prover):

- Provides models **T** (`CSP_T`) and **F** (`CSP_F`) only — **no FD model**.
  fd mode therefore uses the divergence-freedom reduction (§5.3).
- Pinned to **Isabelle2020**; our current verification setup uses a recent
  Isabelle, so obligations are checked in a separate Isabelle2020 + CSP-Prover
  installation (§9).
- License is an LGPL-like custom license. Generated theories only *import*
  CSP-Prover, so this should be unproblematic, but confirm before release.
- Fallback if the Isabelle2020 pin becomes painful or native FD is required:
  AFP **HOL-CSP** (FD model, maintained through Isabelle2025-2), at the cost of
  weaker refinement tactics and of losing parity with the future Lean backend.

## 3. Semantic Decisions (fixed)

- **Enabledness**: `Enabled_e(ℓ,v) = Guard(v) ∧ ∃v'. Post(v,v')`. An edge whose
  Post is unsatisfiable is *disabled* and must contribute a refusal. The CSP
  encoding must guard each prefix with the full enabledness condition (§4);
  guarding with Guard alone is unsound (false PASS via phantom Spec transitions).
- **Multiple edges with the same event**: their enabledness is the disjunction;
  the encoding gets this for free from external choice over per-edge branches.
- **τ**: an edge whose event is `tau`, encoded by a fresh visible event `HTau`
  hidden at the outermost level (§4). Hiding correctly produces instability and,
  on infinite τ-runs, divergence.
- **EndEdge**: treated as CSP successful termination ✓. It maps directly to
  `IF <end guard> THEN SKIP ELSE STOP` in the encoding. (Note this is a point
  where csdfrefinement is *more* permissive than `ComposeParallel`, which rejects
  EndEdge.)
- **Initial valuations**: `StartEdge.Post` denotes a *set* of initial valuations;
  the initial process is a replicated internal choice over that set.
- **Alphabet**: the event datatype is the union of the visible events of both
  diagrams (plus `HTau`). Refusal information depends on this; both processes are
  typed over the single shared event datatype.
- **Event parameters**: out of scope for v1, same as the existing
  `// TODO` in `BuildLivelockFree` (`EventParams` stays empty).

## 4. Encoding a Diagram as a CSP-Prover Process

Sketch for one side (final syntax to be validated against CSP-Prover's bundled
examples, e.g. `ep2`):

```isabelle
theory Refinement_Obligation
  imports CSP_F.CSP_F   (* CSP_T.CSP_T for -m t *)
begin

datatype Event = Ev_a | Ev_b | HTau

(* shared opaque-predicate layer, identical to csdflivelockfree:
   val datatype (when vars exist), pred_<hash> placeholders with the
   NL text as a comment, and per-edge aliases — but namespaced per side:
   guard_S_L<line>, post_S_L<line>, init_S, guard_I_L<line>, ... *)

(* one process-name datatype covering both sides; CSP-Prover has a single
   PNfun per name type, so the two diagrams share it under S_/I_ ctors *)
datatype PN = S_P val | S_Q val | I_R val

primrec PNfun :: "PN ⇒ (PN, Event) proc"
where
  "PNfun (S_P x) =
     (* edge L3: a [g] / p *)
     (IF (guard_S_L3 x ∧ (∃x'. post_S_L3 x x'))
      THEN Ev_a -> (!! x':{x'. post_S_L3 x x'} .. $(S_Q x'))
      ELSE STOP)
     [+] (* further edges of P, incl. HTau -> ... for tau edges *)
     [+] (* EndEdge from P: IF guard_S_L9 x THEN SKIP ELSE STOP *)"
| ...

definition SpecProc :: "(PN, Event) proc"
  where "SpecProc ≡ (!! x:{x. init_S x} .. $(S_P x)) -- {HTau}"
definition ImplProc :: "(PN, Event) proc"
  where "ImplProc ≡ (!! y:{y. init_I y} .. $(I_R y)) -- {HTau}"
```

Encoding rules:

| Diagram construct | CSP term |
|:--|:--|
| state `ℓ` with vars `v` | process name `S_ℓ v` / `I_ℓ v`; body = external choice over its out-edges |
| visible edge `ℓ --e[G]/P--> ℓ'` | `IF (G v ∧ (∃v'. P v v')) THEN e -> (!! v':{v'. P v v'} .. $(ℓ' v')) ELSE STOP` |
| tau edge | same shape with event `HTau`; hidden once, outermost |
| state with no out-edges | `STOP` |
| EndEdge `ℓ --[G]--> [*]` | `IF G v THEN SKIP ELSE STOP` as one more external-choice branch |
| StartEdge `[*] --P--> ℓ` | `!! v:{v. P v} .. $(ℓ v)` |

The explicit `∧ (∃v'. …)` in the guard is the enabledness fix from §3 — do not
"simplify" it away.

## 5. Obligation per Mode

### 5.1 `-m t` (trace refinement)

Theory imports `CSP_T`; single theorem:

```isabelle
theorem refines_t: "SpecProc <=T ImplProc" oops
```

### 5.2 `-m f` (stable failures refinement)

Theory imports `CSP_F`; single theorem (`<=F` subsumes trace inclusion in
CSP-Prover's F model):

```isabelle
theorem refines_f: "SpecProc <=F ImplProc" oops
```

### 5.3 `-m fd` (failures-divergences refinement)

CSP-Prover has no FD model. Use the standard reduction: **for divergence-free
processes, ⊑FD coincides with ⊑F**. The emitted theory contains three theorems,
with a comment stating the reduction and its justification:

```isabelle
(* Divergence of SpecProc/ImplProc arises exactly from infinite HTau runs of the
   underlying diagram; wf of the tau-step relation on reachable states rules
   these out. Given both sides divergence-free, <=F is equivalent to FD
   refinement. *)
theorem livelock_free_S: "wf_on {s. reachable_S s} {(s', s). tau_step_S s s'}" oops
theorem livelock_free_I: "wf_on {s. reachable_I s} {(s', s). tau_step_I s s'}" oops
theorem refines_f: "SpecProc <=F ImplProc" oops
```

The two `wf_on` obligations are **exactly the `csdflivelockfree` obligation**,
built by the existing `BuildLivelockFree` on each input and emitted with the
shared writer (§7), over side-prefixed `step_S`/`tau_step_S`/`reachable_S` (etc.)
definitions. As in `csdflivelockfree`, a side whose structural τ-cycle check
passes (`Structurally == true`) gets a comment instead of a `wf_on` theorem.

## 6. IR Design (`ir-json`)

New builder in `csdf/obligationir`:

```go
type IRRefinementMode string // "trace" | "stable-failure" | "failures-divergence"

type IRSide struct { // one diagram, reusing the existing pieces
    States map[csdf.StateID]IRState
    Edges  []IREdge
    Init   IRInit
    // for mode failures-divergence: the side's livelock IR fields
    StructurallyLivelockFree *bool // nil unless mode is failures-divergence
}

type IRRefinement struct {
    Mode       IRRefinementMode
    Alphabet   []csdf.Event                  // union of visible events, sorted
    Predicates map[IRPredicateID]IRPredicate // shared across both sides
    Constants  []IRConst
    Spec, Impl IRSide
}

func BuildRefinement(mode IRRefinementMode, spec, impl *csdf.Diagram) IRRefinement
```

Naming rules (extends the backend-parity policy — irjson/isabelle/lean must
agree, never fork the spelling):

- `pred_<id>` stays **global and unprefixed**: the ID already hashes text +
  argument types, so identical predicates appearing in both diagrams dedupe into
  one placeholder. Filling it once serves both sides — and serves
  `csdflivelockfree` output for the same diagram, which uses the same spelling.
- Everything derived from a *location* is side-prefixed, because line numbers
  collide between the two files: `guard_S_L<line>` / `post_S_L<line>` /
  `init_S`, state constructors `S_<StateID>` / `I_<StateID>`, relations
  `step_S` / `tau_step_S` / `reachable_S`, and the state datatypes `st_S` /
  `st_I` (fd mode only). Spec = `S`, Impl = `I`, fixed.
- Event constructors `Ev_<event>` plus `HTau`; `HTau` is reserved and collides
  with no user event (`tau` itself is never a constructor).

`BuildRefinement` must first refactor the body of `BuildLivelockFree` so the
per-diagram parts (states → `IRState`s, edges → predicates + `IREdge`s, init)
are extractable helpers shared by both builders (structural change, §8).

## 7. Affinity with csdflivelockfree

Deliberate design constraints so the two tools compose:

1. **One predicate layer, filled once.** Because CSP-Prover is a shallow
   embedding, the process terms reference the very same HOL definitions
   (`pred_<id>` placeholders and their aliases) that `csdflivelockfree` emits.
   A user who has already formalised a diagram's predicates for the livelock
   obligation reuses those bodies verbatim for the refinement obligation.
   Implementation: extract the predicate/val/state-datatype emission from
   `obligationir/isabelle` into shared, prefix-parameterised writers used by
   both the livelock and refinement paths (Tidy First: this extraction is a
   pure structural commit, validated by the existing golden tests).
2. **fd mode embeds the livelock obligation.** §5.3's `wf_on` theorems are the
   `csdflivelockfree` obligation inlined per side. The `wf` form is kept rather
   than restated denotationally because CSP-Prover's F model cannot observe
   divergence at all — the two styles are complementary and coexist in one
   theory (`CSP_F` includes `Main`, so `wf_on` is available).
3. **No behavioural change to csdflivelockfree.** Its CLI and output stay as
   they are; only internals move into shared packages. If desired later, a
   follow-up can teach it a CSP-Prover-flavoured output, but nothing here
   depends on that.

## 8. Implementation Steps (TDD, Tidy First)

Structural commits (S) strictly precede and never mix with behavioural ones (B).

1. **(S)** Extract per-diagram IR-building helpers out of `BuildLivelockFree`
   (states/edges/init/predicate hashing). Existing tests stay green.
2. **(S)** Extract prefix-parameterised Isabelle writers (predicates, aliases,
   state datatype, step/tau_step/reachable) from `obligationir/isabelle`'s
   livelock path. Golden tests unchanged.
3. **(S)** Add `tools.ValidateArgsAsTwoFilePaths` beside
   `ValidateArgsAsFilePath` (two positional args; `-`/stdin allowed for at most
   one). Test-drive it.
4. **(B)** `csdfrefinementcmd` options parsing: mode flag (all six spellings),
   target validation (`lean` rejected with the deferral message), two files,
   usage text. Mirror `csdflivelockfreecmd/options_test.go`.
5. **(B)** `BuildRefinement` for `-m t`: alphabet union, shared predicate map,
   side prefixing. Unit tests on small diagrams (shared predicate dedupes to
   one ID; colliding state IDs/lines across sides stay distinct).
6. **(B)** `ir-json` output for all three modes (mode is just data here; fd adds
   the per-side livelock fields).
7. **(B)** Isabelle backend, staged: (a) ground diagrams — no vars, `True`
   predicates, no τ; (b) vars + opaque predicates (enabledness conjunct!);
   (c) τ edges + hiding; (d) EndEdge → SKIP; (e) `-m f`; (f) `-m fd`
   (per-side `wf_on` reuse). Golden tests at each stage, one increment at a time.
8. **(B)** `tools/csdfrefinement/main.go` + build wiring, matching the other
   tools; end-to-end test via `cli.ProcInoutSpy`.
9. **Validation (not CI-gated at first):** install Isabelle2020 + CSP-Prover in
   the scratchpad, `isabelle build` a generated obligation for a small example,
   **negative control first** (a deliberately broken theory must fail), then a
   hand-discharged positive example (fill predicates, prove `<=F` via
   `cspF_fp_induct_*` + `cspF_auto`-style tactics) to validate the encoding —
   especially the exact process syntax of §4, which must be corrected against
   what Isabelle2020 actually accepts. Feed any syntax fixes back into the
   golden tests.

## 9. Operational Notes

- Obligation checking needs **Isabelle2020 + CSP-Prover** (`-d <csp-prover-dir>`
  session), separate from the repo's current Isabelle setup. Document the
  install + a `ROOT` snippet in the tool's usage/README once step 8.9 pins the
  details.
- Keep the existing rule: when verifying generated theories, always run a
  negative control before trusting a PASS.

## 10. Deferred / Open Items

- **`-target lean`**: blocked on publishing the in-house Lean translation of
  CSP-Prover. The IR and the naming rules (§6) are designed so the Lean backend
  can be added with full pred-name parity later.
- **Structural fast path**: a ground-diagram analogue of `Structurally`
  (deciding refinement outright via the explicit FDR algorithm of
  `docs/REFINEMENT_ALGORITHM_en.md` when all predicates are `True` and there are
  no vars). Valuable, but it is a whole FDR core; not in v1.
- **Event parameters** (`EventParams` TODO shared with livelockfree).
- **CSP-Prover license check** before shipping/distributing generated theories.
- **HOL-CSP fallback** if the Isabelle2020 pin becomes untenable or a native FD
  model obligation is wanted instead of the §5.3 reduction.
