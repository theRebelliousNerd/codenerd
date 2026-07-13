#Requires -Version 5.1
# spec-context-injector.ps1 - PostToolUse hook (matcher: Read), corpus-build fleet.
#
# Dynamic spec injection (PLAN-corpus-build.md SS6): context follows the
# worker's actual reads instead of being front-loaded. Silent on every miss --
# degradation is always today's behavior (no injection), never a broken
# session.
#
# Loads Docs/architecture/roadmap/33_corpus_context_index.json. Missing index
# (not built yet) -> silent exit 0.
#
#   * Read of Docs/architecture/**/*.md:
#       - path IS in the index's `docs` map -> inject a one-line summary
#         (subsystem, doc-class, feature ids + status/plane).
#       - path is NOT in the index -> read the file's first line; if it is not
#         literally "---" (no frontmatter -> untagged), inject the tag-as-you-go
#         reminder. If it DOES have frontmatter, the index is merely stale --
#         stay silent.
#   * Read of internal/**/*.go: longest-prefix match against the index's
#     `packages` map -> inject owning subsystem, feature tags, wiring surfaces.
#   * Dedupe: per-session cache at $env:TEMP\corpus-injector-<session_id>.json
#     keyed on (agent_type, target). Emit each unique injection once.
#
# Fail-open: any parse error, missing field, or unreadable file exits 0 with no
# output -- this hook must never block or noisily fail a Read.

$ErrorActionPreference = 'Stop'

$RepoRoot  = (Resolve-Path "$PSScriptRoot\..\..\..").Path -replace '\\', '/'
$IndexPath = Join-Path $RepoRoot 'Docs/architecture/roadmap/33_corpus_context_index.json'

function Read-StdinJson {
    $raw = [Console]::In.ReadToEnd()
    if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
    try { return ($raw | ConvertFrom-Json -ErrorAction Stop) } catch { return $null }
}

function Get-Prop($obj, [string]$name) {
    if ($null -eq $obj) { return $null }
    $p = $obj.PSObject.Properties[$name]
    if ($null -eq $p) { return $null }
    return $p.Value
}

function ConvertTo-Arr($val) {
    if ($null -eq $val) { return @() }
    return @($val)
}

function Emit-Context([string]$text) {
    $payload = @{
        hookSpecificOutput = @{
            hookEventName     = 'PostToolUse'
            additionalContext = $text
        }
    }
    ($payload | ConvertTo-Json -Depth 6 -Compress) | Write-Output
}

function Normalize-RelPath([string]$filePath) {
    if ([string]::IsNullOrWhiteSpace($filePath)) { return $null }
    $norm = $filePath -replace '\\', '/'
    if ($norm.ToLower().StartsWith($RepoRoot.ToLower() + '/')) {
        return $norm.Substring($RepoRoot.Length + 1)
    }
    return ($norm -replace '^\./', '')
}

function Load-DedupeCache([string]$path) {
    $set = New-Object 'System.Collections.Generic.HashSet[string]'
    if (Test-Path -LiteralPath $path) {
        try {
            $raw = Get-Content -LiteralPath $path -Raw -ErrorAction Stop
            if (-not [string]::IsNullOrWhiteSpace($raw)) {
                $arr = $raw | ConvertFrom-Json -ErrorAction Stop
                foreach ($k in (ConvertTo-Arr $arr)) { [void]$set.Add([string]$k) }
            }
        } catch { }
    }
    # Comma operator is load-bearing: without it, PowerShell enumerates the
    # HashSet onto the pipeline instead of returning it as one object -- an
    # empty set becomes $null and a non-empty set becomes a plain string[]
    # (whose .Add() then throws, and whose lookup identity is no longer the
    # cache). Disk-verified 2026-07-08: this silently broke every dedupe path.
    return ,$set
}

function Save-DedupeCache([string]$path, $set) {
    try {
        $arr = @($set)
        ($arr | ConvertTo-Json -Compress) | Set-Content -LiteralPath $path -Encoding UTF8
    } catch { }
}

try {
    if (-not (Test-Path -LiteralPath $IndexPath)) { exit 0 }

    $data = Read-StdinJson
    if ($null -eq $data) { exit 0 }

    $toolName = $data.tool_name
    if ($null -ne $toolName -and $toolName -ne 'Read') { exit 0 }

    $filePath = $data.tool_input.file_path
    $rel = Normalize-RelPath $filePath
    if ([string]::IsNullOrWhiteSpace($rel)) { exit 0 }

    $isDoc = ($rel -match '^Docs/architecture/.*\.md$')
    $isGo  = ($rel -match '^internal/.*\.go$')
    if (-not $isDoc -and -not $isGo) { exit 0 }

    $index = $null
    try {
        $index = Get-Content -LiteralPath $IndexPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    } catch { exit 0 }
    if ($null -eq $index) { exit 0 }

    $agentType = $data.agent_type
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = $data.subagent_type }
    if ([string]::IsNullOrWhiteSpace($agentType)) { $agentType = 'unknown' }

    $sessionId = $data.session_id
    if ([string]::IsNullOrWhiteSpace($sessionId)) { $sessionId = 'nosession' }

    $tempDir = $env:TEMP
    if ([string]::IsNullOrWhiteSpace($tempDir)) { $tempDir = [System.IO.Path]::GetTempPath() }
    $cachePath = Join-Path $tempDir "corpus-injector-$sessionId.json"

    $message   = $null
    $targetKey = $null

    if ($isDoc) {
        $targetKey = $rel
        $docsMap = Get-Prop $index 'docs'
        $entry   = Get-Prop $docsMap $rel

        if ($null -ne $entry) {
            $subsystem = Get-Prop $entry 'subsystem'
            $docClass  = Get-Prop $entry 'doc-class'
            $features  = ConvertTo-Arr (Get-Prop $entry 'features')

            $featStr = ''
            if ($features.Count -gt 0) {
                $featuresMap = Get-Prop $index 'features'
                $parts = @()
                foreach ($fid in $features) {
                    $fentry = Get-Prop $featuresMap $fid
                    $status = if ($null -ne $fentry) { Get-Prop $fentry 'status' } else { $null }
                    $plane  = if ($null -ne $fentry) { Get-Prop $fentry 'plane' } else { $null }
                    $tag = "$fid"
                    if ($status -or $plane) { $tag += " ($status/$plane)" }
                    $parts += $tag
                }
                $featStr = ($parts -join ', ')
            }

            $message = "[corpus-context] $rel -- subsystem=$subsystem, doc-class=$docClass"
            if ($featStr) { $message += ", features: $featStr" }
        } else {
            # Not indexed -- check the raw file directly for frontmatter.
            $firstLine = $null
            $readOk = $true
            try { $firstLine = Get-Content -LiteralPath $filePath -TotalCount 1 -ErrorAction Stop } catch { $readOk = $false }
            if ($readOk) {
                $tagged = ($null -ne $firstLine -and $firstLine.Trim() -eq '---')
                if (-not $tagged) {
                    $message = "[corpus-context] This architecture doc is untagged. Before completing your current task, stamp schema-conformant YAML frontmatter (see Docs/architecture/roadmap/FEATURE_TAGGING_SCHEMA.md) -- tag-as-you-go policy."
                }
            }
        }
    } elseif ($isGo) {
        $pkgMap = Get-Prop $index 'packages'
        if ($null -ne $pkgMap) {
            $bestKey = $null
            foreach ($prop in $pkgMap.PSObject.Properties) {
                $normKey = $prop.Name.TrimEnd('/') + '/'
                if ($rel.StartsWith($normKey)) {
                    if ($null -eq $bestKey -or $normKey.Length -gt $bestKey.Length) { $bestKey = $normKey }
                }
            }
            if ($null -ne $bestKey) {
                $targetKey = $bestKey
                $entry = Get-Prop $pkgMap $bestKey
                if ($null -eq $entry) { $entry = Get-Prop $pkgMap ($bestKey.TrimEnd('/')) }

                if ($null -ne $entry) {
                    $subsystem = Get-Prop $entry 'subsystem'
                    $features  = ConvertTo-Arr (Get-Prop $entry 'features')
                    $surfaces  = ConvertTo-Arr (Get-Prop $entry 'surfaces')

                    $message = "[corpus-context] $bestKey -- subsystem=$subsystem"
                    if ($features.Count -gt 0) { $message += "; features: $($features -join ', ')" }
                    if ($surfaces.Count -gt 0) { $message += "; surfaces: $($surfaces -join ', ')" }
                }
            }
        }
    }

    if ([string]::IsNullOrWhiteSpace($message) -or [string]::IsNullOrWhiteSpace($targetKey)) { exit 0 }

    $dedupeKey = "$agentType|$targetKey"
    $cache = Load-DedupeCache $cachePath
    if ($cache.Contains($dedupeKey)) { exit 0 }

    [void]$cache.Add($dedupeKey)
    Save-DedupeCache $cachePath $cache

    Emit-Context $message
    exit 0

} catch {
    exit 0
}
