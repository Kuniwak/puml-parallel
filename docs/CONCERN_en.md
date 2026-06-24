
# Concerns About Scale and Finiteness of the Proof Obligations in Proposal 1

## Overview

Proposal 1 in `PLAN.md` assumes that a finite number of proof obligations (VCs) referring to Guard / Post are generated from control-level normalization, product construction, and witness search.

However, this finiteness and scale require additional assumptions.

- Even assuming finite control LTSs and inductive invariants, the number of VCs can be exponential in the number of states of the Spec in the worst case.
- In an approach that directly expands paths, the number of obligations becomes exponential in the search depth and, in the presence of loops, is generally infinite.
- In symbolic normalization that accurately includes state variables, the number of symbolic states can be infinite even when there is only one control state.

Therefore, the property of “a finite number of auditable VCs” is not obtained merely by treating Guard / Post as opaque predicates.
Some separate finiteness mechanism is required, such as inductive invariants, finite abstraction, or a bound on the search depth.

## Parameters

We use the following parameters.

- \(S\): the number of control states of the Spec
- \(I\): the number of control states of the Impl
- \(E_S\): the number of edges in the Spec
- \(E_I\): the number of edges in the Impl
- \(A\): the number of visible event kinds
- \(R\): the number of reachable product states of the normalized Spec and Impl
- \(T\): the number of reachable product transitions

## When Using Inductive Invariants

Since a normalized state of the Spec is a subset of control states, there are at most \(2^S\) such states.
Therefore, the number of product states and product transitions has the following upper bounds.

\[
R \le I2^S
\]

\[
T \le E_I2^S
\]

If the initial condition, transition preservation, and refusal condition are separated into distinct VCs, the number of VCs is roughly of the following scale.

\[
N_{VC} \approx 1 + T + R
\]

If path existence is made a separate VC for each product transition, the scale is as follows.

\[
N_{VC} \approx 1 + 2T + R
\]

Thus, in the worst case,

\[
N_{VC}=O(2^S(I+E_I))
\]

Even if multiple VCs are combined into a single huge proposition, the total formula size does not decrease. Instead of reducing the number of VCs, the individual propositions become huge.

## Size of O-trace

For one Impl transition, a disjunction is required to select the corresponding Spec edge.

Let the number of edges labeled with event \(a\) from Spec states contained in the normalized state \(U\) be as follows.

\[
B_S(U,a)=\sum_{p\in U}d_S(p,a)
\]

An O-trace roughly has the following form.

\[
assumption \Rightarrow
\bigvee_{k=1}^{B_S(U,a)}
  (Guard_k \land Post_k \land Inv_k)
\]

Therefore, the number of principal terms in one O-trace VC has the following scale.

\[
O(B_S(U,a))
\]

In the worst case, \(B_S(U,a)\le E_S\), so the number of predicate references contained in all O-trace VCs can grow up to the following scale.

\[
O(TE_S)=O(2^SE_IE_S)
\]

In practice, the number of outgoing edges with the same event is usually small, so the actual scale is smaller than this upper bound.

## Size of O-refusal

When checking enabledness inclusion for each Spec state and each event in a normalized state \(U\), an O-refusal roughly has the following form.

\[
\bigvee_{p\in U}
  \left(
    Stable_S(p)
    \land
    \bigwedge_{a\in Act}
      (Enabled_S(p,a)\Rightarrow Enabled_I(i,a))
  \right)
\]

The number of top-level terms has the following scale.

\[
|U|A \le SA
\]

However, correct enabledness is not determined by Guard alone. The condition that an event \(a\) is enabled is the disjunction, over all edges with the same event, of the condition that Guard holds and that there exists a successor valuation satisfying Post.

\[
Enabled(p,a)
=
\bigvee_{e:p\xrightarrow{a}}
  \left(
    Guard_e(v)
    \land
    \exists v'.Post_e(v,v')
  \right)
\]

Therefore, the number of Guard / Post references contained in one O-refusal is roughly of the following scale.

\[
O(E_S(U)+|U|\cdot d_I(i))
\]

In the worst case, the scale is as follows.

\[
O(E_S+S\cdot d_I(i))
\]

Summing over all product states, up to roughly the following number of Guard / Post references can be generated.

\[
O\left(2^S(IE_S+SE_I)\right)
\]

Stability checking with respect to \(\tau\) also requires formulae of a comparable size expressing the absence of executable \(\tau\)-successors.

## When Generating O-feasible Per Path

An O-feasible obligation for a path of length \(L\) roughly has the following form.

\[
\exists v_0,\ldots,v_L.
Init(v_0)\land
\bigwedge_{k=1}^{L}
  (Guard_k(v_{k-1})\land Post_k(v_{k-1},v_k))
\]

Therefore, the number of terms in one proposition is \(O(L)\). If each state has \(V\) variables, the number of quantified variables is \(O(LV)\).

On the other hand, if the average branching factor is \(b\), the number of paths up to depth \(L\) has the following scale.

\[
O(b^L)
\]

The total formula size has the following scale.

\[
O(Lb^L)
\]

If loops exist, the number of paths is infinite. Therefore, the method of generating O-feasible obligations for all paths is generally not implementable. To make it finite, it is necessary to transform it into state- and transition-level VCs using inductive invariants.

## A Concrete Example of the Maximum Scale

Assume the following conditions.

- Number of Impl states: \(I=10\)
- Number of Impl edges: \(E_I=30\)
- Number of visible event kinds: \(A=8\)

Even considering only the control part, the upper bounds have the following scale.

| Number of Spec states \(S\) | Normalized states \(2^S\) | Product-state upper bound \(I2^S\) | Product-transition upper bound \(E_I2^S\) | Approximate number of VCs \(R+T\) |
|---:|---:|---:|---:|---:|
| 10 | 1,024 | 10,240 | 30,720 | about 40,000 |
| 15 | 32,768 | 327,680 | 983,040 | about 1.31 million |
| 20 | 1,048,576 | 10,485,760 | 31,457,280 | about 41.94 million |

One refusal VC contains at most \(SA\) enabledness implications. For example, if \(S=20\), this is about 160.
Multiplying this by all product states, a naive textual expansion can result in several billion terms.

In practice, only reachable subsets are generated, so in many cases the scale is smaller than this upper bound. However, depending on the structure, even a Spec with around 20 states can become impractical.

## Problems That Cannot Be Estimated from the Number of States Alone

The above upper bounds are based on the assumption that normalization can be performed using only subsets of control states.

When state variables are handled precisely, different symbolic states are generated even for the same control state if their reachability conditions differ. For example, consider the following self-loop.

```text
x' = x + 1
```

In this case, different reachability conditions such as `x=0`, `x=1`, `x=2`, and so on can appear depending on the number of trace iterations.
As a result, the following problems arise.

- Even when the number of control states is \(S=1\), the number of symbolic states can be infinite.
- Strongest-postcondition formulae grow with each iteration.
- Formulae can blow up rapidly due to quantifier elimination.
- A finite upper bound cannot be derived solely from the number of control states or edges.

## Conclusion

The scale and finiteness of Proposal 1 fall into the following three levels, depending on the implementation strategy.

1. **Finite control plus user-provided inductive invariants**
   - The number of VCs is roughly \(O(2^S(I+E_I))\).
   - Although exponential, this can be implementable if the reachable subset space is small.

2. **Depth-bounded path expansion**
   - For depth \(L\), the total formula size is \(O(Lb^L)\).
   - This is implementable as bounded checking, but it is not a complete decision procedure.

3. **Exact symbolic normalization or full path expansion**
   - Finiteness cannot be guaranteed in the presence of state variables and loops.
   - A finite upper bound on the number of VCs cannot be shown from only the numbers of control states and edges.

Therefore, the statement in `PLAN.md` that there will be “a finite number of auditable VCs” needs to explicitly state the assumption “when inductive invariants or finite abstractions are provided.”
Treating Guard / Post as opaque predicates in itself does not guarantee that the proof obligations become finite or small.
