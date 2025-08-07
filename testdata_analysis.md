# Core Testdata Refinement Analysis

## Expected Refinement Relationships (from README.md)

According to `core/testdata/README.md`, the following refinement relationships should hold:

### Positive Refinements (should succeed)
1. `abint [F= abext` - abext refines abint
2. `abint [F= a` - a refines abint  
3. `vmi [F= vmdtea` - vmdtea refines vmi
4. `vmi [F= vmdalt` - vmdalt refines vmi
5. `vmi [F= vmd` - vmd refines vmi

### Negative Refinements (should fail)
1. `not (abint [F= stop)` - stop does NOT refine abint
2. `not (a [F= abint)` - abint does NOT refine a

## Analysis of PUML Files

### Basic Processes
- **a.puml**: Simple process `0 --a--> 1`
- **stop.puml**: STOP process with no transitions
- **abint.puml**: Internal choice between `a→SKIP` and `b→SKIP` using tau transitions
- **abext.puml**: External choice between `a→SKIP` and `b→SKIP`

### Vending Machine Processes
- **vmi.puml**: Vending machine with internal choice between tea and coffee after coin insertion
- Other vending machine files (vmdtea, vmdalt, vmd) likely represent different implementations

## Expected puml-refinement Tool Results

For each test case, the tool should generate proof obligations and the analysis should align with the expected outcomes:

### Test Case 1: `abint [F= abext`
- **Command**: `puml-refinement -spec abext.puml -impl abint.puml`
- **Expected**: Should succeed - abext (external choice) is refined by abint (internal choice)
- **Reasoning**: Internal choice is more restrictive than external choice

### Test Case 2: `abint [F= a` 
- **Command**: `puml-refinement -spec abint.puml -impl a.puml`
- **Expected**: Should succeed - a simple `a` action refines the choice between `a` and `b`
- **Reasoning**: Implementation only offers `a`, which is acceptable when spec offers choice

### Test Case 3-5: Vending Machine Refinements
- **Commands**: Various vmi vs vmd* combinations
- **Expected**: Should succeed - specific implementations refine the general vmi interface

### Test Case 6: `not (abint [F= stop)`
- **Command**: `puml-refinement -spec abint.puml -impl stop.puml`
- **Expected**: Should fail - STOP offers no actions, cannot refine a process requiring actions
- **Reasoning**: STOP cannot satisfy the behavioral requirements of abint

### Test Case 7: `not (a [F= abint)`
- **Command**: `puml-refinement -spec a.puml -impl abint.puml`  
- **Expected**: Should fail - abint can choose `b`, but `a` spec cannot accept this
- **Reasoning**: Implementation offers more behaviors than specification allows

## Verification Strategy

Due to Go cache permission issues, direct tool execution may fail. However, the proof obligation generation should still work and produce:

1. **Trace inclusion obligations** - verifying implementation traces are acceptable
2. **Refusal set inclusion obligations** - verifying implementation refusals are acceptable  
3. **Guard weakening obligations** - if guards differ
4. **Postcondition strengthening obligations** - if postconditions differ

The analysis of these obligations should align with the expected refinement outcomes listed above.