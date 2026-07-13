#Requires -Version 5.1
# corpus-token-meter.ps1 - SubagentStop hook, corpus-build fleet.
#
# PowerShell port of neurolog/.claude/hooks/skill-token-subagent.sh, adapted to
# the corpus-build ledger. Preserves every hard-won fix from that script:
#
#   * transcript_path on the SubagentStop event is the MAIN session
#     transcript, NOT the subagent's own. The subagent's BOUNDED transcript
#     lives at
#       <dirname(transcript_path)>/<session_id>/subagents/agent-<agent_id>.jsonl
#   * recency fallback: named teammates can carry an agent_id with no on-disk
#     correlate (their transcripts are agent-a<name>-<hash>.jsonl); fall back
#     to the newest agent-*.jsonl in that dir modified within 5 minutes.
#   * HARD GUARD: never sum the main session transcript. If resolution lands
#     on <session_id>.jsonl (or the main transcript's own basename), or
#     resolves to nothing at all, log the raw event to
#     .corpus-build/ledger/skipped_subagent_stops.jsonl and exit -- never guess.
#   * dedupe: one ledger row per transcript filename stem (first stop wins).
#     Released teammates re-fire SubagentStop on every idle ping and would
#     otherwise re-sum a growing cumulative transcript into near-duplicate rows.
#   * billable_total = output + input + cache_creation. cache_read is tracked
#     separately and excluded from the billable figure (heavily discounted).
#
# Gated on .corpus-build/ledger/<session_id>.active (same JSON file
# corpus-fleet-start.ps1 reads: {"run_id":"...","phase":"..."}). No active file
# = untracked dispatch -> exit 0, no directories created.
#
# Output: .corpus-build/ledger/token_runs.csv (header
# ts,run_id,phase,agent_type,agent_id,output,input,cache_creation,cache_read,billable_total)
# plus a JSONL mirror token_runs.jsonl with the same fields.
#
# Never blocks: exits 0 on every path.

$ErrorActionPreference = 'Stop'

function Read-StdinJson {
    $raw = [Console]::In.ReadToEnd()
    if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
    try { return ($raw | ConvertFrom-Json -ErrorAction Stop) } catch { return $null }
}

function Append-JsonlRaw([string]$path, [string]$json) {
    try {
        $dir = Split-Path -Parent $path
        if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
        Add-Content -LiteralPath $path -Value $json -Encoding UTF8
    } catch { }
}

try {
    $RepoRoot  = (Resolve-Path "$PSScriptRoot\..\..\..").Path
    $LedgerDir = Join-Path $RepoRoot '.corpus-build\ledger'

    $data = Read-StdinJson
    if ($null -eq $data) { exit 0 }

    $sessionId      = [string]$data.session_id
    $mainTranscript = [string]$data.transcript_path
    $agentId        = [string]$data.agent_id
    $agentType      = [string]$data.agent_type
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = [string]$data.subagent_type }

    if ([string]::IsNullOrWhiteSpace($sessionId)) { exit 0 }
    if ([string]::IsNullOrWhiteSpace($mainTranscript)) { exit 0 }

    # Gate: no active run for this session -> untracked dispatch, skip silently
    # (no directory created).
    $activePath = Join-Path $LedgerDir "$sessionId.active"
    if (-not (Test-Path -LiteralPath $activePath)) { exit 0 }

    $active = $null
    try { $active = Get-Content -LiteralPath $activePath -Raw -Encoding UTF8 -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop } catch { exit 0 }
    if ($null -eq $active) { exit 0 }
    $runId = [string]$active.run_id
    $phase = [string]$active.phase

    # ---- resolve the subagent's BOUNDED transcript -----------------------------
    $subDir = Join-Path (Split-Path -Parent $mainTranscript) "$sessionId\subagents"
    $transcript = $null

    if (-not [string]::IsNullOrWhiteSpace($agentId)) {
        $candidate = Join-Path $subDir "agent-$agentId.jsonl"
        if (Test-Path -LiteralPath $candidate) { $transcript = $candidate }
    }

    if ($null -eq $transcript -and (Test-Path -LiteralPath $subDir)) {
        $cutoff = (Get-Date).AddMinutes(-5)
        $newest = Get-ChildItem -LiteralPath $subDir -Filter 'agent-*.jsonl' -ErrorAction SilentlyContinue |
            Where-Object { $_.LastWriteTime -ge $cutoff } |
            Sort-Object LastWriteTime -Descending |
            Select-Object -First 1
        if ($null -ne $newest) { $transcript = $newest.FullName }
    }

    # Newer harness layout: subagent transcripts live under the session scratch
    # tasks dir as <agent_id>.output (JSONL), keyed by the project dir name
    # taken from the main transcript's parent (e.g. C--CodeProjects-codeNERD).
    if ($null -eq $transcript) {
        $projectLeaf = [System.IO.Path]::GetFileName((Split-Path -Parent $mainTranscript))
        $tasksDir = Join-Path $env:LOCALAPPDATA "Temp\claude\$projectLeaf\$sessionId\tasks"
        if (-not [string]::IsNullOrWhiteSpace($agentId)) {
            $candidate = Join-Path $tasksDir "$agentId.output"
            if ((Test-Path -LiteralPath $candidate) -and ((Get-Item -LiteralPath $candidate).Length -gt 0)) {
                $transcript = $candidate
            }
        }
    }

    $mainBase     = [System.IO.Path]::GetFileName($mainTranscript)
    $resolvedBase = if ($null -ne $transcript) { [System.IO.Path]::GetFileName($transcript) } else { $null }

    # HARD GUARD: never sum the main transcript, and never proceed on an
    # unresolved subagent transcript.
    if ($null -eq $transcript -or $resolvedBase -eq "$sessionId.jsonl" -or $resolvedBase -eq $mainBase) {
        $skipRecord = @{
            ts              = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ss.fffZ')
            reason          = 'subagent_transcript_unresolved'
            agent_id        = $agentId
            agent_type      = $agentType
            session_id      = $sessionId
            transcript_path = $mainTranscript
        }
        Append-JsonlRaw (Join-Path $LedgerDir 'skipped_subagent_stops.jsonl') ($skipRecord | ConvertTo-Json -Compress -Depth 6)
        exit 0
    }

    $stem = [System.IO.Path]::GetFileNameWithoutExtension($resolvedBase)
    if ($stem.StartsWith('agent-')) { $stem = $stem.Substring(6) }

    # ---- sum usage across the bounded transcript --------------------------------
    $out = [int64]0; $inp = [int64]0; $cc = [int64]0; $cr = [int64]0
    $lines = @()
    try { $lines = Get-Content -LiteralPath $transcript -ErrorAction Stop } catch { $lines = @() }

    foreach ($line in $lines) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        $obj = $null
        try { $obj = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
        if ($null -eq $obj) { continue }

        $usage = $null
        if ($null -ne $obj.message -and $null -ne $obj.message.usage) { $usage = $obj.message.usage }
        elseif ($null -ne $obj.usage) { $usage = $obj.usage }
        if ($null -eq $usage) { continue }

        if ($null -ne $usage.output_tokens) { $out += [int64]$usage.output_tokens }
        if ($null -ne $usage.input_tokens) { $inp += [int64]$usage.input_tokens }
        if ($null -ne $usage.cache_creation_input_tokens) { $cc += [int64]$usage.cache_creation_input_tokens }
        if ($null -ne $usage.cache_read_input_tokens) { $cr += [int64]$usage.cache_read_input_tokens }
    }

    # Nothing measured (empty/malformed transcript) -> skip a zero row.
    if ($out -eq 0 -and $inp -eq 0 -and $cc -eq 0) { exit 0 }

    $billableTotal = $out + $inp + $cc

    # ---- dedupe: one row per transcript stem, first stop wins ------------------
    $csvPath   = Join-Path $LedgerDir 'token_runs.csv'
    $jsonlPath = Join-Path $LedgerDir 'token_runs.jsonl'

    if (Test-Path -LiteralPath $csvPath) {
        try {
            $existing = Import-Csv -LiteralPath $csvPath -ErrorAction Stop
            if ($null -ne ($existing | Where-Object { $_.agent_id -eq $stem })) { exit 0 }
        } catch { }
    }

    $ts = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ss.fffZ')

    if (-not (Test-Path -LiteralPath $LedgerDir)) {
        New-Item -ItemType Directory -Path $LedgerDir -Force | Out-Null
    }
    if (-not (Test-Path -LiteralPath $csvPath)) {
        'ts,run_id,phase,agent_type,agent_id,output,input,cache_creation,cache_read,billable_total' |
            Set-Content -LiteralPath $csvPath -Encoding UTF8
    }
    $csvLine = '{0},{1},{2},{3},{4},{5},{6},{7},{8},{9}' -f $ts, $runId, $phase, $agentType, $stem, $out, $inp, $cc, $cr, $billableTotal
    Add-Content -LiteralPath $csvPath -Value $csvLine -Encoding UTF8

    $record = @{
        ts             = $ts
        run_id         = $runId
        phase          = $phase
        agent_type     = $agentType
        agent_id       = $stem
        output         = $out
        input          = $inp
        cache_creation = $cc
        cache_read     = $cr
        billable_total = $billableTotal
    }
    Append-JsonlRaw $jsonlPath ($record | ConvertTo-Json -Compress -Depth 6)

} catch {
    # Swallow -- telemetry hooks never block the fleet.
}
exit 0
