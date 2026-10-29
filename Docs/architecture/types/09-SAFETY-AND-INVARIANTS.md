# 09 — Safety and Invariants: `internal/types`

> Last verified: **2026-08-15**

## 1. Package safety role

`types` is a **safety choke point** for fact encoding. It does not evaluate `permitted(...)` but it prevents garbage from becoming “truth” in the EDB.

Constitutional safety chain:

```
types.Fact (valid encoding)
  → kernel EDB
  → Mangle derives permitted(action) | deny-by-default
  → VirtualStore executes only permitted effects
```

Broken `ToAtom` short-circuits the entire chain with poisoned data.

## 2. Invariants

### I-FACT-01 — No nil args in ToAtom

`Fact.ToAtom` returns error if any arg is `nil`.  
Rationale: avoid string `"<nil>"` in EDB.

### I-FACT-02 — No pointer/address stringification

Unknown types must not fall through to `fmt.Sprintf("%v")` for store insertion. JSON best-effort only when marshal produces non-null non-empty non-`{}` output; else error.

### I-FACT-03 — Errors identify call site

Unsupported/nil args error messages include **predicate** and **arg index**.

### I-FACT-04 — Name constants are conservative

Strings resembling Unix paths or files are **not** name constants. Prefer explicit `MangleAtom`.

### I-FACT-05 — Bool mapping is stable

Go `true`/`false` map to Mangle true/false constants (`/true` `/false` in String form).

### I-FACT-06 — Fact.String output must re-parse as Mangle

Every branch of `Fact.String` has to emit a valid Mangle token, because the output is loaded back:
`northstar.RenderVisionMangle` writes it into a `.mg` file the kernel parses at boot, and
`world/scope.go` keys a dedup map on it. Containers therefore render as quoted JSON (matching
`ToAtom`), floats keep `%f` (mangle-go renders `Float64(2.0)` as `2`, which re-parses as `int64`),
and an unsupported type is quoted rather than emitted bare. Anchored by
`TestFactString_WhenAnySupportedArgType_ShouldParseBackAsMangle`.

### I-FACT-07 — Declared argument types are the authority

`internal/core/defaults/*.mg` `bound [...]` declarations decide an argument's Mangle type; the Go
call site conforms to them, not the reverse. `ToAtom` cannot enforce this (it never sees the Decl),
so the enforcement is the repo-wide ratchet in `fact_conventions_guard_test.go`, whose baseline
records every current violation and why it survives.

### I-TX-01 — Atomic commit or panic

`NewKernelTx` requires `KernelTransactor`. Non-atomic multi-op fallback is **removed**. Incomplete kernels must not silently multi-rebuild.

### I-TX-02 — Commit is the only apply point

Buffered ops on `KernelTransaction` apply only on `Commit()`.

### I-SHARD-01 — Permissions are declarative tags

`ShardPermission` does not grant OS rights by itself. Policy + VirtualStore enforce.

### I-SESSION-01 — Dream mode is advisory data

`DreamMode` is a flag; shards/executors must honor it. Types package does not intercept `Execute`.

### I-DEP-01 — No upward imports

`types` must not import implementer packages (cycle + safety boundary).

## 3. Concurrency invariants

| Object | Rule |
|--------|------|
| `Fact` | Value type; share by copy or treat immutable after build |
| `SessionContext` | No internal locks; snapshot semantics recommended |
| `KernelTx` | Single-goroutine use per transaction instance |
| Context keys | Session uses typed key; spawn keys are strings — avoid collisions |

## 4. Panic policy

| Location | Panic? | Why |
|----------|--------|-----|
| `NewKernelTx` | **Yes** if kernel not `KernelTransactor` | Fail closed on non-atomic multi-mutation |
| Extract helpers | **No** | Return zero / false / empty |
| `ToAtom` | **No** | Return error |
| `String` | **No** | Always emits a parseable token; worst case it quotes |

## 5. Mangle Decl relationship

This package does **not** declare Mangle predicates. When adding new fact shapes used as EDB:

1. Ensure `Decl` exists in `internal/core/defaults` / policy corpus
2. Ensure arity matches `Fact.Args`
3. Prefer atoms for enums (`/passing`, `/coder`) via `MangleAtom` for literals, or `types.Atom(v)`
   for a value assembled at runtime
4. The Decl-conformance ratchet will fail the build if a literal argument contradicts the Decl

## 6. Security-adjacent notes

- `VirtualStore.Exec` is effectful — interface presence does not authorize use
- Tool schemas in `ToolDefinition` are JSON Schema blobs; validation is consumer-side
- Grounding URL controls (`SetURLContextURLs`) carry provider limits (max 20 URLs, size) documented on interface

## 7. Safety test anchors

- `TestToAtom_WhenNilArg_ShouldReturnError`
- `TestToAtom_WhenUnknownType_ShouldReturnError`
- `TestToAtom_WhenContainerHoldsUnencodableValue_ShouldReturnNamedError`
- `TestFactString_WhenAnySupportedArgType_ShouldParseBackAsMangle`
- `TestFactConventions_WhenNewBareAssertOrPercentVAppears_ShouldFail` (repo-wide)
- `TestKernelTransactor_WhenKernelImplementationLacksTransaction_ShouldBeBaselined` (repo-wide)
- `TestNewKernelTx_WhenKernelCannotTransact_ShouldPanicNamingTheConcreteType`
- Name-constant negative cases (`//`, file paths, extensions)
- Extract bool atom conventions
