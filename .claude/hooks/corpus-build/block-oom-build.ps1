#Requires -Version 5.1
# block-oom-build.ps1 - PreToolUse hook (matcher: Bash|PowerShell), corpus-build fleet.
#
# cmd/nerd has frozen this 128 GB machine TWICE on a host
# `go build`/`go test`/`go vet`. Any command whose target includes that package,
# or a whole-tree build/test/vet from the repo root (`./...`, which transitively
# compiles handlers), is blocked with a message pointing at the Docker compile
# path. See agent memory handlers_package_host_build_oom and
# PLAN-corpus-build.md SS5 (Layer 2 hook table).
#
# Docker commands always pass -- that IS the required route, not a bypass.
#
# Escape hatch (orchestrator-managed only): a marker file
#   <cwd>/.corpus-build/.compile-grant-<agent_type>
# lifts the block for that agent_type. The orchestrator creates/deletes this
# file; workers must never create their own grant.
#
# Fail-open: any parse error, missing field, or unexpected shape allows the
# command through silently (exit 0). This hook narrows an already-permissioned
# command; it must never be the reason a session breaks.

$ErrorActionPreference = 'Stop'

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

try {
    $data = Read-StdinJson
    if ($null -eq $data) { Allow }

    $toolName = $data.tool_name
    if ($null -ne $toolName -and $toolName -notmatch '^(Bash|PowerShell)$') { Allow }

    $command = $data.tool_input.command
    if ([string]::IsNullOrWhiteSpace($command)) { Allow }
    $cmd = [string]$command

    $agentType = $data.agent_type
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = $data.subagent_type }
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = 'orchestrator' }

    $cwd = $data.cwd
    if ([string]::IsNullOrWhiteSpace($cwd)) { $cwd = (Get-Location).Path }

    # Escape hatch: orchestrator-managed compile grant, scoped to this agent_type.
    $grantPath = Join-Path $cwd ".corpus-build/.compile-grant-$agentType"
    if (Test-Path -LiteralPath $grantPath) { Allow }

    # scoped package paths always pass -- that IS the required compile route for the OOM package.
    if ($cmd -match '(?im)(^|[;&|]\s*)\s*(docker(\s|-compose)|make\s+docker-)') { Allow }

    $goVerbPattern = '(?im)\bgo\s+(build|test|vet)\b'
    if ($cmd -notmatch $goVerbPattern) { Allow }

    # Scope the check per shell sub-command so an unrelated segment (e.g. `cd x && docker ...`)
    # sharing a command line with an innocuous `go build ./cmd/foo` doesn't get flagged.
    $segments = [regex]::Split($cmd, '(?:&&|\|\||;|\r?\n|\|)')

    foreach ($seg in $segments) {
        if ($seg -notmatch $goVerbPattern) { continue }

        $norm = $seg -replace '\\', '/'
        $targetsHandlers = $norm -match 'cmd/nerd'
        $wholeTree = ($norm -match '(?<!\S)\./\.\.\.(?!\S)') -or ($norm -match '(?<!\S)\.\.\.(?!\S)')

        if ($targetsHandlers -or $wholeTree) {
            $reasonKind = if ($targetsHandlers) { 'targets cmd/nerd' } else { 'whole-tree build/test/vet (./...) from the repo root' }
            $reason = @"
BLOCK-OOM-BUILD blocked this command -- $reasonKind

  Segment: $($seg.Trim())

cmd/nerd OOMs a host go build/go test/go vet -- it has
frozen this 128 GB machine TWICE. Compilation for this package (and any
whole-tree build that transitively includes it) must go through the Docker
compile path instead:
  make docker-build
  (or run the compile inside the host go test/build (scoped packages) directly)

Escape hatch (orchestrator-managed ONLY -- do not create this yourself unless
you ARE the orchestrator): create
  .corpus-build/.compile-grant-$agentType
in the repo root to lift this block for this agent_type, then delete it when
the grant is no longer needed.
"@
            Deny $reason
        }
    }

    Allow

} catch {
    [Console]::Error.WriteLine("block-oom-build hook error (fail-open): $($_.Exception.Message)")
    exit 0
}
