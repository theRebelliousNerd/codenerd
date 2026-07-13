# scripts/ — intentionally no `build_tag_index.py` here

The corpus context index builder (`33_corpus_context_index.json` regeneration)
is centrally owned at:

```
.codex/skills/corpus-build/scripts/build_tag_index.py
```

`corpus-doc-auditor` **consumes** that script — it does not own, duplicate, or
recreate it. This directory intentionally has no scripts of its own for that
reason; the doc-auditor's only agent-runnable tooling is the central script,
invoked directly:

```
python .codex/skills/corpus-build/scripts/build_tag_index.py
python .codex/skills/corpus-build/scripts/build_tag_index.py --check
```

If that script does not yet exist when you are dispatched, do not author a
substitute in this directory — surface the gap to the orchestrator instead.
Recreating it here would fork a machine-owned artifact into two divergent
copies, which is exactly the failure mode the "centrally owned" boundary
exists to prevent.
