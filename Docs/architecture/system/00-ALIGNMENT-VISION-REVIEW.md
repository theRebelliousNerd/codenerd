# Alignment and vision review: system

> Reviewed 2026-07-13 against `internal/system` and the cross-package safety
> path it constructs. The signed Superstar score is in [_progress.md](_progress.md).

## North-star alignment

| Question | Verdict | Evidence |
|---|---|---|
| Does the package keep the LLM creative rather than executive? | **VERIFIED CURRENT** | `internal/system/factory.go#initKernel` constructs Mangle kernel shards; `initPerceptionLayer` supplies LLM clients without granting them effect authority |
| Is constitutional control deterministic and default-deny? | **VERIFIED CURRENT** for wiring | `defaultKernelShardConfigs` places the complete authorization envelope in the policy shard; `cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard` proves exact matching |
| Does prompt behavior use the JIT path? | **VERIFIED CURRENT** | `factory.go#initIntelligenceLayer` requires the embedded corpus and constructs `JITPromptCompiler`; compilation uses private kernel scopes |
| Is construction separated from execution? | **VERIFIED CURRENT** | System composes `Cortex`; session, shards, VirtualStore, and tools perform runtime work |
| Is lifecycle ownership complete? | **VERIFIED CURRENT** for owned boot resources, **PARTIAL** for registry policy | Stage failures reuse idempotent aggregate cleanup; Close owns MCP, browser, closable embedding, JIT/DBs, shards, maintenance, and perception. Exact reverse acquisition order and caller-owned override policy remain untyped |
| Are package boundaries honest? | **PARTIAL** | Thin kernel, MCP, session, and trace adapters avoid cycles; session file read/write still bypass VirtualStore policy |
| Can operators explain a boot? | **PARTIAL** | Category logs exist, but no one correlated redacted stage/resource receipt exists |

## What improved in the reviewed tree

- **VERIFIED CURRENT:** policy owns `pending_action`, `permitted_action`,
  `permission_check_result`, and `permitted` together.
- **VERIFIED CURRENT:** default Cortex boot exposes a primary RealKernel to the
  VirtualStore, enabling fail-closed Dreamer simulation for destructive routes.
- **VERIFIED CURRENT:** prompt compilation uses a private RealKernel clone and
  does not leak selector facts on success, error, or cancellation.
- **VERIFIED CURRENT:** maintenance starts without an immediate database cycle,
  stores cancel/done ownership on Cortex, and Close stops it before LocalDB.
- **VERIFIED CURRENT:** effect routing preserves the executive action ID and
  uses exact target/payload authorization rather than `safe_action/1` as a grant.
- **VERIFIED CURRENT:** the normalized disabled-system-shard set participates in
  cache identity and boot behavior; equal sets reuse and different sets split.
- **VERIFIED CURRENT:** named boot-step failure routes through shared rollback,
  and repeated Close is idempotent for the tested lifecycle slice.
- **VERIFIED CURRENT:** production kernel shard configs consume the canonical
  predicate manifest; the policy envelope has one typed source of truth.

## Remaining anti-alignments

1. **Identity still omits engine/provider mode.** The disabled-shard defect is
   closed, but config engine can shape construction beyond provider/model.
2. **One adapter bypasses the executive.** Session file reads/writes use raw OS
   calls.
3. **Teardown is enumerated, not registered.** The fixed Close path owns MCP,
   browser, and closable embeddings, but exact acquisition order, ownership
   metadata, and cleanup receipts are not a typed registry.
4. **Chat uses direct boot.** This is a conscious integration seam, but it means
   cache identity and maintenance behavior are not universal.

## Non-claims

- System does not implement `permitted/3`; core policy does.
- A passing system suite does not prove all CLI, network, campaign, or race paths.
- A non-nil Cortex does not mean every optional integration is healthy.
- The package-local crash dump is not production Mangle source.
