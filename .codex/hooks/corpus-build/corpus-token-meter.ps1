#Requires -Version 5.1
# Codex SubagentStop telemetry for the corpus-build fleet.
#
# Codex does not expose a stable bounded subagent transcript contract. This
# handler records the stop event and consumes explicit usage fields only when
# the hook payload provides them. It never infers totals from the parent
# transcript and never blocks the subagent lifecycle.

$ErrorActionPreference = 'Stop'

function Read-StdinJson {
    $reader = [IO.StreamReader]::new([Console]::OpenStandardInput())
    $raw = $reader.ReadToEnd()
    $reader.Dispose()
    if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
    try { return ($raw | ConvertFrom-Json -ErrorAction Stop) } catch { return $null }
}

function Read-UsageValue($usage, [string]$name) {
    if ($null -eq $usage) { return [int64]0 }
    $property = $usage.PSObject.Properties[$name]
    if ($null -eq $property -or $null -eq $property.Value) { return [int64]0 }
    try { return [int64]$property.Value } catch { return [int64]0 }
}

try {
    if (-not [string]::IsNullOrWhiteSpace($env:CODEX_HOOK_REPO_ROOT) -and
        (Test-Path -LiteralPath $env:CODEX_HOOK_REPO_ROOT -PathType Container)) {
        $repoRoot = (Resolve-Path -LiteralPath $env:CODEX_HOOK_REPO_ROOT).Path
    } else {
        $repoRoot = (Resolve-Path "$PSScriptRoot\..\..\..").Path
    }
    $ledgerDir = Join-Path $repoRoot '.corpus-build\ledger'
    $data = Read-StdinJson
    if ($null -eq $data) { exit 0 }

    $sessionId = [string]$data.session_id
    if ([string]::IsNullOrWhiteSpace($sessionId)) { exit 0 }

    $activePath = Join-Path $ledgerDir "$sessionId.active"
    if (-not (Test-Path -LiteralPath $activePath)) { exit 0 }

    try {
        $active = Get-Content -LiteralPath $activePath -Raw -Encoding UTF8 |
            ConvertFrom-Json -ErrorAction Stop
    } catch { exit 0 }

    $agentType = [string]$data.agent_type
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = [string]$data.subagent_type }
    $agentId = [string]$data.agent_id

    New-Item -ItemType Directory -Path $ledgerDir -Force | Out-Null
    $record = [ordered]@{
        ts                = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ss.fffZ')
        event             = 'stop'
        run_id            = [string]$active.run_id
        phase             = [string]$active.phase
        agent_type        = $agentType
        agent_id          = $agentId
        token_measurement = 'unavailable'
    }

    $usage = $data.usage
    if ($null -ne $usage) {
        $output = Read-UsageValue $usage 'output_tokens'
        $inputTokens = Read-UsageValue $usage 'input_tokens'
        $cacheCreation = Read-UsageValue $usage 'cache_creation_input_tokens'
        $cacheRead = Read-UsageValue $usage 'cache_read_input_tokens'
        if (($output + $inputTokens + $cacheCreation + $cacheRead) -gt 0) {
            $record['token_measurement'] = 'hook_payload'
            $record['output'] = $output
            $record['input'] = $inputTokens
            $record['cache_creation'] = $cacheCreation
            $record['cache_read'] = $cacheRead
            $record['billable_total'] = $output + $inputTokens + $cacheCreation
        }
    }

    $json = $record | ConvertTo-Json -Compress -Depth 6
    Add-Content -LiteralPath (Join-Path $ledgerDir 'fleet_events.jsonl') -Value $json -Encoding UTF8
} catch {
    # Telemetry must never block the fleet.
}

exit 0

