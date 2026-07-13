# Spec Attribution Format

Use one concise attribution near the primary symbol implementing a corpus
contract:

```go
// SPEC: Docs/architecture/prompt/08-JIT-PROMPT-AND-AGENT-BEHAVIOR.md#selection-contract
func SelectAtoms(...) ... {
```

Rules:

- use a repo-relative path and real heading anchor
- cite the nearest owning section
- stack multiple SPEC lines when genuinely necessary
- do not attribute tests or helpers indiscriminately
- verify the referenced file and section exist
- treat attribution as traceability, not proof of correctness
