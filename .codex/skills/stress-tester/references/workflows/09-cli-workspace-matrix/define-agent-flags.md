# Workflow: `define-agent` Required Flags (`--name`, `--topic`)

## What It Stresses

- Cobra flags: `--name` and `--topic` are **required** (`MarkFlagRequired`)
- Positional args are **not** accepted for name/topic (live matrix 2026-07 failed with bare invocation)
- Name validation: alphanumeric + dash/underscore only (`^[a-zA-Z0-9_-]+$`)
- Happy path boots Cortex and defines a Type U specialist profile

## Why This Exists (2026-07 live matrix)

Matrix step `04_define_agent` invoked define-agent without flags and failed in ~2s:

```text
Error: required flag(s) "name", "topic" not set
```

Correct form (from `cmd/nerd/cmd_spawn.go` / `main.go`):

```text
nerd define-agent --name RustExpert --topic "Tokio Async Runtime"
```

**Not** `nerd define-agent <name> <desc>` (positional).

## Severity Levels

| Level | Action |
|-------|--------|
| **Conservative** | Negative: missing flags; Positive: valid --name/--topic smoke |
| **Aggressive** | Invalid names (spaces, path traversal chars); long topic string |
| **Chaos** | Duplicate define, empty topic if flag present as `""`, unicode name |
| **Hybrid** | define-agent then `nerd agents` / spawn specialist by name under `-w` |

## Conservative Procedure (PowerShell)

```powershell
$ErrorActionPreference = "Continue"
$APP = Join-Path $env:TEMP "nerd-define-agent-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $APP | Out-Null

# --- Negative: missing required flags (must fail fast, exit != 0) ---
& nerd define-agent -w $APP 2>&1 | Tee-Object -Variable missBoth
if ($LASTEXITCODE -eq 0) { throw "FAIL: define-agent without flags should not succeed" }
if ("$missBoth" -notmatch "required flag") {
  Write-Warning "Expected required-flag error text; got: $missBoth"
}

& nerd define-agent --name OnlyName -w $APP 2>&1 | Tee-Object -Variable missTopic
if ($LASTEXITCODE -eq 0) { throw "FAIL: missing --topic should fail" }

& nerd define-agent --topic "OnlyTopic" -w $APP 2>&1 | Tee-Object -Variable missName
if ($LASTEXITCODE -eq 0) { throw "FAIL: missing --name should fail" }

# --- Negative: positional-style (must not silently succeed as old API) ---
& nerd define-agent PositionalName "some topic" -w $APP 2>&1 | Tee-Object -Variable positional
if ($LASTEXITCODE -eq 0) {
  Write-Warning "Positional args accepted unexpectedly — document as regression if intentional"
}

# --- Positive: valid flags ---
Get-Process nerd -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
& nerd define-agent --name MatrixAgent --topic "Live matrix specialist smoke" -w $APP --timeout 15m
$ok = $LASTEXITCODE
# May need LLM for research step; if auth missing, expect clean error not panic
if ($ok -ne 0) {
  Write-Warning "define-agent happy path exit $ok (check LLM/auth); flag parsing still validated above"
} else {
  Write-Host "define-agent succeeded for MatrixAgent"
}

# Optional: agents list
& nerd agents -w $APP
```

### Aggressive — name validation

```powershell
$APP = Join-Path $env:TEMP "nerd-define-agent-val-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $APP | Out-Null

# Spaces / injection-ish names must fail
foreach ($bad in @("Bad Name", "../escape", "a/b", "has!bang")) {
  & nerd define-agent --name $bad --topic "x" -w $APP 2>&1 | Out-Null
  if ($LASTEXITCODE -eq 0) { throw "FAIL: invalid name accepted: $bad" }
}

# Valid charset
& nerd define-agent --name "Good_Agent-1" --topic "Valid charset" -w $APP --timeout 15m
# exit 0 or clean LLM error; never panic
```

## Pass Criteria

- [ ] Missing `--name` and/or `--topic` → non-zero exit + clear required-flag message
- [ ] Help text documents `--name` / `--topic` as required
- [ ] Invalid agent names rejected (no path traversal / spaces)
- [ ] Valid flags reach Cortex boot (or clean pre-boot error)
- [ ] Process exits cleanly (no Close hang after success — see [one-shot-cli-exit.md](one-shot-cli-exit.md))
- [ ] No `debug_program_ERROR.mg`

## Related Surfaces

- `cmd/nerd/main.go` — `MarkFlagRequired("name")`, `MarkFlagRequired("topic")`
- `cmd/nerd/cmd_spawn.go` — `defineAgent`, name regexp
- Type U shards / `/define-agent` chat wizard (interactive path is separate)
- [full-cli-surface.md](full-cli-surface.md) catalog row (use flags, not positionals)
