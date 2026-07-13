#Requires -Version 5.1
# write-scope-guard.ps1 - PreToolUse hook (matcher: Write|Edit), corpus-build fleet.
#
# Enforces the corpus-build fleet's write scope (PLAN-corpus-build.md SS5/SS3.3):
#
#   1. .corpus-build/** is always writable -- it is the fleet's own ledger,
#      intents, manifests, and results directory.
#   2. UNIVERSAL: Docs/architecture/** is read-only for every agent_type EXCEPT
#      corpus-doc-auditor. Builders never edit the architecture corpus directly.
#   3. UNIVERSAL: reserved files (internal/app/server/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main), .nerd/config.json and internal/config,
#      MCP/tool schemas**) are blocked for agent_type corpus-builder -- those changes go
#      through the registration-intent pattern
#      (.corpus-build/intents/<WU>_intents.json) so parallel work units never
#      collide on a shared reserved file.
#   4. If a slice manifest exists for this agent_type
#      (.corpus-build/slices/current/<agent_type>.json, shape
#      {"work_unit":"WU-NNN","files":[...]}), the write is scoped to exactly
#      the files it lists (plus .corpus-build/** from rule 1). No manifest for
#      this agent_type = permissive: only rules 2-3 apply.
#
# Fail-open: any parse error, missing field, or corrupt manifest allows the
# write. This hook narrows an already-permissioned agent; it must never be the
# reason a session breaks.

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path "$PSScriptRoot\..\..\..").Path -replace '\\', '/'

function Read-StdinJson {
    $raw = [Console]::In.ReadToEnd()
    if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
    try { return ($raw | ConvertFrom-Json -ErrorAction Stop) } catch { return $null }
}

function Allow { exit 0 }

function Deny([string]$reason) {
    $payload = @{
        hookSpecificOutput = @{
            hookEventName            = 'PreToolUse'
            permissionDecision       = 'deny'
            permissionDecisionReason = $reason
        }
    }
    try { ($payload | ConvertTo-Json -Depth 6 -Compress) | Write-Output } catch { }
    [Console]::Error.WriteLine($reason)
    exit 2
}

function Normalize-RelPath([string]$filePath) {
    if ([string]::IsNullOrWhiteSpace($filePath)) { return $null }
    $norm = $filePath -replace '\\', '/'
    if ($norm.ToLower().StartsWith($RepoRoot.ToLower() + '/')) {
        return $norm.Substring($RepoRoot.Length + 1)
    }
    if ($norm -match '^[A-Za-z]:/') {
        # Absolute path outside the repo root -- return as-is; none of the
        # repo-relative rules below will match it, so it falls through to Allow.
        return $norm
    }
    return ($norm -replace '^\./', '')
}

try {
    $data = Read-StdinJson
    if ($null -eq $data) { Allow }

    $toolName = $data.tool_name
    if ($null -ne $toolName -and $toolName -notin @('Write', 'Edit')) { Allow }

    $filePath = $data.tool_input.file_path
    $rel = Normalize-RelPath $filePath
    if ([string]::IsNullOrWhiteSpace($rel)) { Allow }

    $agentType = $data.agent_type
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = $data.subagent_type }
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = '' }

    # Rule 1: the fleet's own scratch/artifact directory is always writable.
    if ($rel -match '^\.corpus-build/') { Allow }

    # Rule 2 (UNIVERSAL): architecture corpus is read-only except for corpus-doc-auditor.
    if ($rel -match '^Docs/architecture/' -and $agentType -ne 'corpus-doc-auditor') {
        Deny "WRITE-SCOPE-GUARD blocked $toolName on $rel`n`nThe architecture corpus (Docs/architecture/**) is read-only for builders. Doc reconciliation (IMPLEMENTED_SPEC status, frontmatter stamps, tag/roadmap register regen) goes through corpus-doc-auditor -- hand this update to that agent instead of editing the doc directly."
    }

    # Rule 3 (UNIVERSAL): reserved files are blocked for corpus-builder -- use the
    # registration-intent pattern instead of a direct edit.
    if ($agentType -eq 'corpus-builder') {
        $isReserved = ($rel -eq 'internal/app/server/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main)') -or
                      ($rel -match '^configs/[^/]+\.ya?ml$') -or
                      ($rel -match '^MCP/tool schemas')
        if ($isReserved) {
            Deny "WRITE-SCOPE-GUARD blocked $toolName on $rel`n`nThis is a reserved file (internal/app/server/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main), .nerd/config.json and internal/config, MCP/tool schemas**) -- direct builder edits here risk collisions across parallel work units. Use the registration-intent pattern instead: write your requested change to .corpus-build/intents/<WU>_intents.json and let the orchestrator's serial gate apply it."
        }
    }

    # Rule 4: slice-manifest scoping. No agent_type identified -> can't look up a
    # manifest, so stay permissive (rules 2-3 already applied).
    if ([string]::IsNullOrWhiteSpace($agentType)) { Allow }

    $manifestPath = Join-Path $RepoRoot ".corpus-build/slices/current/$agentType.json"
    if (-not (Test-Path -LiteralPath $manifestPath)) { Allow }

    $manifest = $null
    try {
        $manifest = Get-Content -LiteralPath $manifestPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    } catch {
        # Corrupt manifest -- fail open rather than lock the agent out entirely.
        Allow
    }
    if ($null -eq $manifest) { Allow }

    $allowedFiles = @()
    if ($null -ne $manifest.files) {
        $allowedFiles = @($manifest.files | ForEach-Object { ([string]$_) -replace '\\', '/' })
    }
    $wu = $manifest.work_unit
    if ([string]::IsNullOrWhiteSpace($wu)) { $wu = 'unknown' }

    if ($allowedFiles -contains $rel) { Allow }

    $listStr = if ($allowedFiles.Count -gt 0) { ($allowedFiles -join ', ') } else { '(empty list)' }
    Deny "WRITE-SCOPE-GUARD blocked $toolName on $rel`n`nAgent '$agentType' is scoped to work unit $wu. Allowed files: $listStr (plus anything under .corpus-build/). $rel is not on that list -- if it genuinely belongs to $wu, add it to .corpus-build/slices/current/$agentType.json; otherwise this write belongs to a different work unit or agent."

} catch {
    [Console]::Error.WriteLine("write-scope-guard hook error (fail-open): $($_.Exception.Message)")
    exit 0
}
