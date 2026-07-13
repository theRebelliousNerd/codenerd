# prompt — Prompt / JIT Surface

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/prompt/` (25 non-test .go, 32 tests, 0 .mg)**


## Anchors

- Compiler: `internal/prompt/compiler.go`
- Atoms: `internal/prompt/atoms/`
- Assembler bridge: `internal/articulation/prompt_assembler.go`

## Discipline

New LLM-facing behavior → prompt atoms first, not ad-hoc shard prose.
