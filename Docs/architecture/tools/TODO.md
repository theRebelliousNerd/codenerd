# tools — TODO

> Last verified: **2026-07-13**  
> Prioritized backlog for `internal/tools` and tightly coupled contracts.

## P0 — Safety

- [ ] Apply `resolveWorkspacePath` (or equivalent) to `glob`, `grep`, `search_code` base_path/path.  
- [ ] Apply workspace containment to codedom path args (elements + line tools).  
- [ ] Contain shell/git `working_dir` to workspace root.  
- [ ] Contract with session: empty `AllowedTools` should not mean “all tools” when safety gate is on (document + implement).

## P1 — Catalog & correctness

- [ ] Add `git_diff`, `git_log`, `git_operation` to `modular_tool_allowed` in `intent_routing.mg` (or explicitly document intentional omit).  
- [ ] Add `research_cache_clear`, `research_cache_stats` to Mangle routing if they should be agent-callable.  
- [ ] Use `coerceInt` for search tool integer args (`max_results`, `context_lines`).  
- [ ] Thread workspace root through tool context; reduce env-only coupling (TODO already in `workspace_guard.go`).  
- [ ] Golden test: RegisterAll names ⊆ Mangle modular_tool_allowed ∪ intentional_exceptions.

## P2 — Hygiene

- [ ] Rewrite `codedom/doc.go` to match registered tools.  
- [ ] Decide CategoryReview / CategoryAttack: implement tools or stop mapping intents to empty categories.  
- [ ] Prefer `logging.Tools*` for file/shell completions instead of VirtualStore channel (optional consistency).  
- [ ] Consider registering tools only once into Global from VS pointer to eliminate dual-map drift risk.

## P3 — Product depth

- [ ] Optional disk-backed research cache under `.nerd/`.  
- [ ] Assert `tool_execution` facts from Registry.Execute for learning.  
- [ ] Improve codedom EndLine via simple brace/indent block tracking.  
- [ ] Metrics counters for tool success/duration.

## Done / not TODO

- Modular registry + RegisterAll hydrate — **done**.  
- Workspace guard for core file ops — **done**.  
- shellquote for run_command — **done**.  
- Grounding/Thinking helpers — **done** as libraries.
