Semantics
=========

Interface Parallel
------------------
### Notation
Premises and conclusion are written in the following format:

```
⟦ Premise1; Premise2; ... ⟧ ⟹ Conclusion
```

Where `Premise1`, `Premise2`, etc. are the premises, and `Conclusion` is the conclusion.


### Transition
If process `P` transitions to `P'` by event `α`, then:

```
P -α-> P'
```

`X` is a set of all events except `τ` (tau) and `✔︎` (tick).
`Ω` is the successful termination state, which is reached by any process after a `✔︎` event.

### Para1
```
⟦ P -α-> P'; ¬ α ∈ X∪{✔︎} ⟧ ⟹ P [|X|] Q -α-> P' [|X|] Q
```

### Para2
```
⟦ Q -α-> Q'; ¬ α ∈ X∪{✔︎} ⟧ ⟹ P [|X|] Q -α-> P [|X|] Q'
```

### Para3
```
⟦ P -α-> P'; Q -α-> Q'; α ∈ X ⟧ ⟹ P [|X|] Q -α-> P' [|X|] Q'
```

### Para4
```
⟦ P -✔︎-> Ω ⟧ ⟹ P [|X|] Q -✔︎-> Ω [|X|] Q
```

### Para5
```
⟦ Q -✔︎-> Ω ⟧ ⟹ P [|X|] Q -✔︎-> P [|X|] Ω
```

### Para6
```
Ω [|X|] Ω -✔︎-> Ω
```

Refinements
-----------

