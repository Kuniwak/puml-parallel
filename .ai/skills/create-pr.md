---
name: create-pr
---

Create a Pull Request in draft state. Use one of the following templates depending on the nature of the PR. If the commits have not been pushed yet, push them first.

# Feature addition

Treat the change as a feature addition when new normal-case behavior is defined for an input that would previously have been an error case or implementation-dependent behavior.

```markdown
# Requirement

...

# Specification

...

# Notes, including inferred assumptions

...
```

# Fix

Treat the change as a fix when the implementation previously produced an output different from the specification for an input defined by the specification, meaning the implementation was defective, and the work makes it conform to the specified behavior.

```markdown
# Problem, such as errors

...

# Cause and evidence

...

# Fix options considered

...

# Selected fix and rationale

...

# Notes, including inferred reasoning

...
```
