#Requires -Version 5.1
# spec-attribution-check.ps1 - PostToolUse hook (matcher: Write|Edit), corpus-build fleet.
#
# WARN-level only -- this hook never blocks. When a NEW .go file under
# internal/ (git doesn't track it yet) is written without a
# `// SPEC: <doc>#<section>` attribution line anywhere in its content, emit a
# reminder via additionalContext so the doc<->code link stays traceable.
# _test.go files are exempt (tests aren't the spec-attributed artifact).
#
# Fail-open: any parse error, missing git, or unreadable file exits 0 with no
# output.

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path "$PSScriptRoot\..\..\..").Path -replace '\\', '/'

function Read-StdinJson {
    $raw = [Console]::In.ReadToEnd()
    if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
    try { return ($raw | ConvertFrom-Json -ErrorAction Stop) } catch { return $null }
}

function Emit-Context([string]$text) {
    $payload = @{
        hookSpecificOutput = @{
            hookEventName     = 'PostToolUse'
            additionalContext = $text
        }
    }
    try { ($payload | ConvertTo-Json -Depth 6 -Compress) | Write-Output } catch { }
}

function Normalize-RelPath([string]$filePath) {
    if ([string]::IsNullOrWhiteSpace($filePath)) { return $null }
    $norm = $filePath -replace '\\', '/'
    if ($norm.ToLower().StartsWith($RepoRoot.ToLower() + '/')) {
        return $norm.Substring($RepoRoot.Length + 1)
    }
    return ($norm -replace '^\./', '')
}

function Test-IsNewFile([string]$absPath) {
    # "New" = git doesn't track this path yet. `git ls-files --error-unmatch`
    # exits non-zero (and writes a "pathspec did not match" line to stderr)
    # for untracked paths -- that is the EXPECTED, common case here, not a
    # failure. Function-scoped override (does not leak to the caller): under
    # $ErrorActionPreference = 'Stop', Windows PowerShell 5.1 promotes that
    # expected stderr line to a terminating RemoteException even with `*>
    # $null` redirection, which would make every untracked file look like an
    # unresolvable error. 'Continue' lets the redirect actually swallow it;
    # the real signal is $LASTEXITCODE, read right after.
    $ErrorActionPreference = 'Continue'
    try {
        & git -C $RepoRoot ls-files --error-unmatch -- $absPath *> $null
        return ($LASTEXITCODE -ne 0)
    } catch {
        return $false
    }
}

try {
    $data = Read-StdinJson
    if ($null -eq $data) { exit 0 }

    $toolName = $data.tool_name
    if ($null -ne $toolName -and $toolName -notin @('Write', 'Edit')) { exit 0 }

    $filePath = $data.tool_input.file_path
    $rel = Normalize-RelPath $filePath
    if ([string]::IsNullOrWhiteSpace($rel)) { exit 0 }

    if ($rel -notmatch '^internal/.*\.go$') { exit 0 }
    if ($rel -match '_test\.go$') { exit 0 }

    $abs = Join-Path $RepoRoot $rel
    if (-not (Test-Path -LiteralPath $abs)) { exit 0 }

    if (-not (Test-IsNewFile $abs)) { exit 0 }

    $content = $null
    try { $content = Get-Content -LiteralPath $abs -Raw -ErrorAction Stop } catch { exit 0 }
    if ($null -eq $content) { exit 0 }

    if ($content -match '//\s*SPEC:') { exit 0 }

    Emit-Context "[corpus-context] $rel is a new spec-driven file with no '// SPEC: <doc>#<section>' attribution. Add one near the symbol it implements (e.g. // SPEC: Docs/architecture/graph/IMPLEMENTED_SPEC.md#4-data-model) so the doc<->code link stays traceable."
    exit 0

} catch {
    exit 0
}
