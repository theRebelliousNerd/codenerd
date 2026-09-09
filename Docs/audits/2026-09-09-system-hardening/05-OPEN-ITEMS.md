# Open items — found, verified, deliberately not fixed

Date: 2026-09-09
Companion to [00-FINDINGS.md](00-FINDINGS.md).

Each item below was verified with file:line evidence during the hardening pass
and left alone on purpose. Every one states why, so the next person does not
re-derive the analysis.

---

## O1 — Per-shard permissions are declared and enforced by nothing

**Severity: medium. Defence-in-depth that is not in depth.**

Ten shard profiles declare a `Permissions` list
(`internal/core/shards/config.go:15,34,53,72`,
`internal/shards/registration.go:624,674,692,711,728`,
`internal/shards/requirements_interrogator.go:29`), drawn from a vocabulary that
maps directly onto tool names: `read_file`, `write_file`, `exec_cmd`, `network`,
`browser`, `code_graph`, `ask_user`, `research`
(`internal/types/shard.go:36-43`).

The only reader is `BaseShardAgent.HasPermission`
(`internal/core/shards/agents.go:92`), which has **no non-test caller**. The
values never reach the kernel either — no `.mg` file mentions `ShardPermission`
or any equivalent predicate.

There is a second, also-unwired mechanism: `stage_shard_tool_allowed(ShardID,
ToolName)` (`stage_context.mg:59,285,294`) derives a per-shard tool allow-list
in Mangle and has **no Go consumer** — only `internal/core/stage_context_test.go`
queries it.

So a researcher shard that declares it may not write files is not prevented from
writing files by either mechanism. The actual gates are the constitution's
`permitted(ActionType, Target, Payload)` — which takes **no actor argument**
(`schemas_safety.mg:15`), so it cannot distinguish shards by construction — and
the session's `AllowedTools`.

**Why not fixed here.** Wiring either mechanism is a behaviour change that would
begin denying tools shards currently use, and validating that needs interactive
runs against real workloads, not a test suite. Choosing between them is also an
architectural decision, not a wiring repair.

**Recommendation.** Prefer the Mangle path: assert `shard_permission(ShardID,
Permission)` at spawn, join it into `stage_shard_tool_allowed`, and have the
session intersect that with `AllowedTools`. That keeps enforcement in the
executive rather than in a Go helper, which is the project's stated inversion of
control. Ship it behind a feature flag and log denials for a release before
enforcing.

## O2 — `atom_context_boost` has no producer, so half of atom scoring is inert

`jit_logic.mg:90-92` derives `atom_matches_context(AtomID, FinalScore)` from
`prompt_atom` joined with `atom_context_boost`, described in the schema as a
virtual predicate ("Go-computed boost", `schemas_prompts.mg:448`).

`RegisterVirtualPredicate` (`internal/mangle/differential.go:847`) has **no call
site anywhere in the repo**. The virtual-predicate mechanism is entirely unused,
so nothing produces `atom_context_boost` and the non-mandatory branch of
`atom_matches_context` never fires in production. The mandatory branch
(`:94-95`, score 100 for `is_mandatory` atoms) does.

The thirteen `atom_has_*_match` rules that fed the same area **were** removed in
this pass — they were dead at both ends (see F12). This one is different: it has
a live consumer chain and a missing producer, so removing it would delete
working logic, and implementing it means deciding what the boost should be.

**Recommendation.** Either implement the boost as a real Go-side scorer and
register it, or replace the virtual predicate with ordinary derived rules over
facts the selector already emits (`atom_priority`, `atom_tag`,
`current_context`). Do not leave it as a third unused mechanism.

## O3 — `internal/context` never reaches three of four entry points

The spreading-activation and semantic-compression subsystem
(`internal/context/`, ~5.3k non-test lines) is imported **only** by
`cmd/nerd/chat/**` and `cmd/nerd/cmd_context_stats.go`. `internal/session` does
not import it at all.

So every `nerd <verb>` CLI run, every spawned subagent and every campaign shard
gets no compression — their only history control is the fixed 6-message /
24,000-character window at `internal/session/executor.go:1688`. Only the
interactive TUI benefits.

**Why not fixed here.** Threading a compressor through the executor is a
substantial change to the session lifecycle, not a wiring gap, and the two paths
have genuinely different lifetimes: a TUI session is long-lived and a CLI verb
is not. It may be correct as designed. It is not *documented* as designed, which
is the actual defect.

Several exported `ActivationEngine` methods have no non-test callers as a
consequence: `ScoreFacts`, `SelectWithinBudget`, `SpreadFromSeeds`,
`SetCorpusPriorities`, `AddDependency` (`internal/context/activation.go:266,
411, 573, 229, 493`).

## O4 — Dead exported surface not worth touching yet

Verified zero non-test callers, left in place because each is the visible half of
a capability whose other half is also unwired — the repo's `audit-before-delete`
rule says find the gap before deleting, and the gap is a design decision in each
case:

| Symbol | File | Note |
|---|---|---|
| `SelfHealer` (whole type) | `internal/core/self_healing.go:71` | `internal/session/build_verify.go:35-38` documents that it supersedes this. Two of its strategies are also stubs. Wire or delete — a maintainer's call. |
| `LimitedExecutorInterface`, `SandboxedExecutorInterface`, `CompositeExecutorInterface` | `internal/tactile/executor_interface.go:31,39,50` | Whole file unreferenced. |
| `NewPersistentDockerExecutor`, `DefaultContainerPoolConfig` | `internal/tactile/persistent_docker.go:92,149` | Container pool never constructed. |
| `GetUserJourneyState`, `RecordSessionStart`, `CheckJourneyTransition`, `GetExperienceLevelFromPreferences`, `GetDisclosureLevel` | `internal/ux/migration.go:211,220,238,262`, `user_state.go:74` | Progressive disclosure: journey state is never recorded and disclosure level never computed. `ShouldShowOnboarding` in the same package **is** live. |
| `SetGlobalAllowlist`, `SetGlobalFactSink` | `internal/tools/registry.go:594,630` | Instance-level equivalents are wired at `internal/core/virtual_store_tools.go:65,69`. |
| `ReconcileEmbeddedCorpus` | `internal/prompt/reconciler.go:301` | Sibling `ReconcilePromptCorpus` is live. |

## O5 — Repo-wide `gofmt` drift

`gofmt -l internal cmd` reports **55 files** unformatted at the branch point, all
pre-existing. Files touched by this pass were formatted; the rest were left
alone so the diff stays readable.

**Recommendation.** One mechanical `gofmt -w` commit on its own, then a CI gate.
Mixing it into feature work is what let it reach 55 files.

## O6 — Current-turn user input is uncapped into generation

Classification caps user input at 50,000 characters; the generation path does
not. This is the one remaining uncapped path into a live prompt.

Left alone deliberately: truncating the user's actual request is a semantic
decision, not a hygiene one, and the failure mode is a clean provider 413 rather
than the model reasoning on a silently shortened request. It is bounded on the
replay side now, so it can affect at most one turn.

**Recommendation.** A configurable cap with a visible CLI warning, so the user
learns their input was shortened rather than the model quietly working from
half of it.
