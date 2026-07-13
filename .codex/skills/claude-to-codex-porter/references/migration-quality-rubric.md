# Migration Quality Rubric

Score every dimension from 1 to 5. A passing port has no dimension below 3 and
an average of at least 4.

| Dimension | 1 | 3 | 5 |
|---|---|---|---|
| Surface completeness | Major source systems omitted | All requested systems classified | Full support closure with runtime and root-system wiring accounted |
| Behavioral fidelity | Target merely resembles source | Core behavior survives with justified adaptations | Activation, boundaries, failure modes, and output contracts retain parity |
| Runtime activation | Files only parse | Primary targets are discoverable | Every skill, agent, hook, rule, and plugin target has activation evidence |
| Safety and ownership | Secrets, permissions, or roots changed casually | No unsafe incidental changes | Trust, secrets, permissions, source mutability, and human checkpoints are explicit |
| Validation quality | Read-back only | Surface-specific syntax checks pass | Syntax, behavior, collision, trust, and fresh-session checks all pass |
| Evidence honesty | Gaps hidden or history rewritten | Gaps listed | Every residual difference is classified and preserved history stays literal |

Record the six scores and short evidence in the migration report or dated
journal entry.
