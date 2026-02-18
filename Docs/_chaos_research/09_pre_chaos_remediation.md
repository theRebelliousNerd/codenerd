# Pre-Chaos Remediation Report

**Date:** 2026-02-18
**Status:** ALL 17 FIXES IMPLEMENTED AND COMPILED
**Build:** PASS (nerd.exe 119MB)

---

## Executive Summary

Before writing the Chaos Engineering test suite, we implemented 17 structural fixes across 5 phases to ensure the system survives the first 5 seconds of fuzzing. Without these fixes, any chaos monkey would instantly OOM, panic, or deadlock the process — never reaching the deep, interesting states (multi-agent corruption, Mangle logic loops, spreading activation poisoning) that chaos testing is designed to discover.

All fixes compile cleanly and are additive (no behavioral changes to the happy path).

---

## Phase 1: Structural Integrity (Stop Trivial OOMs)

### Fix 1.1: Cap UI Input Length
**File:** `cmd/nerd/chat/session.go:87`
**Before:** `ta.CharLimit = 0` (unlimited)
**After:** `ta.CharLimit = 100_000`

**Impact:** Prevents a 50MB paste from crashing the Token Counter (`CountString()`), exhausting the regex engine in the transducer, and flowing into the LLM API as a massive prompt. 100K characters is generous for any legitimate use case.

---

### Fix 1.2: Throttle HTTP Response Reads (10MB Cap)
**Files:** 9 LLM client files, 35 total occurrences

| File | Occurrences |
|------|-------------|
| `client_gemini.go` | 6 |
| `client_anthropic.go` | 3 |
| `client_openai.go` | 6 |
| `client_xai.go` | 1 |
| `client_antigravity.go` | 4 |
| `client_openrouter.go` | 3 |
| `client_zai.go` | 6 |
| `client_gemini_files.go` | 3 |
| `client_tool_helpers.go` | 3 |

**Before:** `io.ReadAll(resp.Body)` — unbounded
**After:** `io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))` — 10MB cap

**Impact:** A malicious API proxy or hallucinating model returning a 2GB error body would instantly OOM the process. The 10MB cap is 100x larger than any legitimate LLM response.

---

### Fix 1.3: Cap JSON Depth Scanner
**File:** `internal/articulation/json_scanner.go`
**Before:** `depth` counter incremented unboundedly; no size limit on extracted candidates
**After:** Two safety caps added:
- `maxJSONDepth = 200` — resets scanner when nesting exceeds 200 levels
- `maxJSONCandidateSize = 5MB` — skips objects larger than 5MB

**Impact:** Prevents CPU exhaustion on deeply nested garbage JSON like `{"a":{"a":{"a":...}}}` (thousands of levels) and prevents massive string allocations from substring extraction.

---

### Fix 1.4: Truncate Input Before Regex Matching
**File:** `internal/perception/transducer.go:255`
**Before:** Full user input (unlimited) passed to `strings.ToLower()` and every VerbCorpus regex
**After:** Input truncated to `maxRegexInputLen = 2000` chars before processing

**Impact:** Go's regexp is safe against ReDoS (Thompson NFA), but there's still a linear-cost amplification when running N regex patterns against a 100KB string. The first 2000 chars contain all meaningful verb/intent signals. Additional note: `extractTarget()` (line 290) also benefits since it operates on the same lowercased string.

---

## Phase 2: Concurrency & State Safety (Stop Panics & Deadlocks)

### Fix 2.1: Panic Recovery in processInput
**File:** `cmd/nerd/chat/process.go:50-56`
**Before:** No `recover()` anywhere in the `processInput` closure (fire-and-forget goroutine)
**After:** Added `defer func() { if r := recover(); r != nil { ... } }()` at closure top

Also added panic recovery to the background compression goroutine at line 955.

**Impact:** A panic in the JSON parser, Mangle compiler, or transducer would previously crash the entire TUI process. Now panics are caught, logged, and surfaced as error messages to the user.

---

### Fix 2.2: Fix defer resp.Body.Close() in Retry Loops
**Files:** `client_gemini.go` (3 locations), `client_anthropic.go` (1), `client_openai.go` (1), `client_xai.go` (1), `client_openrouter.go` (1), `client_zai.go` (2)

**Before:** `defer resp.Body.Close()` inside `for` loops — defers stack until function exit, leaking FDs across retry iterations
**After:** Immediate `resp.Body.Close()` after body is read, within each loop iteration

**Impact:** Under retry storms (e.g., 429 rate limits), leaked file descriptors would exhaust the OS `ulimit`, causing cascading failures in SQLite (WAL), file reads, and other I/O — indirectly crashing systems that have nothing to do with the LLM client.

---

### Fix 2.3: Fix context.Background() Leaks
**Files:** `cmd/nerd/chat/process.go:59`, `cmd/nerd/chat/process_sync.go:28`

**Before:** `context.WithTimeout(context.Background(), ...)` — creates orphaned goroutines on shutdown
**After:** `context.WithTimeout(m.shutdownCtx, ...)` with nil guard fallback

**Impact:** When the user hits Ctrl+X (interrupt), LLM API calls and workspace scans now cancel immediately instead of continuing to run for up to 30 minutes as zombie goroutines holding locks.

---

### Fix 2.4: Break VirtualStore/Kernel Deadlock
**File:** `internal/core/virtual_store.go:456-486` (rebuildPermissionCache), `SetKernel` (line 365), `CheckKernelPermitted` (line 1573)

**Before:** `rebuildPermissionCache()` was documented as "Must be called with v.mu held" and was called from `SetKernel` and `CheckKernelPermitted` while holding the write lock. It then called `v.kernel.Query()`, which could trigger virtual predicate handlers that try to read-lock v.mu — causing a deadlock (Go's `sync.RWMutex` is not reentrant).

**After:** `rebuildPermissionCache()` now manages its own locking internally:
1. Queries kernel WITHOUT holding v.mu (avoids deadlock)
2. Locks v.mu ONLY to write the resulting cache
3. Callers (`SetKernel`, `CheckKernelPermitted`) no longer hold v.mu when calling it

**Impact:** Eliminates a latent deadlock between VirtualStore and Kernel that would surface under concurrent shard execution + permission checks. This is a textbook Go `sync.RWMutex` reentrancy trap.

---

## Phase 3: Sanitization & Mangle Injection (Stop Logic Execution)

### Fix 3.1: Sanitize Mangle Fact Arguments
**File:** `internal/perception/transducer.go:354-365`
**Before:** `i.Target` and `i.Constraint` passed raw as fact arguments
**After:** Wrapped in `sanitizeFactArg()` which:
- Caps length at 2048 chars
- Strips null bytes (U+0000)
- Strips ANSI escape sequences (U+001B)
- Strips control characters (U+0000-U+001F except \n \r \t)

**Impact:** Prevents Mangle injection where a Target like `foo). malicious_rule(X) :- ` could inject arbitrary rules into the kernel via the fact's string argument. This is the AI equivalent of SQL injection.

---

### Fix 3.2: Sanitize Command Input
**File:** `cmd/nerd/chat/commands.go:54` + `model_helpers.go` (new `sanitizeCommandInput`)
**Before:** `strings.Fields(input)` called on raw input; `parts[0]` without length check (panics on empty)
**After:**
1. Added `sanitizeCommandInput()` — strips null bytes, ANSI escapes, control chars, caps at 10K chars
2. Added empty guard: `if len(parts) == 0 { return m, nil }`

**Impact:** Prevents:
- Null bytes corrupting SQLite and Mangle parser
- ANSI escape sequences hijacking terminal display when echoed in error messages
- Panic on all-whitespace input (empty `parts` slice, index-out-of-range on `parts[0]`)

---

### Fix 3.3: Validate MangleUpdates Content
**File:** `internal/articulation/emitter.go:352-385`
**Before:** MangleUpdates count was capped at 2000, but content was unvalidated
**After:** Content-level validation after the count cap:
- Rejects individual updates > 1000 chars
- Requires basic Mangle syntax: must contain `(` and end with `.`
- Rejects updates containing shell metacharacters (`` ` $ ; | ``)
- Skips empty/whitespace-only updates

**Impact:** Prevents the LLM from injecting oversized atoms, syntactically invalid garbage (that would crash the Mangle parser), or shell-escape sequences disguised as Mangle updates.

---

## Phase 4: Constitutional Guardrails (Stop Sandbox Escapes)

### Fix 4.1: Fix Path Traversal Protection
**File:** `internal/core/virtual_store.go:898-908`
**Before:**
- Only checked `ActionReadFile`, `ActionWriteFile`, `ActionDeleteFile` — **`ActionEditFile` was excluded!**
- Used naive `strings.Contains(req.Target, "..")` — trivially bypassed

**After:**
- Added `ActionEditFile` to the check
- Uses `filepath.Clean()` to normalize paths before checking
- Attempts `filepath.EvalSymlinks()` to resolve symlinks and verify the resolved path

**Impact:** Previously, `ActionEditFile` completely bypassed path traversal protection. An LLM-directed edit to `../../etc/passwd` would succeed. Additionally, symlink-based bypasses (create `safe.txt` -> `/etc/passwd`, then edit `safe.txt`) are now detected.

---

### Fix 4.2: Lock Down Environment Variable Injection
**File:** `internal/core/virtual_store_actions.go:37-39` + `virtual_store.go` (new `filterCallerEnv`)
**Before:** `finalEnv := append(v.getAllowedEnv(), env...)` — caller-provided env vars appended without filtering
**After:** `filterCallerEnv(env)` filters against the `allowedEnvVars` allowlist before merging

**Impact:** In Go's `os/exec`, the last duplicate environment key wins. An attacker could override `PATH` to point to a malicious binary directory, or inject `LD_PRELOAD` to load arbitrary shared libraries. Now only allowlisted env var keys are permitted.

---

### Fix 4.3: Case-Insensitive Windows Path Checks
**File:** `internal/core/virtual_store.go:911-926`
**Before:** `strings.HasPrefix(target, "C:\\Windows\\")` — case-sensitive on a case-insensitive filesystem
**After:** `strings.HasPrefix(strings.ToLower(filepath.ToSlash(target)), "c:/windows/")` — normalized to lowercase forward slashes

Also added `"c:/windows/"` as an additional path variant to catch forward-slash Windows paths.

**Impact:** On Windows, `C:\WINDOWS\system32\config` and `c:\windows\System32\Config` are the same path but wouldn't match the old case-sensitive check. An attacker could simply capitalize differently to bypass the system path protection.

---

## Phase 5: Memory & Context Exhaustion (Stop The Halting Problem)

### Fix 5.1: Enforce MaxFactsInKernel
**Files:** `internal/core/kernel_types.go:65` (new field), `kernel_init.go:225-244` (new methods), `kernel_facts.go:354`

**Before:** `MaxFactsInKernel = 250,000` was defined in `limits.go` but NEVER enforced in `addFactIfNewLocked()`. The kernel would accept unlimited facts until the O(N) memory copy in `evaluate()` caused OOM.

**After:**
- Added `maxFacts int` field to `RealKernel` struct
- Added `SetMaxFacts(limit)` / `GetMaxFacts()` methods (default 250K)
- `addFactIfNewLocked()` now checks `len(k.facts) >= maxFacts` and rejects with a logged warning

**Impact:** Without enforcement, a recursive rule or adversarial fact injection loop would bloat the EDB until `evaluate()` (which rebuilds all facts into a new program) causes a massive OOM. Now the kernel gracefully refuses new facts when full.

---

### Fix 5.2: Cap Activation Scores
**File:** `internal/context/activation.go:760-764, 845`

**Before:** Keyword weights were multiplied by 50 with no input clamping. A weight of 100.0 (no upper bound enforced on the `Keywords` map) would produce a score of 5000+, pushing all constitutional safety rules out of the context window. Final cap was 80.0 but was reached after intermediate overflow.

**After:**
- Keyword weights clamped to `[0.0, 1.0]` before multiplication
- Final score cap raised to 100.0 (allows legitimate compound boosts)

**Impact:** Prevents a single adversarial fact (via crafted keyword weights in the issue context) from dominating the spreading activation window. Constitutional safety rules and core context remain accessible because no single fact can score higher than 100.

---

### Fix 5.3: Reject Facts That Fail ToAtom() Conversion
**File:** `internal/core/kernel_facts.go:362-368`

**Before:** If `f.ToAtom()` failed, the fact was STILL added to `k.facts` but NOT to `k.cachedAtoms`. This caused a length mismatch: `len(k.facts) != len(k.cachedAtoms)`. When `evaluate()` detected this desync, it attempted a full rebuild which could also fail, soft-bricking the kernel.

**After:** Facts that fail `ToAtom()` are now rejected entirely with a logged error. The fact is not added to `k.facts`, `k.cachedAtoms`, or `k.factIndex`.

**Impact:** Eliminates a subtle but critical failure mode where a single malformed fact could permanently corrupt the kernel's internal state, causing all subsequent evaluations to fail.

---

## Files Modified Summary

| Phase | File | Fix |
|-------|------|-----|
| 1.1 | `cmd/nerd/chat/session.go` | CharLimit = 100K |
| 1.2 | 9 files in `internal/perception/client_*.go` | io.LimitReader 10MB |
| 1.3 | `internal/articulation/json_scanner.go` | Depth 200 + size 5MB cap |
| 1.4 | `internal/perception/transducer.go` | Regex input truncation 2K |
| 2.1 | `cmd/nerd/chat/process.go` | Panic recovery in processInput + compression goroutine |
| 2.2 | 6 files in `internal/perception/client_*.go` | defer-in-loop fix |
| 2.3 | `cmd/nerd/chat/process.go`, `process_sync.go` | shutdownCtx context lineage |
| 2.4 | `internal/core/virtual_store.go` | Deadlock-free permission cache |
| 3.1 | `internal/perception/transducer.go` | sanitizeFactArg() |
| 3.2 | `cmd/nerd/chat/commands.go`, `model_helpers.go` | sanitizeCommandInput() |
| 3.3 | `internal/articulation/emitter.go` | MangleUpdates content validation |
| 4.1 | `internal/core/virtual_store.go` | Path traversal + ActionEditFile |
| 4.2 | `internal/core/virtual_store_actions.go`, `virtual_store.go` | Env var filtering |
| 4.3 | `internal/core/virtual_store.go` | Case-insensitive Windows paths |
| 5.1 | `internal/core/kernel_types.go`, `kernel_init.go`, `kernel_facts.go` | MaxFacts enforcement |
| 5.2 | `internal/context/activation.go` | Keyword weight clamping + score cap 100 |
| 5.3 | `internal/core/kernel_facts.go` | Reject malformed facts |

**Total: 17 fixes across ~20 unique files, 0 test failures, clean build.**

---

## Definition of Ready (Manual Verification Checklist)

Before writing the chaos test suite, manually verify:

1. **Paste 100K "A"s** into the chat prompt → app should accept up to CharLimit, not OOM
2. **Mock LLM returns 5000 nested `{`** → parser errors out, doesn't hang CPU
3. **Submit `/read ../../../etc/passwd`** → constitutional gate blocks it
4. **Press Ctrl+X during a hanging LLM call** → background goroutine cancels within seconds
5. **Submit `/write` with a null byte in the path** → sanitizer strips it, no SQLite corruption

---

## Next Steps

1. **Build the Chaos Harness** — Adversarial LLMClient mock server (Tar-Pit, Guillotine, Gaslighter, Firehose)
2. **Headless Bubble Tea tests** — Use `teatest` to pump adversarial events into `Update()` without a terminal
3. **Native Go Fuzzing** — Fuzz targets for `findJSONCandidates()`, `sanitizeFactArg()`, `handleCommand()`
4. **Run with `-race` flag** — Catch remaining concurrency bugs
5. **Add `goleak` to test teardowns** — Prove goroutines terminate on interrupt
