./core/testdata
===============

Processes
---------

| Process Name | File Name     |
|:-------------|:--------------|
| `a`          | `a.puml`      |
| `abint`      | `abint.puml`  |
| `abext`      | `abext.puml`  |
| `vmi`        | `vmi.puml`    |
| `vmdtea`     | `vmdtea.puml` |
| `vmdalt`     | `vmdalt.puml` |
| `vmd`        | `vmd.puml`    |
| `stop`       | `stop.puml`   |


Refinement Relationships
------------------------
Trivially, `x [F= x` (reflexivity) holds for all processes `x`.

```cspm
abint [F= abext
abint [F= a
vmi [F= vmdtea
vmi [F= vmdalt
vmi [F= vmd

not (abint [F= stop)
not (abint [F= a)
```