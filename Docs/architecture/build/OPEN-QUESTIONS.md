# OPEN QUESTIONS — `internal/build`

> Real open questions, not filler.  
> Last updated: **2026-07-13**

---

## Q1 — Is sandbox tool compile in-scope for sqlite/CGO auto-detect?

Autopoiesis passes temp/arena roots and forces `CGO_ENABLED=0`. Should `GetBuildEnv*` ever try to see the monorepo headers on those paths, or is the contract “tools are always pure-Go portable binaries”?

**Impact:** Detection root design (G-03), docs for tool authors.

---

## Q2 — Should `UserConfig` be mandatory for Cortex-spawned compiles?

Today `nil` is first-class. Making config required would surface Build.EnvVars but break simple unit tests and isolated forges.

**Options:** (a) keep nil-safe; (b) require config in production constructors only; (c) inject a `BuildEnvProvider` interface at autopoiesis construction.

---

## Q3 — Who owns applying `GoFlags`?

Env-only package cannot set argv. Should a sibling helper live here (`AppendGoFlags(args, cfg)`) or in callers?

---

## Q4 — What is the adoption boundary with tactile?

Tactile builds env for general sandboxed processes. Build specializes for Go. Overlap invites drift.

**Question:** Must tactile optionally call into `internal/build` when binary is `go`, or is dual-stack permanent?

---

## Q5 — Should default sample config keep a machine-specific CGO path?

`user_config.go` sample embeds `C:/CodeProjects/codeNERD/sqlite_headers`. Auto-detect is more portable. Keep sample as documentation of shape only?

---

## Q6 — Is `GetBuildEnvForTest` a permanent API?

Empty specialization creates false confidence. Delete vs implement race/CI policy needs a product decision (speed vs safety defaults).

---

## Q7 — Observability depth for security audits?

Should final env keys be glass-box visible when the agent compiles code? Privacy vs forensics tradeoff.

---

## Q8 — Multi-module workspaces

If workspace is a monorepo with nested modules, is detection root always repo root, or nearest `go.mod` + nearest `sqlite_headers`? Current code only checks the provided root.

---

## Resolved / closed (keep for history)

| ID | Resolution |
|----|------------|
| — | *None closed in this rebuild cycle* |
