# TODO — `internal/logging`

> Last verified: **2026-07-13**  
> Backlog only; **no code changes in this corpus revision**.

## P1

- [ ] Align config schema: `json_format` bool vs `config.LoggingConfig.Format` string — pick one and document/load both  
- [ ] Document (and optionally implement) loading from the same file the rest of the app treats as source of truth  
- [ ] LLM I/O redaction hooks for common secret patterns  

## P2

- [ ] Make `CloseAll` close audit + LLM I/O (or document and wire CLI/chat to call all three)  
- [ ] Resolve `sync.Once` + `--workspace` first-init race (boot order or rebind design)  
- [ ] Add enabled-path tests for `trace_llm_io` writing expected markers  

## P3

- [ ] ContextLogger / RequestLogger respect `json_format` via structured entries  
- [ ] Operator playbook snippet in root AGENTS or help command pointing at this corpus  
- [ ] Optional CLI offline: audit JSONL → `.mg` facts file  
- [ ] Size/time-based log rotation beyond daily name  

## P4

- [ ] Northstar convenience wrappers (Info/Debug/Warn/Error)  
- [ ] Optional `runtime.Caller` population of StructuredLogEntry file/line  
- [ ] Expand call-site audit for `SafetyCheck` next to real `permitted` checks  

## Done (already in code)

- [x] Categorized file logging with debug master switch  
- [x] Audit JSON + Mangle fact strings  
- [x] LLM I/O full dump opt-in  
- [x] Timers + performance sampling/thresholds  
- [x] Dense unit test suite including concurrency  
- [x] Idempotent Initialize via sync.Once  
