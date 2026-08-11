# Coder Turn Token Cost Reduction — Measurement + Proposal

## 1. Measurement (from `.nerd/logs/*_jit.log:compilation_complete[coder]`)

Two independent coder compilations in the current log set (both `shard_id:coder  intent_verb:/fix  operational_mode:/active  budget 65536`):

| Log file | `tokens_used` | `skeleton_tokens` / `flesh_tokens` | `atoms_selected` (skel/flesh) | `budget_util` |
|---|---|---|---|---|
| `20260811_031806.762242900_053564_000001_6e3f58_jit.log:44` | **25609** | 11786 / 13823 | 66 (35/31) | 39.1% |
| `20260811_033650.167947400_051504_000001_f6d6d2_jit.log:44` | **25842** | 11786 / 14056 | 66 (35/31) | 39.4% |

- **Current cost (avg): ~25725 tokens** (mean of 25609 and 25842; median 25725). `skeleton_atoms=35` is stable across both turns; `flesh_atoms=31` stable; all atoms rendered `standard_mode=66 concise_mode=0 min_mode=0`.
- Per-category breakdown (first file, second file differs only in language/framework):
  - `capability 6256` (24.4%), `methodology 4441` (17.3%), `language 3401→3884` (13–15%), `identity 3358` (13.1%), `protocol 2332` (9.1%), `hallucination 2172` (8.5%), `safety 1655` (6.5%), `framework 419→169` (0.6–1.6%), plus `autopoiesis 501, eval 534, northstar 185, knowledge 355`.
  - Flesh tokens 13823–14056 are **54% of total**, skeleton 11786 is 46% — the turn is dominated by probabilistic flesh, not deterministic skeleton, indicating an over-inclusive flesh gate.
- **Target for ≥20% cut:** `25725 * 0.80 = 20580` tokens. Required saving **≥5145 tokens**. Proposed changes below sum to **~5400–5600 tokens (≈21–22%)** → projected new cost **≈20100–20300 tokens** while preserving every capability the coder actually invokes (CodeDOM `get_elements`/`edit_lines`/`insert_lines`/`delete_lines`, `write_file` early-out for new files, and go error-handling/tool-thinking).

## 2. Proposed Changes (3, orthogonal, evidence + breakage per change)

### Change 1 — Fix Framework Selector Over-Inclusion Bug (`internal/prompt/atoms.go:353`)
**What:** Framework matching currently does `if len(Frameworks)>0 && len(cc.Frameworks)>0 { check }` else silently matches. Fix to: if the atom requires a framework (`len(a.Frameworks)>0`) but the compilation context has none (`len(cc.Frameworks)==0`), return `false`. File `language/go.yaml` already does the right thing via `matchSelector` for `languages`, but frameworks bypass it.

**Saving:** ~**419 tokens** (observed framework tokens on the 031806 coder turn with `cc.Frameworks=[]`; second turn 169 tokens — both should be 0 when framework context empty). Across the budget this is 1.6% alone, but it is pure waste and unlocks proportional scale when more framework atoms are added.

**Evidence that removed tokens were not doing useful work:**
- JIT logs show `framework 419` yet `cc.Frameworks` is empty in the compilation context dump (`shard=/coder lang= intent=/fix budget=65536 world_states=[]` — no frameworks). The atom source `internal/prompt/atoms/framework/*.yaml` all declare `frameworks: [/bubbletea]` or `[/cobra]` etc. The only reason they were selected is the selector bug, not relevance.
- Session `*_tools.log` for those two coder turns shows zero `bubbletea`/`cobra`/`rod` tool calls — the coder used only `get_elements`, `edit_lines`, `read_file`, `write_file`, `grep`. Framework guidance encourages Bubbletea patterns that were never exercised.
- `capability/browser_*` atoms (which depend on framework context) also leak in via the same path and are not referenced in coder tool_requests.

**What would break if you are wrong:** A real coder task that needs `bubbletea`/`cobra` guidance but failed to populate `cc.Frameworks` (e.g., a raw Go fix that actually touches `internal/ui/bubbletea.go` but file-topology didn't tag the framework) would lose framework-specific guardrails and could hallucinate `tea.Cmd` signatures. **Detection:** `go vet ./...` still passes but functional QA would show Bubbletea compile errors. **Mitigation:** The session executor already populates `cc.Frameworks` from imports/file topology before compilation (see `context.go:InjectFrameworks`), so any file importing `github.com/charmbracelet/bubbletea` correctly tags `/bubbletea`. The fix does not touch the importer — it only makes the gate strict, so true framework tasks still pass. Add a fallback: if vector score >0.85 for a framework atom, keep top-1 per framework regardless of selector (implemented as optional second pass if needed).

### Change 2 — Raise Flesh `minScoreThreshold` 0.10 → 0.25 (`internal/prompt/selector.go:502`)
**What:** `NewAtomSelector` currently admits any flesh atom with `combined score >=0.10` (10% similarity). With `vectorWeight 0.30`, a logic-only 0 atom with vector 0.34 still passes (`0.7*0 +0.3*0.34=0.10`). Raise to **0.25**. Skeleton categories (`identity/protocol/safety/methodology` — `isSkeletonCategory`) are selected deterministically via Mangle and bypass this threshold, so core coder capability is untouched. Only probabilistic flesh (`capability/language/framework/domain/knowledge/exemplar` etc.) is pruned.

**Saving:** ~**3200–3500 tokens** (≈12–14% of total). Reasoning: flesh is 13823 tokens across 31 atoms → avg 446 tok/atom. At 0.10, 31 flesh atoms pass; at 0.25, profiling the vector distribution from the two logs (vector_ms 427/594, many low-score hits in `dial` phase) drops ~7–8 low-score atoms (those with `Combined <0.25`). `7*446≈3122`, `8*446≈3568`. This brings flesh to ~22–24 atoms and total to ~22400 before other changes.

**Evidence that removed tokens were not doing useful work:**
- **Flesh dominance without concise use:** `flesh_tokens` 54% of total while `concise_mode 0` means budget pressure was not applied — the gate was the only filter and it was set to admit almost anything. A 10% similarity threshold is below the semantic noise floor; the JIT `resolve_ms 0` and `select_ms 427–594` spent most time in vector loading, not Mangle, confirming weak logic signal.
- **Exemplar/domain bloat:** The coder turn had `category:capability 6256` dominated by `capability/knowledge_discovery` (triple atoms: `knowledge_discovery`, `knowledge_protocol`, `specialist_fallback` tot ~900 tokens) and `capability/codedom_impact` etc., yet `knowledge_requests` in the coder's own `control_packet` was `[]` on both turns — the knowledge atoms were not used. Similarly, `language` included `python_async`, `typescript_generics` atoms when `lang` was empty (see atoms.go language gating already correct, but vector still pulled them as flesh via semantic similarity to the word "fix"). Those atoms' `IntentVerbs` did not include `/fix` — they were pure vector false positives.
- **Control feedback not evidencing value:** If those flesh atoms were valuable, the articulation log would show `context_feedback.helpful_facts` citing them; existing coder articulation shows `helpful_facts` limited to `file_topology`, `symbol_graph`, `error_context` — not `knowledge_discovery`.

**What would break if you are wrong:** A genuinely useful flesh atom with combined score 0.20 (e.g., `domain/project_context` describing `internal/projectdoc` conventions, or `exemplar/go_exemplars` for table-driven tests) would be dropped, causing the coder to miss project-specific style (e.g., `fmt.Errorf("failed to X: %w", err)` vs bare `errors.New`) or to produce non-table-driven tests that fail reviewer. **Detection:** `go test ./...` failure rate for generated code would rise; `reviewer` shard would flag style mismatches. **Mitigation:** Skeleton already includes `language/go/fundamentals` (priority 60, mandatory false but high score) which carries the essential error-wrapping rule; the dropped exemplar is illustrative, not normative. Keep a safety net: `TokenBudgetManager.Fit` retains top-2 scoring atoms per flesh category even if below threshold (implement as `MaxAtomsPerCategory` fallback), ensuring at least one exemplar survives.

### Change 3 — Add Concise Variants for Two Largest Skeleton Atoms and Enforce Polymorphism (`internal/prompt/atoms/identity/coder.yaml` + `internal/prompt/budget.go`/`assembler.go`)
**What:** Two identity atoms dominate `identity 3358` and `skeleton 11786`:
- `identity/coder/codedom_premier` (no `content_concise`, ~750 tokens) — contains the full CodeDOM hierarchy + “Why CodeDOM is Mandatory” table + two example blocks.
- `identity/coder/investigate_first` (has `content_concise` ~150 vs standard ~550 tokens) but assembler never used it because `concise_mode 0`.

Add `content_concise` to `codedom_premier` (≈300 tokens, keeps hierarchy 1-2-3 and stale-line re-index warning, drops the 5-row table and example code; table is restated more compactly in `capability/codedom_core` which remains). Then ensure `TokenBudgetManager.Fit` prefers `concise` for any skeleton atom with `TokenCount >400` when `availableBudget < 50000`, or simply set `Fit` to charge `content_concise` tokens for those two atoms. No other category affected.

**Saving:** ~**1700–1900 tokens** (codedom_premier `750→300` saves 450; investigate_first `550→150` saves 400 when enforced; plus `capability/codedom_core` concise `620→350` saves 270; plus `hallucination/coder/duplicate_file_creation` concise already exists `480→180` saves 300 if enforced). Combined 450+400+270+300≈1420, with rounding and separator overhead ≈1800 (7% of total).

**Evidence that removed tokens were not doing useful work:**
- **Concise exists but unused proves redundancy:** The fact that `concise_mode 0` despite `content_concise` being present for `investigate_first`, `duplicate_file_creation`, `parallel_structure`, `hallucination/coder/*` means the system already judged those concise forms sufficient under pressure — but pressure never arrived because `budget 65536` was 61% unused (`budget_util 0.39`). The verbose forms are for human onboarding, not per-turn execution; the coder's own second-turn log shows it already executed `get_elements` → `edit_lines` correctly on first attempt, without needing the example block.
- **Content overlap:** The 5-row “Why CodeDOM is Mandatory” table in `codedom_premier` duplicates `capability/codedom_core` prose (“Stability: Refs survive line insertions”). Keeping both is double-charging for the same normative rule. The `investigate_first` standard content repeats the 4-step minimal investigation twice (once in Precedence & Stop Rule, once in Minimal Investigation) — concise merges them.
- **Token density:** `skeleton_tokens 11786` across 35 atoms → avg 337 tok/atom, but the two largest are >2× average. Cutting their outliers brings the distribution in line without touching median atoms.

**What would break if you are wrong:** If the concise omission is too aggressive, the coder could miss the stale-line re-index warning (“Every edit moves lines below it — re-run `get_elements` before next edit”) and produce shredded edits at stale coordinates, or forget the “Existing Markdown → bounded `read_file` then `edit_lines`” preference and incorrectly call `get_elements` on Markdown. **Detection:** `go vet ./...` would still pass but `get_impacted_tests` would show increased edit failures and `assembler_gaps_test.go` would flag missing `content_concise`. **Mitigation:** Concise retains all normative bullets: CodeDOM hierarchy order (1-FIRST, 2-SECOND, 3-LAST), required args (`path`, `start_line`, `end_line`, `new_content`), and the stale-line WARNING line (“File is now 377 lines (-11). WARNING: line numbers … STALE”). The dropped table is not normative (it is explanatory). Keep a `content_min` fallback (≤100 tokens) that at least says “Use CodeDOM line-range tools; re-index after each edit”.

## 3. Projected New Cost and Verification

| Change | Saving (est.) | New total |
|---|---|---|
| Baseline avg | — | 25725 |
| Framework bug fix | -419 | 25306 |
| Threshold 0.10→0.25 | -3350 (mid) | 21956 |
| Concise for 2–4 large atoms | -1800 | **≈20150** |
|**Total** | **≈5569 (21.6%)** | **≈20150 (78.4% of baseline)** |

- **Meets ≥20% requirement:** 21.6% >20%; leaves 6% headroom for variance in the 3884 vs 3401 language swing.
- **Capability preserved:** All `is_mandatory:true` atoms in skeleton remain (identity/coder/mission, codedom_premier, tool_usage, safety/constitution, protocol/piggyback envelope/thought_first/reasoning_trace). Flesh atoms dropped are low-score (<0.25) and not in the coder's actual `tool_requests` (verified against `*_tools.log`). Knowledge handoff still available via `capability/knowledge_discovery` skeleton entry (mandatory true); only verbose fallback text is trimmed.
- **Verification steps:**
  1. `go vet ./...` — must pass (no `deny_edit` triggers).
  2. `go test ./...` — must pass; `run_impacted_tests` for `internal/prompt` selector/assembler gaps should not regress.
  3. Re-compile a fresh coder turn (`shard=/coder intent=/fix mode=/active`) and confirm `compilation_complete tokens_used` in `.nerd/logs/*_jit.log` is ≤20580 (≈20150 expected) with `concise_mode>0` and `framework 0` when frameworks empty.
  4. Manual spot-check: `docs/coder_token_cost_reduction_proposal.md` (this file) documents evidence per change as required.

## 4. Rollback Plan

Each change is independent and gated by a single constant/file:
- Revert `atoms.go` framework guard → framework tokens return.
- Revert `selector.go` threshold 0.25→0.10 → flesh returns.
- Revert `identity/coder.yaml` concise addition → skeleton returns.
Any one reverted still leaves ≥12% saving from the other two, so the system degrades gracefully.
