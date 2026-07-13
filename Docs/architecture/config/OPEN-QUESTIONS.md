# config — open decisions

> Questions only; feature work is authoritative in [TODO.md](TODO.md).

## Q1 — Environment policy

**OPEN QUESTION.** Should an environment key override a valid explicit file key,
or only fill an omitted secret reference? The choice affects 12-factor operation,
provider honesty, and provenance. Validation must still distinguish absent file
from malformed present file.

## Q2 — Legacy YAML retirement

**OPEN QUESTION.** Is `.nerd/config.yaml` still an operator contract, or can the
Cobra timeout move to the JSON snapshot? Current evidence is
`cmd/nerd/main.go#rootCmd`; removal needs compatibility telemetry or a pinned
migration window.

## Q3 — Resource default policy

**OPEN QUESTION.** Should shard/world/API defaults remain fixed and portable, or
derive from a typed host profile? JSON and YAML currently disagree. Dynamic
defaults need upper bounds and provenance, not implicit hardware guessing.

## Q4 — Reload scope

**OPEN QUESTION.** Is codeNERD one workspace per process, or must a process
switch workspaces? The answer determines whether features, logging, LLM timeouts,
scheduler, clients and stores can remain process-global.

## Q5 — Secret source

**OPEN QUESTION.** Should config store secret values, environment/keyring
references, or both? A reference model improves persistence safety but must work
non-interactively and across Windows/macOS/Linux.

## Q6 — Compatibility diagnostics

**OPEN QUESTION.** How long may the migration decoder accept deprecated fields,
and which warnings fail CI? The decision must pin schema versions, removal dates,
and a dry-run path.
