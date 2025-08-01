# puml-refinement Tool

Stable failures refinement verification tool for PlantUML state diagrams.

## Usage

```bash
puml-refinement -spec <spec.puml> -impl <impl.puml>
```

## Example

```bash
# Test guard condition failure case
puml-refinement -spec examples/test_cases/spec_guard_fail.puml -impl examples/test_cases/impl_guard_fail.puml

# Expected output:
# ===== Stable Failures Refinement Verification =====
# 
# Specification: examples/test_cases/spec_guard_fail.puml
# Implementation: examples/test_cases/impl_guard_fail.puml
# 
# === Proof Obligations ===
# 
# [1] Guard Weakening Obligation:
#     Edge: s0 --withdraw(amount)--> s1
#     Specification Guard: balance >= amount
#     Implementation Guard: balance > 0
#     Proof Obligation: (balance > 0) → (balance >= amount)
#     Status: REQUIRES PROOF ⚠️
# 
# [2] Trace Inclusion Obligation:
#     Specification traces must be included in implementation traces
#     Status: REQUIRES VERIFICATION ⚠️
# 
# [3] Refusal Set Inclusion Obligation:
#     Implementation refusal sets must be included in specification refusal sets
#     Status: REQUIRES VERIFICATION ⚠️
# 
# === Summary ===
# Total Obligations: 3
# Requiring Proof: 3
# Refinement Status: REQUIRES MANUAL VERIFICATION
```

## Implementation

The tool:
1. Parses both specification and implementation PUML files using `core.NewParser()`
2. Creates a `core.StableFailuresVerifier` instance
3. Generates proof obligations using `GenerateStableFailuresProofObligations()`
4. Formats and outputs the results using `core.FormatProofObligations()`

All core refinement verification logic is implemented in `core/refinement.go` to avoid code duplication.