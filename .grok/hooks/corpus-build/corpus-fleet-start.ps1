#Requires -Version 5.1
# corpus-fleet-start.ps1 - SubagentStart hook, corpus-build fleet.
#
# Appends a dispatch row to .corpus-build/ledger/fleet_events.jsonl, keyed to
# the active run stamped by the orchestrator at
# .corpus-build/ledger/<session_id>.active (shape:
# {"run_id":"...","phase":"...","skill":"corpus-build"}). No active file =
# untracked dispatch (an agent spun up outside a corpus-build run) -> skip
# silently, no directory created. Mirrors the telemetry-only contract of
# .claude/hooks/roadmap-grinder/subagent-start.ps1.
#
# Never blocks: exits 0 on every path.

$ErrorActionPreference = 'Stop'

function Read-StdinJson {
    $raw = [Console]::In.ReadToEnd()
    if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
    try { return ($raw | ConvertFrom-Json -ErrorAction Stop) } catch { return $null }
}

try {
    $RepoRoot  = (Resolve-Path "$PSScriptRoot\..\..\..").Path
    $LedgerDir = Join-Path $RepoRoot '.corpus-build\ledger'

    $data = Read-StdinJson
    if ($null -eq $data) { exit 0 }

    $sessionId = [string]$data.session_id
    if ([string]::IsNullOrWhiteSpace($sessionId)) { exit 0 }

    $activePath = Join-Path $LedgerDir "$sessionId.active"
    if (-not (Test-Path -LiteralPath $activePath)) { exit 0 }

    $active = $null
    try { $active = Get-Content -LiteralPath $activePath -Raw -Encoding UTF8 -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop } catch { exit 0 }
    if ($null -eq $active) { exit 0 }

    $runId = [string]$active.run_id
    $phase = [string]$active.phase

    $agentType = [string]$data.agent_type
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = [string]$data.subagent_type }
    $agentId = [string]$data.agent_id

    if (-not (Test-Path -LiteralPath $LedgerDir)) {
        New-Item -ItemType Directory -Path $LedgerDir -Force | Out-Null
    }

    $record = @{
        ts         = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ss.fffZ')
        event      = 'start'
        run_id     = $runId
        phase      = $phase
        agent_type = $agentType
        agent_id   = $agentId
    }
    $json = ($record | ConvertTo-Json -Compress -Depth 6)
    Add-Content -LiteralPath (Join-Path $LedgerDir 'fleet_events.jsonl') -Value $json -Encoding UTF8

} catch {
    # Swallow -- telemetry hooks never block the fleet.
}
exit 0
