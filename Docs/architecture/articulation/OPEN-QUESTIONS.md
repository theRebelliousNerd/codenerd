# OPEN QUESTIONS — articulation

> Last verified: 2026-07-13  
> Real open design questions, not rhetorical filler.

## Q1 — Should fallback ever attempt “repair” LLM calls?

Today fallback is local (surface salvage / plain text). Should the package or callers automatically re-prompt with schema after a failed parse, or does that risk cost loops?

## Q2 — Is `context_feedback` required every turn?

Schema lists it as required in the full JSON Schema; Go type is optional pointer; protocol suffix says emit empty values. Which contract is normative for models and for scoring pipelines?

## Q3 — Memory operations authority

Who is the single owner for `promote_to_long_term` / `store_vector` / `forget` / `note`? Session logs today; is store/learning the target, and should articulation validate `op` enums beyond schema?

## Q4 — Strict mode in production?

`RequireValidJSON` enables unknown-field rejection and required intent fields. Should interactive chat ever enable this when schema-capable providers are selected, or is tolerance permanently preferred?

## Q5 — StreamParser vs full dual-stream control

Should control fields ever stream (e.g. progressive mangle_updates), or is post-hoc full-buffer parse permanent architecture?

## Q6 — Instance ID vs stable ShardID

`toCompilationContext` strips `-<digits>` suffixes for stable agent IDs. Are all spawner ID formats covered, or do non-numeric suffixes break atom tag matching?

## Q7 — Metrics productization

Should `ProcessorStats` become first-class glass-box / session metrics, or is log-derived SRE enough?

## Q8 — Relationship to session “no shards” architecture

Session package comments describe a clean executor without shards, while articulation still ships shard fallback templates and shard-type budgets. Long-term, does PromptAssembler become pure CompilationContext bridge with zero shard-named templates?

## Q9 — Tool request vs native function calling coexistence

When a provider supports both native tools and Piggyback `tool_requests`, which wins on conflict, and should articulation detect dual emission?

## Q10 — Package README version stamp

Package README still says “Architecture Version 2.0.0 (December 2024)”. Should versioning move solely to this architecture corpus with last-verified dates?
