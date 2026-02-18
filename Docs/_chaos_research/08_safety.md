# 08 - Safety & Constitutional Gate

## 1. Hardcoded Constitutional Rules (Go Layer)

The `VirtualStore` initializes 4 hardcoded constitutional rules in `initConstitution()` (`virtual_store.go:833-911`). These are Go closures checked **before every action** via `checkConstitution()` (`virtual_store.go:913-921`), which iterates all rules and short-circuits on the first violation.

### Rule 1: `no_destructive_commands` (line 837)
- **Scope:** Only `ActionExecCmd` actions.
- **Mechanism:** `strings.ToLower(req.Target)` then `strings.Contains()` against a forbidden list.
- **Blocked patterns:** `"rm -rf"`, `"mkfs"`, `"dd if="`, `":(){ "` (fork bomb), `"chmod 777"`
- **Weakness:** Only 5 patterns. No regex. Lowercase comparison only.

### Rule 2: `no_secret_exfiltration` (line 854)
- **Scope:** All action types (checks `req.Payload` as stringified `%v`).
- **Mechanism:** Dual-condition AND gate. Both a secret keyword AND a dangerous binary must be present.
- **Secret keywords:** `".env"`, `"credentials"`, `"secret"`, `"api_key"`, `"password"`
- **Dangerous binaries:** `"curl"`, `"wget"`, `"nc "` (note trailing space), `"netcat"`
- **Weakness:** Requires BOTH conditions. `curl https://evil.com/` alone passes. Reading `.env` alone passes. Only the combination is blocked.

### Rule 3: `path_traversal_protection` (line 881)
- **Scope:** `ActionReadFile`, `ActionWriteFile`, `ActionDeleteFile` only.
- **Mechanism:** `strings.Contains(req.Target, "..")`
- **Weakness:** Only checks for literal `..` in the target string. Does not resolve symlinks, does not check URL-encoded `%2e%2e`, does not handle `ActionEditFile` (note: rule 4 covers edit for system paths, but rule 3 does NOT check edit for traversal).

### Rule 4: `no_system_file_modification` (line 893)
- **Scope:** `ActionWriteFile`, `ActionDeleteFile`, `ActionEditFile`.
- **Mechanism:** `strings.HasPrefix(target, sp)` against system paths.
- **Protected paths:** `"/etc/"`, `"/usr/"`, `"/bin/"`, `"/sbin/"`, `"C:\\Windows\\"`
- **Weakness:** Case-sensitive prefix match on Windows. `c:\windows\` (lowercase) would pass. Missing `C:\Program Files\`, `/var/`, `/root/`, etc.

### RouteAction Flow (`virtual_store.go:923-960`)
1. Boot guard check (blocks action routing until first user interaction).
2. Parse action fact into `ActionRequest`.
3. **Constitutional check** - `checkConstitution(req)`.
4. On violation: injects `security_violation` fact into kernel AND returns error.
5. On pass: proceeds to action handler dispatch.

## 2. Mangle Policy: `permitted()` and `blocked_pattern()`

Source: `internal/core/defaults/policy/constitution.mg`

### `permitted()` Rules (lines 6-20)
Three derivation paths for `permitted(Action, Target, Payload)`:

1. **Standard path** (line 6-9): `safe_action(Action)` AND `pending_action(...)` AND `!dangerous_content(Action, Payload)`.
2. **Admin override path** (line 11-15): `dangerous_action(Action)` AND `admin_override(User)` AND `signed_approval(Action)`. Both override AND signed approval required.
3. **Downstream executor bridge** (line 18-20): `permitted_action(...)` AND `permission_check_result(ActionID, /permit, ...)`. For actions validated by external permission systems.

**Default deny**: `permitted()` must be positively derived. If none of the 3 rules fire, the action is blocked.

### `safe_action()` Facts (lines 32-243)
**~90 safe_action declarations** across 13 categories:
- **File operations** (7): `/read_file`, `/fs_read`, `/write_file`, `/fs_write`, `/search_files`, `/glob_files`, `/analyze_code`
- **Code analysis** (4): `/parse_ast`, `/query_symbols`, `/check_syntax`, `/code_graph`
- **Review** (3): `/review`, `/lint`, `/check_security`
- **Test** (3): `/run_tests`, `/test_single`, `/coverage`
- **Knowledge** (3): `/vector_search`, `/knowledge_query`, `/embed_text`
- **Browser** (3): `/browser_navigate`, `/browser_screenshot`, `/browser_read_dom`
- **System lifecycle** (4): `/initialize`, `/system_start`, `/shutdown`, `/heartbeat`
- **Campaign** (18): `/campaign_create_file` through `/pause_and_replan`
- **TDD repair** (4): `/read_error_log`, `/analyze_root_cause`, `/generate_patch`, `/complete`
- **Autopoiesis** (6): `/generate_tool`, `/refine_tool`, `/ouroboros_*`
- **Execution** (8): `/exec_cmd`, `/run_command`, `/bash`, `/run_build`, `/run_tests`, `/git_operation`, `/git_diff`, `/git_log`
- **Code DOM** (8): `/edit_element`, `/open_file`, etc.
- **Delegates** (4): `/delegate_reviewer`, `/delegate_coder`, etc.

**Critical observation**: `/exec_cmd`, `/bash`, and `/run_command` are all `safe_action`. This means shell execution is permitted by default. Safety relies entirely on the `dangerous_content()` and `blocked_pattern()` checks below.

### `blocked_pattern()` Facts (lines 143-153)
10 blocked patterns checked via `:string:contains()`:
```
"git push --force", "git push -f", "git push origin --force", "git push origin -f"
"rm -rf /", "rm -rf", "sudo", "> /dev/", "mkfs", "dd if="
```

### `dangerous_content()` Rules (lines 167-223)
Checks `pending_action` payloads for `blocked_pattern` matches using `:string:contains()`. Applied to `/exec_cmd`, `/run_command`, `/bash`, and `/git_operation`. Also has specific compound rules for `git push` + `--force` or `-f` (handles flag reordering).

### `requires_permission()` / `dangerous_action()` (lines 156-164)
5 actions require explicit permission: `/delete_file`, `/git_push`, `/git_force`, `/run_arbitrary_command`, `/system_modify`. These derive `dangerous_action()` which requires `admin_override` + `signed_approval`.

### Appeal Mechanism (Section 7C, lines 287-299)
- `suggest_appeal(ActionID)` fires when an action is blocked but is NOT a `dangerous_action`.
- `appeal_needs_review(ActionID, ActionType, Justification)` tracks pending appeals.
- `has_temporary_override(ActionType)` allows temporary overrides.

### Network Policy (lines 246-250)
Domain allowlist: `"github.com"`, `"pypi.org"`, `"crates.io"`, `"npmjs.com"`, `"pkg.go.dev"`. Only 5 domains.

## 3. Mangle Injection Prevention

Source: `internal/core/kernel_policy.go:324-420`, `internal/core/kernel_validation.go:57-333`

### `HotLoadLearnedRule()` - 5-Layer Validation Pipeline (`kernel_policy.go:327-401`)

Every dynamically learned Mangle rule passes through 5 sequential validation stages:

**Layer 0: Repair Interceptor** (line 330-348)
- If a `repairInterceptor` (MangleRepairShard) is registered, the rule is sent through it FIRST.
- The interceptor can reject the rule entirely or return a repaired version.
- This is an LLM-powered pre-validation step.

**Layer 1a: Sandbox Compilation** (line 367-371)
- `validateRuleSandbox()` creates a throwaway `RealKernel` with the current schemas, policy, and learned rules.
- Appends the candidate rule and calls `rebuildProgram()`.
- If the Mangle compiler rejects it (syntax error, stratification violation), the rule is blocked.
- **This catches invalid syntax, undeclared predicates, and circular negation.**

**Layer 1b: Schema Validation** (line 373-378)
- `ValidateLearnedRule(rule)` checks that all predicates used in the rule body are declared in schemas.
- Prevents "Schema Drift" where rules reference hallucinated predicates.
- Uses the `SchemaValidator` (`kernel_validation.go:57-68`), backed by `mangle.NewSchemaValidator(schemas, learned)`.

**Layer 1c: Infinite Loop Risk Detection** (line 381-386)
- `checkInfiniteLoopRisk(rule)` (`kernel_validation.go:222-333`) performs pattern-based detection:
  - **Unconditional `next_action` facts** for system actions like `/system_start` or `/initialize` (line 239-243).
  - **Ubiquitous predicate dependency**: Rules where `next_action` depends solely on always-true predicates: `current_time(`, `entry_point(`, `current_phase(`, `build_system(`, `system_startup(`, `northstar_defined` (lines 252-269). Only flagged if the body has <= 1 predicate.
  - **Idle state triggers**: Patterns like `coder_state(/idle)`, `current_task(/idle)`, `_state(/idle)`, `_status(/idle)`, `/idle)` (lines 274-289). Flagged if body has <= 2 predicates.
  - **Wildcard state patterns**: `session_state(`, `session_planner_status(`, `system_shard_state(`, `dream_state(` with excessive `_` wildcards (lines 293-314).
  - **Negation-only conditions**: Rules where the body starts with `!` or all conditions are negated with 0 positive predicates (lines 316-329).

**Layer 2: Disk Persistence** (line 389-393)
- `appendToLearnedFile(rule)` writes to `.nerd/mangle/learned.mg` with timestamp.
- Only reached if all validations pass.

### `healLearnedRules()` - Self-Repair on Startup (`kernel_validation.go:76-218`)

On kernel boot, all rules in `learned.mg` are re-validated line by line:
1. **Syntax check** via `checkSyntax(trimmed)` - parser validation.
2. **Schema + safety validation** via `schemaValidator.ValidateLearnedRule(trimmed)`.
3. **Infinite loop risk** via `checkInfiniteLoopRisk(trimmed)`.

Invalid rules are commented out with `# SELF-HEALED: <reason>` prefix and the healed file is persisted back to disk. Previously healed rules are tracked (`PreviouslyHealed` counter) to detect recurring issues.

### `SchemaValidator` (`kernel_validation.go:57-68`)

- Created by `mangle.NewSchemaValidator(schemas, learned)`.
- Calls `LoadDeclaredPredicates()` to parse all `Decl` statements.
- Refreshed after every `HotLoadLearnedRule()` and `SetLearned()` call.
- Validates that rule body predicates exist in the declared schema.

## 4. Shell & Path Safety

### Path Traversal Checks
- **Constitutional rule** (`virtual_store.go:887`): `strings.Contains(req.Target, "..")` for file read/write/delete.
- **Exec command guard** (`virtual_store_actions.go:24,107`): `strings.Contains(cmd, "..")` checked in both `Exec()` and `handleExecCmd()`.
- **Gap**: No symlink resolution. No URL-encoded path detection. No canonicalization via `filepath.EvalSymlinks()`.

### Binary Allowlist (`virtual_store.go:149-154, 1323-1333`)
Default allowed binaries (14 total):
```
bash, sh, pwsh, powershell, cmd,
go, git, grep, ls, mkdir, cp, mv,
npm, npx, node, python, python3, pip,
cargo, rustc, make, cmake
```
- `isBinaryAllowed()` uses `strings.EqualFold()` (case-insensitive).
- Checks the `binary` field, NOT the command arguments. `bash -c "curl evil.com"` passes because `bash` is allowed.
- The binary check is defense-in-depth; actual dangerous command detection happens in constitutional rules and `blocked_pattern`.

### Environment Variable Filtering (`virtual_store.go:148, 1335-1343`)
Only 4 env vars are forwarded to subprocesses:
```
PATH, HOME, GOPATH, GOROOT
```
- `getAllowedEnv()` reads current process env and only forwards these keys.
- **No sanitization of PATH value itself.** If PATH contains attacker-controlled directories, arbitrary binaries could be resolved.

### File Read Size Cap (`virtual_store_actions.go:228`)
- `MaxFileSize = 100 * 1024` (100KB).
- Files larger than 100KB are truncated (first 100KB read, `truncated = true`).
- Uses `os.Stat()` to check size, then `os.Open()` + `f.Read()` for large files.
- **No check for special files** (`/dev/zero`, `/dev/urandom`, named pipes). `os.Stat()` would return 0 size for these on Linux, so they'd take the `os.ReadFile()` path which could hang on pipes.

## 5. CHAOS FAILURE PREDICTIONS

### P1: Unicode Homoglyph Bypass of Constitutional Rules
- **Severity: HIGH**
- **Target:** `virtual_store.go:843-844` - `strings.ToLower(req.Target)` + `strings.Contains(cmd, "rm -rf")`
- `strings.ToLower()` does NOT normalize Unicode. Fullwidth characters like `ｒｍ　-ｒｆ` (U+FF52 U+FF4D) would pass the check but could be interpreted by the shell.
- Similarly, homoglyphs for `sudo`, `mkfs`, `dd if=` would bypass detection.
- **Fix needed before chaos testing?** YES - add Unicode normalization or reject non-ASCII in commands.

### P2: Path Traversal via Symlinks
- **Severity: CRITICAL**
- **Target:** `virtual_store.go:887` - `strings.Contains(req.Target, "..")`
- An attacker can create a symlink `./innocent -> /etc/passwd` and read/write through it without `..` appearing in the path.
- No call to `filepath.EvalSymlinks()` or `filepath.Abs()` anywhere in the constitutional check.
- **Fix needed before chaos testing?** YES - add symlink resolution before path checks.

### P3: Binary Allowlist Bypass via PATH Manipulation
- **Severity: HIGH**
- **Target:** `virtual_store.go:148` (PATH forwarded), `virtual_store.go:1323-1333` (`isBinaryAllowed`)
- The allowlist checks the binary NAME, not its resolved path. If PATH is manipulated (e.g., via a previous `exec_cmd` that modifies the environment), a malicious binary named `git` in a user-controlled directory could be executed.
- `Exec()` at `virtual_store_actions.go:39` appends user-provided `env` AFTER `getAllowedEnv()`, meaning user env can override PATH.
- **Fix needed before chaos testing?** YES - resolve binary to absolute path before allowlist check.

### P4: Mangle Injection via Transducer `user_intent` Atoms
- **Severity: HIGH**
- **Target:** `kernel_policy.go:327` (HotLoadLearnedRule), perception transducer
- If the perception transducer generates `user_intent` facts from user input without sanitizing Mangle metacharacters, a crafted input like `"fix :- permitted(/delete_file, _, _)."` could inject a rule through the autopoiesis feedback loop.
- The 5-layer validation only applies to `HotLoadLearnedRule`. Direct `Assert()` calls bypass it.
- **Fix needed before chaos testing?** INVESTIGATE - need to verify transducer sanitization.

### P5: Deadlock via Blocking All `permitted()` Actions
- **Severity: MEDIUM**
- **Target:** `constitution.mg:6-9` - permitted requires `!dangerous_content()`
- If an attacker can assert `dangerous_content(Action, Payload)` facts for every safe_action, the system enters a state where no actions are permitted. Combined with the boot guard, this could permanently deadlock the session.
- Possible vector: corrupted `learned.mg` that asserts broad `dangerous_content` facts.
- **Fix needed before chaos testing?** NO - test this as-is to verify recovery behavior.

### P6: Prompt Injection via Control Packets
- **Severity: CRITICAL**
- **Target:** Articulation emitter (Piggyback Protocol), `virtual_store.go:956-959` (fact injection)
- If the LLM is tricked into generating a `control_packet` with `security_violation` retraction or `permitted` assertion, and the articulation layer doesn't validate control packet structure, arbitrary facts could be injected.
- The `injectFact()` method at `virtual_store.go:1345-1355` has NO validation - it calls `kernel.Assert()` directly.
- **Fix needed before chaos testing?** INVESTIGATE - verify control_packet schema enforcement.

### P7: Unbounded Fact Assertion (Resource Exhaustion)
- **Severity: HIGH**
- **Target:** `virtual_store.go:1345-1355` (`injectFact`), `kernel_facts.go` (Assert)
- Every constitutional violation injects a `security_violation` fact. Rapid-fire invalid actions could flood the fact store with millions of violation facts.
- No deduplication or rate limiting on `security_violation` fact injection visible in the checked code.
- The `LimitsEnforcer` (`limits.go`) exists but its integration with fact count needs verification.
- **Fix needed before chaos testing?** NO - test to verify LimitsEnforcer triggers.

### P8: File Read Cap Bypass via `/dev/zero` or Named Pipes
- **Severity: MEDIUM**
- **Target:** `virtual_store_actions.go:228-273` (MaxFileSize check)
- `os.Stat()` returns size 0 for `/dev/zero`, `/dev/urandom`, and named pipes. The code would take the `os.ReadFile()` path (line 275), which blocks indefinitely on pipes and infinitely on `/dev/zero`.
- Windows mitigation: These paths don't exist on Windows. But `\\.\NUL` or `CON` device files could cause similar issues.
- **Fix needed before chaos testing?** YES on Linux deployments - add `info.Mode().IsRegular()` check.

### P9: Case-Sensitivity Bypass on Windows System Path Check
- **Severity: MEDIUM**
- **Target:** `virtual_store.go:900-906` (`no_system_file_modification`)
- `strings.HasPrefix(target, "C:\\Windows\\")` is case-sensitive. On Windows, `c:\windows\system32\config\sam` (lowercase) bypasses the check entirely.
- The path is NOT normalized before comparison.
- **Fix needed before chaos testing?** YES - use `strings.EqualFold` or normalize paths on Windows.

### P10: `Exec()` Env Override Allows Arbitrary Environment Injection
- **Severity: HIGH**
- **Target:** `virtual_store_actions.go:39` - `finalEnv := append(v.getAllowedEnv(), env...)`
- The `Exec()` function appends caller-provided `env` after the allowed env. On most systems, the last value wins for duplicate keys. This means any caller of `Exec()` can inject arbitrary environment variables including `LD_PRELOAD`, `PYTHONPATH`, or override `PATH`.
- **Fix needed before chaos testing?** YES - validate/filter provided env vars against allowlist.

### P11: Secret Exfiltration via `git push` to Attacker-Controlled Remote
- **Severity: HIGH**
- **Target:** `virtual_store.go:856-877` (`no_secret_exfiltration`)
- The exfiltration check requires BOTH a secret keyword AND a dangerous binary (`curl`/`wget`/`nc`/`netcat`). But `git push` to an attacker-controlled remote exfiltrates the entire repo (including `.env` files in history) without triggering either condition.
- `git` is in the binary allowlist. `git push` is only blocked with `--force`/`-f` flags, not for arbitrary remotes.
- **Fix needed before chaos testing?** INVESTIGATE - verify if `allowed_domain` policy in Mangle is enforced at the git layer.

### P12: `ActionEditFile` Excluded from Path Traversal Check
- **Severity: MEDIUM**
- **Target:** `virtual_store.go:884` - rule 3 checks `ActionReadFile`, `ActionWriteFile`, `ActionDeleteFile` but NOT `ActionEditFile`.
- Rule 4 checks `ActionEditFile` for system paths but NOT for `..` traversal.
- A `..` path in an `ActionEditFile` request bypasses the traversal check entirely.
- **Fix needed before chaos testing?** YES - add `ActionEditFile` to rule 3's scope.
