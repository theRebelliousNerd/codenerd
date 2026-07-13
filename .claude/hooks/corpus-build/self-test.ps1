#Requires -Version 5.1
# self-test.ps1 - dry-run harness for the corpus-build fleet hooks.
#
# Feeds each hook synthetic stdin JSON fixtures (block/inject case, allow/silent
# case, malformed case) and asserts exit codes + stdout shape. Every hook calls
# `exit` directly, so each invocation MUST run as a real child process --
# dot-sourcing or `&`-invoking in this process would terminate the harness on
# the first exit. Every scratch artifact this harness creates is cleaned up
# surgically (marker-scoped line/row removal, not whole-file deletes) so a
# concurrent real corpus-build run's ledger rows are never clobbered.
#
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/self-test.ps1
# Exit 0 = every case passed (look for one "PASS: ..." line per case).
# Exit 1 = at least one case failed (see the "FAIL: ..." lines for why).

$ErrorActionPreference = 'Stop'

$HooksDir = $PSScriptRoot
$RepoRoot = (Resolve-Path "$HooksDir\..\..\..").Path
$Marker   = 'selftest-' + [guid]::NewGuid().ToString('N').Substring(0, 10)

$script:PassCount = 0
$script:FailCount = 0

function Invoke-Hook {
    param(
        [Parameter(Mandatory)][string]$ScriptPath,
        [Parameter(Mandatory)][AllowEmptyString()][string]$StdinText
    )

    # IMPORTANT (disk-verified 2026-07-08): driving a nested `powershell.exe
    # -File` child via System.Diagnostics.Process's StandardInput pipe
    # delivers BOM/encoding-mangled bytes in this environment -- every
    # "expects deny/inject" fixture silently degraded to the malformed-input
    # fail-open path (ConvertFrom-Json choked on a stray leading character)
    # while "expects allow/silent" fixtures vacuously passed regardless of
    # what was actually sent. Routing stdin through a temp file and
    # PowerShell's own pipeline operator (`Get-Content -Raw | & powershell
    # ...`) is the reliable path -- confirmed byte-for-byte correct across
    # repeated runs.
    $stdinFile  = Join-Path $env:TEMP ('corpus-selftest-stdin-' + [guid]::NewGuid().ToString('N') + '.json')
    $stderrFile = Join-Path $env:TEMP ('corpus-selftest-stderr-' + [guid]::NewGuid().ToString('N') + '.txt')

    try {
        [System.IO.File]::WriteAllText($stdinFile, $StdinText, (New-Object System.Text.UTF8Encoding($false)))

        # Function-scoped override (does not leak to the caller): under
        # $ErrorActionPreference = 'Stop', PowerShell 5.1 promotes a native
        # command's stderr lines to a terminating NativeCommandError even
        # though `2> $stderrFile` redirects them to disk. 'Continue' lets the
        # redirect do its job; the hook's actual exit code is still read from
        # $LASTEXITCODE afterward.
        $ErrorActionPreference = 'Continue'
        $stdoutRaw = Get-Content -LiteralPath $stdinFile -Raw | & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $ScriptPath 2> $stderrFile
        $exitCode = $LASTEXITCODE

        $stdout = if ($null -ne $stdoutRaw) { ($stdoutRaw -join "`n") } else { '' }
        $stderr = ''
        if (Test-Path -LiteralPath $stderrFile) {
            $stderrContent = Get-Content -LiteralPath $stderrFile -Raw
            if ($null -ne $stderrContent) { $stderr = $stderrContent }
        }

        return @{ ExitCode = $exitCode; Stdout = $stdout; Stderr = $stderr }
    } finally {
        Remove-Item -LiteralPath $stdinFile -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $stderrFile -Force -ErrorAction SilentlyContinue
    }
}

function Test-Case {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$ScriptPath,
        [Parameter(Mandatory)]$Payload,
        [Parameter(Mandatory)][int]$ExpectedExit,
        [scriptblock]$StdoutCheck = $null,
        [scriptblock]$PostCheck   = $null
    )

    $stdinText = if ($Payload -is [string]) { $Payload } else { ($Payload | ConvertTo-Json -Depth 10 -Compress) }

    $result = Invoke-Hook -ScriptPath $ScriptPath -StdinText $stdinText

    $ok = ($result.ExitCode -eq $ExpectedExit)
    $reason = ''
    if (-not $ok) { $reason = "expected exit $ExpectedExit, got $($result.ExitCode) (stderr: $($result.Stderr.Trim()))" }

    if ($ok -and $null -ne $StdoutCheck) {
        $stdoutOk = & $StdoutCheck $result.Stdout
        if (-not $stdoutOk) { $ok = $false; $reason = "stdout shape check failed (stdout: '$($result.Stdout.Trim())')" }
    }

    if ($ok -and $null -ne $PostCheck) {
        $postOk = & $PostCheck
        if (-not $postOk) { $ok = $false; $reason = 'post-condition check failed' }
    }

    if ($ok) {
        $script:PassCount++
        Write-Output "PASS: $Name"
    } else {
        $script:FailCount++
        Write-Output "FAIL: $Name -- $reason"
    }
}

# Shared ledger dir (used by corpus-fleet-start.ps1 and corpus-token-meter.ps1
# fixtures) -- tracked once so final cleanup only removes it if THIS run
# created it and it is empty afterward.
$LedgerDir        = Join-Path $RepoRoot '.corpus-build\ledger'
$LedgerPreExisted = Test-Path -LiteralPath $LedgerDir

# Declared up front (not just inside section 3) so the `finally` block's
# context-index restore is well-defined even if an earlier section throws
# before section 3 runs.
$indexPath       = $null
$indexPreExisted = $false
$indexBackupPath = $null

try {

    # =========================================================================
    # 1. block-oom-build.ps1
    # =========================================================================
    $boomScript = Join-Path $HooksDir 'block-oom-build.ps1'

    $grantCwd   = Join-Path $env:TEMP "corpus-build-$Marker-cwd"
    $grantAgent = "selftest-$Marker"
    New-Item -ItemType Directory -Path (Join-Path $grantCwd '.corpus-build') -Force | Out-Null
    New-Item -ItemType File -Path (Join-Path $grantCwd ".corpus-build/.compile-grant-$grantAgent") -Force | Out-Null

    Test-Case -Name 'block-oom-build: blocks handlers-targeted go test' -ScriptPath $boomScript -ExpectedExit 2 `
        -Payload @{ tool_name = 'Bash'; tool_input = @{ command = 'go test ./cmd/nerd/...' }; agent_type = 'corpus-builder'; session_id = $Marker; cwd = $RepoRoot } `
        -StdoutCheck { param($s) $s -match '"permissionDecision":"deny"' }

    Test-Case -Name 'block-oom-build: blocks whole-tree go build ./...' -ScriptPath $boomScript -ExpectedExit 2 `
        -Payload @{ tool_name = 'Bash'; tool_input = @{ command = 'go build ./...' }; agent_type = 'corpus-builder'; session_id = $Marker; cwd = $RepoRoot } `
        -StdoutCheck { param($s) $s -match '"permissionDecision":"deny"' }

    Test-Case -Name 'block-oom-build: allows a scoped go build' -ScriptPath $boomScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Bash'; tool_input = @{ command = 'go build ./cmd/codenerd/' }; agent_type = 'corpus-builder'; session_id = $Marker; cwd = $RepoRoot }

    Test-Case -Name 'block-oom-build: allows a docker build command' -ScriptPath $boomScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Bash'; tool_input = @{ command = 'make docker-build' }; agent_type = 'corpus-builder'; session_id = $Marker; cwd = $RepoRoot }

    Test-Case -Name 'block-oom-build: escape-hatch grant lifts the block' -ScriptPath $boomScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Bash'; tool_input = @{ command = 'go test ./cmd/nerd/...' }; agent_type = $grantAgent; session_id = $Marker; cwd = $grantCwd }

    Test-Case -Name 'block-oom-build: malformed JSON fails open' -ScriptPath $boomScript -ExpectedExit 0 -Payload '{not valid json'
    Test-Case -Name 'block-oom-build: empty stdin fails open' -ScriptPath $boomScript -ExpectedExit 0 -Payload ''

    Remove-Item -LiteralPath $grantCwd -Recurse -Force -ErrorAction SilentlyContinue


    # =========================================================================
    # 2. write-scope-guard.ps1
    # =========================================================================
    $wsgScript = Join-Path $HooksDir 'write-scope-guard.ps1'

    Test-Case -Name 'write-scope-guard: blocks architecture-corpus write for non-doc-auditor' -ScriptPath $wsgScript -ExpectedExit 2 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = (Join-Path $RepoRoot 'Docs/architecture/graph/IMPLEMENTED_SPEC.md') }; agent_type = 'corpus-builder'; session_id = $Marker } `
        -StdoutCheck { param($s) $s -match '"permissionDecision":"deny"' }

    Test-Case -Name 'write-scope-guard: allows architecture-corpus write for corpus-doc-auditor' -ScriptPath $wsgScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = (Join-Path $RepoRoot 'Docs/architecture/graph/IMPLEMENTED_SPEC.md') }; agent_type = 'corpus-doc-auditor'; session_id = $Marker }

    Test-Case -Name 'write-scope-guard: blocks a reserved config file for corpus-builder' -ScriptPath $wsgScript -ExpectedExit 2 `
        -Payload @{ tool_name = 'Edit'; tool_input = @{ file_path = (Join-Path $RepoRoot 'configs/development.yaml') }; agent_type = 'corpus-builder'; session_id = $Marker }

    Test-Case -Name 'write-scope-guard: allows .corpus-build writes for any agent' -ScriptPath $wsgScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = (Join-Path $RepoRoot ".corpus-build/results/$Marker.json") }; agent_type = 'corpus-builder'; session_id = $Marker }

    $manifestAgent    = "selftest-manifest-$Marker"
    $slicesCurrentDir = Join-Path $RepoRoot '.corpus-build\slices\current'
    $slicesPreExisted = Test-Path -LiteralPath $slicesCurrentDir
    New-Item -ItemType Directory -Path $slicesCurrentDir -Force | Out-Null
    $manifestPath = Join-Path $slicesCurrentDir "$manifestAgent.json"
    (@{ work_unit = 'WU-999'; files = @('internal/selftest/foo.go') } | ConvertTo-Json) | Set-Content -LiteralPath $manifestPath -Encoding UTF8

    Test-Case -Name 'write-scope-guard: manifest blocks a file not on the slice list' -ScriptPath $wsgScript -ExpectedExit 2 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = (Join-Path $RepoRoot 'internal/selftest/other.go') }; agent_type = $manifestAgent; session_id = $Marker }

    Test-Case -Name 'write-scope-guard: manifest allows a listed file' -ScriptPath $wsgScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = (Join-Path $RepoRoot 'internal/selftest/foo.go') }; agent_type = $manifestAgent; session_id = $Marker }

    Test-Case -Name 'write-scope-guard: malformed JSON fails open' -ScriptPath $wsgScript -ExpectedExit 0 -Payload '{not valid json'
    Test-Case -Name 'write-scope-guard: empty stdin fails open' -ScriptPath $wsgScript -ExpectedExit 0 -Payload ''

    Remove-Item -LiteralPath $manifestPath -Force -ErrorAction SilentlyContinue
    if (-not $slicesPreExisted) {
        Remove-Item -LiteralPath (Join-Path $RepoRoot '.corpus-build\slices') -Recurse -Force -ErrorAction SilentlyContinue
    }


    # =========================================================================
    # 3. spec-context-injector.ps1
    # =========================================================================
    $injectorScript = Join-Path $HooksDir 'spec-context-injector.ps1'
    $indexPath        = Join-Path $RepoRoot 'Docs/architecture/roadmap\33_corpus_context_index.json'
    $indexPreExisted  = Test-Path -LiteralPath $indexPath
    if ($indexPreExisted) {
        # Concurrent-agent safety: this path is also the REAL production
        # index (W5's build_tag_index.py output) that another agent may own.
        # spec-context-injector.ps1 hardcodes this path with no cwd/override,
        # so this section must write fixture content here -- back up the
        # exact original bytes now and restore them verbatim in the script's
        # `finally` block (not just "delete if we created it") so a
        # concurrent run's real index is never left clobbered, even if a
        # later test case in this run throws.
        $indexBackupPath = Join-Path $env:TEMP "corpus-selftest-index-backup-$Marker.json"
        Copy-Item -LiteralPath $indexPath -Destination $indexBackupPath -Force
        # The "index missing" behavior needs the path to be GENUINELY absent
        # to test truthfully (not skipped) even when a concurrent agent has
        # already shipped the real index -- remove it now; it's restored from
        # $indexBackupPath in the `finally` block regardless of how this
        # section exits.
        Remove-Item -LiteralPath $indexPath -Force -ErrorAction SilentlyContinue
    }

    $sessNoIndex  = "$Marker-noindex"
    $sessInject   = "$Marker-inject"
    $sessUntagged = "$Marker-untagged"
    $sessTagged   = "$Marker-tagged"
    $sessPkg      = "$Marker-pkg"
    $sessPkgMiss  = "$Marker-pkgmiss"

    Test-Case -Name 'spec-context-injector: silent when the context index is missing' -ScriptPath $injectorScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Read'; tool_input = @{ file_path = (Join-Path $RepoRoot 'internal/graph/service.go') }; agent_type = 'selftest-agentZ'; session_id = $sessNoIndex } `
        -StdoutCheck { param($s) [string]::IsNullOrWhiteSpace($s) }

    $indexBody = @{
        docs     = @{
            'Docs/architecture/graph/IMPLEMENTED_SPEC.md' = @{ subsystem = 'graph'; 'doc-class' = 'shipped'; features = @('GR-001'); 'last-verified' = '2026-07-08' }
        }
        features = @{
            'GR-001' = @{ topic = 'traversal'; plane = 'system1'; status = 'shipped'; owner_doc = 'Docs/architecture/graph/IMPLEMENTED_SPEC.md'; source_paths = @() }
        }
        packages = @{
            'internal/graph/' = @{ subsystem = 'graph'; features = @('GR-001'); surfaces = @('rest', 'mcp') }
        }
    }
    New-Item -ItemType Directory -Path (Split-Path -Parent $indexPath) -Force | Out-Null
    ($indexBody | ConvertTo-Json -Depth 10) | Set-Content -LiteralPath $indexPath -Encoding UTF8

    $scratchDocDir = Join-Path $RepoRoot "Docs/architecture/_selftest_scratch_$Marker"
    New-Item -ItemType Directory -Path $scratchDocDir -Force | Out-Null
    $untaggedDoc = Join-Path $scratchDocDir 'untagged.md'
    Set-Content -LiteralPath $untaggedDoc -Value @('# no frontmatter here', 'Some text.') -Encoding UTF8
    $taggedDoc = Join-Path $scratchDocDir 'tagged.md'
    Set-Content -LiteralPath $taggedDoc -Value @('---', 'title: x', '---', '# body') -Encoding UTF8

    $indexedDocAbs = Join-Path $RepoRoot 'Docs/architecture/graph\IMPLEMENTED_SPEC.md'

    Test-Case -Name 'spec-context-injector: injects a summary for an indexed doc' -ScriptPath $injectorScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Read'; tool_input = @{ file_path = $indexedDocAbs }; agent_type = 'selftest-agentA'; session_id = $sessInject } `
        -StdoutCheck { param($s) ($s -match '\[corpus-context\]') -and ($s -match 'subsystem=graph') }

    Test-Case -Name 'spec-context-injector: dedupes a repeated injection for the same agent+doc' -ScriptPath $injectorScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Read'; tool_input = @{ file_path = $indexedDocAbs }; agent_type = 'selftest-agentA'; session_id = $sessInject } `
        -StdoutCheck { param($s) [string]::IsNullOrWhiteSpace($s) }

    Test-Case -Name 'spec-context-injector: injects the untagged reminder for an unindexed doc with no frontmatter' -ScriptPath $injectorScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Read'; tool_input = @{ file_path = $untaggedDoc }; agent_type = 'selftest-agentB'; session_id = $sessUntagged } `
        -StdoutCheck { param($s) ($s -match 'untagged') -and ($s -match 'FEATURE_TAGGING_SCHEMA') }

    Test-Case -Name 'spec-context-injector: silent for an unindexed doc that already has frontmatter' -ScriptPath $injectorScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Read'; tool_input = @{ file_path = $taggedDoc }; agent_type = 'selftest-agentB2'; session_id = $sessTagged } `
        -StdoutCheck { param($s) [string]::IsNullOrWhiteSpace($s) }

    Test-Case -Name 'spec-context-injector: injects a package summary via longest-prefix match' -ScriptPath $injectorScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Read'; tool_input = @{ file_path = (Join-Path $RepoRoot 'internal/graph/service.go') }; agent_type = 'selftest-agentC'; session_id = $sessPkg } `
        -StdoutCheck { param($s) ($s -match 'internal/graph/') -and ($s -match 'surfaces: rest, mcp') }

    Test-Case -Name 'spec-context-injector: silent for a go file with no package match' -ScriptPath $injectorScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Read'; tool_input = @{ file_path = (Join-Path $RepoRoot 'internal/totallyunknownpkg12345/file.go') }; agent_type = 'selftest-agentD'; session_id = $sessPkgMiss } `
        -StdoutCheck { param($s) [string]::IsNullOrWhiteSpace($s) }

    Test-Case -Name 'spec-context-injector: malformed JSON fails open' -ScriptPath $injectorScript -ExpectedExit 0 -Payload '{not valid json'

    Remove-Item -LiteralPath $scratchDocDir -Recurse -Force -ErrorAction SilentlyContinue
    # Index file restore/removal happens in the script's `finally` block (see
    # $indexPreExisted / $indexBackupPath above) so it still runs even if a
    # later section throws.
    foreach ($s in @($sessNoIndex, $sessInject, $sessUntagged, $sessTagged, $sessPkg, $sessPkgMiss)) {
        Remove-Item -LiteralPath (Join-Path $env:TEMP "corpus-injector-$s.json") -Force -ErrorAction SilentlyContinue
    }


    # =========================================================================
    # 4. spec-attribution-check.ps1
    # =========================================================================
    $attrScript = Join-Path $HooksDir 'spec-attribution-check.ps1'

    $attrScratchDir = Join-Path $RepoRoot "internal\_selftest_scratch_$Marker"
    New-Item -ItemType Directory -Path $attrScratchDir -Force | Out-Null

    $noAttrFile = Join-Path $attrScratchDir 'no_attr.go'
    Set-Content -LiteralPath $noAttrFile -Value @('package selftest', '', 'func Foo() {}') -Encoding UTF8

    $withAttrFile = Join-Path $attrScratchDir 'with_attr.go'
    Set-Content -LiteralPath $withAttrFile -Value @('package selftest', '', '// SPEC: Docs/architecture/graph/IMPLEMENTED_SPEC.md#4-data-model', 'func Bar() {}') -Encoding UTF8

    $testFile = Join-Path $attrScratchDir 'no_attr_test.go'
    Set-Content -LiteralPath $testFile -Value @('package selftest', '', 'func TestFoo(t *testing.T) {}') -Encoding UTF8

    Test-Case -Name 'spec-attribution-check: warns on a new .go file with no SPEC line' -ScriptPath $attrScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = $noAttrFile }; agent_type = 'corpus-builder'; session_id = $Marker } `
        -StdoutCheck { param($s) ($s -match '\[corpus-context\]') -and ($s -match 'SPEC') }

    Test-Case -Name 'spec-attribution-check: silent on a new .go file that already has a SPEC line' -ScriptPath $attrScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = $withAttrFile }; agent_type = 'corpus-builder'; session_id = $Marker } `
        -StdoutCheck { param($s) [string]::IsNullOrWhiteSpace($s) }

    Test-Case -Name 'spec-attribution-check: silent on a new _test.go file (exempt)' -ScriptPath $attrScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Write'; tool_input = @{ file_path = $testFile }; agent_type = 'corpus-builder'; session_id = $Marker } `
        -StdoutCheck { param($s) [string]::IsNullOrWhiteSpace($s) }

    Test-Case -Name 'spec-attribution-check: silent on an existing tracked file regardless of content' -ScriptPath $attrScript -ExpectedExit 0 `
        -Payload @{ tool_name = 'Edit'; tool_input = @{ file_path = (Join-Path $RepoRoot 'internal/graph/service.go') }; agent_type = 'corpus-builder'; session_id = $Marker } `
        -StdoutCheck { param($s) [string]::IsNullOrWhiteSpace($s) }

    Test-Case -Name 'spec-attribution-check: malformed JSON fails open' -ScriptPath $attrScript -ExpectedExit 0 -Payload '{not valid json'

    Remove-Item -LiteralPath $attrScratchDir -Recurse -Force -ErrorAction SilentlyContinue


    # =========================================================================
    # 5. corpus-fleet-start.ps1
    # =========================================================================
    $fleetStartScript = Join-Path $HooksDir 'corpus-fleet-start.ps1'

    $fsSession    = "$Marker-fleetstart"
    $fsActivePath = Join-Path $LedgerDir "$fsSession.active"
    New-Item -ItemType Directory -Path $LedgerDir -Force | Out-Null
    (@{ run_id = "run-$Marker"; phase = 'BUILD'; skill = 'corpus-build' } | ConvertTo-Json) | Set-Content -LiteralPath $fsActivePath -Encoding UTF8

    $fleetEventsPath = Join-Path $LedgerDir 'fleet_events.jsonl'

    Test-Case -Name 'corpus-fleet-start: tracked session appends a start row' -ScriptPath $fleetStartScript -ExpectedExit 0 `
        -Payload @{ session_id = $fsSession; agent_id = 'agent-selftest-001'; agent_type = 'corpus-builder'; cwd = $RepoRoot } `
        -PostCheck {
            if (-not (Test-Path -LiteralPath $fleetEventsPath)) { return $false }
            $lines = Get-Content -LiteralPath $fleetEventsPath
            $match = $lines | Where-Object { $_ -match [regex]::Escape("run-$Marker") -and $_ -match '"event":"start"' }
            return ($null -ne $match)
        }

    $linesBeforeUntracked = if (Test-Path -LiteralPath $fleetEventsPath) { @(Get-Content -LiteralPath $fleetEventsPath).Count } else { 0 }
    $fsUntrackedSession = "$Marker-untracked-fleetstart"

    Test-Case -Name 'corpus-fleet-start: untracked session (no active file) skips silently' -ScriptPath $fleetStartScript -ExpectedExit 0 `
        -Payload @{ session_id = $fsUntrackedSession; agent_id = 'agent-selftest-002'; agent_type = 'corpus-builder'; cwd = $RepoRoot } `
        -PostCheck {
            $linesAfter = if (Test-Path -LiteralPath $fleetEventsPath) { @(Get-Content -LiteralPath $fleetEventsPath).Count } else { 0 }
            return ($linesAfter -eq $linesBeforeUntracked)
        }

    Test-Case -Name 'corpus-fleet-start: malformed JSON fails open' -ScriptPath $fleetStartScript -ExpectedExit 0 -Payload '{not valid json'
    Test-Case -Name 'corpus-fleet-start: empty stdin fails open' -ScriptPath $fleetStartScript -ExpectedExit 0 -Payload ''

    Remove-Item -LiteralPath $fsActivePath -Force -ErrorAction SilentlyContinue

    # Surgical marker-scoped cleanup of the one row this section appended
    # (never a whole-file delete -- a concurrent real corpus-build run may
    # share fleet_events.jsonl).
    if (Test-Path -LiteralPath $fleetEventsPath) {
        $kept = @(Get-Content -LiteralPath $fleetEventsPath | Where-Object { $_ -notmatch [regex]::Escape($Marker) })
        if ($kept.Count -gt 0) { Set-Content -LiteralPath $fleetEventsPath -Value $kept -Encoding UTF8 }
        else { Remove-Item -LiteralPath $fleetEventsPath -Force -ErrorAction SilentlyContinue }
    }


    # =========================================================================
    # 6. corpus-token-meter.ps1
    # =========================================================================
    $tokenMeterScript = Join-Path $HooksDir 'corpus-token-meter.ps1'

    $tmSession    = "$Marker-tokenmeter"
    $tmActivePath = Join-Path $LedgerDir "$tmSession.active"
    New-Item -ItemType Directory -Path $LedgerDir -Force | Out-Null
    (@{ run_id = "run-$Marker-tm"; phase = 'REVIEW'; skill = 'corpus-build' } | ConvertTo-Json) | Set-Content -LiteralPath $tmActivePath -Encoding UTF8

    $scratchTranscriptRoot = Join-Path $env:TEMP "corpus-build-$Marker-transcripts"
    $mainTranscriptPath    = Join-Path $scratchTranscriptRoot 'main.jsonl'
    New-Item -ItemType Directory -Path $scratchTranscriptRoot -Force | Out-Null
    Set-Content -LiteralPath $mainTranscriptPath -Value 'placeholder' -Encoding UTF8

    $subDir = Join-Path $scratchTranscriptRoot "$tmSession\subagents"
    New-Item -ItemType Directory -Path $subDir -Force | Out-Null
    $tmAgentId = 'agent-selftest-tm-001'
    $agentTranscriptPath = Join-Path $subDir "agent-$tmAgentId.jsonl"

    $usageLines = @(
        (@{ message = @{ usage = @{ output_tokens = 100; input_tokens = 50; cache_creation_input_tokens = 10; cache_read_input_tokens = 5 } } } | ConvertTo-Json -Compress -Depth 6),
        (@{ usage = @{ output_tokens = 25; input_tokens = 10; cache_creation_input_tokens = 0; cache_read_input_tokens = 0 } } | ConvertTo-Json -Compress -Depth 6),
        'not-json-garbage-line'
    )
    Set-Content -LiteralPath $agentTranscriptPath -Value $usageLines -Encoding UTF8

    $csvPath     = Join-Path $LedgerDir 'token_runs.csv'
    $jsonlPath   = Join-Path $LedgerDir 'token_runs.jsonl'
    $skippedPath = Join-Path $LedgerDir 'skipped_subagent_stops.jsonl'
    $csvPreExisted     = Test-Path -LiteralPath $csvPath
    $jsonlPreExisted   = Test-Path -LiteralPath $jsonlPath
    $skippedPreExisted = Test-Path -LiteralPath $skippedPath

    Test-Case -Name 'corpus-token-meter: sums usage from the resolved bounded transcript' -ScriptPath $tokenMeterScript -ExpectedExit 0 `
        -Payload @{ session_id = $tmSession; transcript_path = $mainTranscriptPath; agent_id = $tmAgentId; agent_type = 'corpus-builder' } `
        -PostCheck {
            if (-not (Test-Path -LiteralPath $csvPath)) { return $false }
            $rows = Import-Csv -LiteralPath $csvPath
            $row = $rows | Where-Object { $_.agent_id -eq $tmAgentId }
            if ($null -eq $row) { return $false }
            return ([int]$row.output -eq 125 -and [int]$row.input -eq 60 -and [int]$row.cache_creation -eq 10 -and [int]$row.billable_total -eq 195)
        }

    Test-Case -Name 'corpus-token-meter: dedupes a repeated stop for the same transcript stem' -ScriptPath $tokenMeterScript -ExpectedExit 0 `
        -Payload @{ session_id = $tmSession; transcript_path = $mainTranscriptPath; agent_id = $tmAgentId; agent_type = 'corpus-builder' } `
        -PostCheck {
            $rows = @(Import-Csv -LiteralPath $csvPath | Where-Object { $_.agent_id -eq $tmAgentId })
            return ($rows.Count -eq 1)
        }

    # A DIFFERENT session on purpose: recency fallback (by design) would
    # otherwise resolve this agent_id to $tmSession's just-written
    # agent-selftest-tm-001.jsonl (correctly -- that's what the fallback is
    # for), which would mask the hard-guard path this case exists to prove.
    # A fresh session has no subagents dir at all, so resolution genuinely
    # fails and the guard fires.
    $tmGuardSession    = "$Marker-tm-guard"
    $tmGuardActivePath = Join-Path $LedgerDir "$tmGuardSession.active"
    (@{ run_id = "run-$Marker-tm-guard"; phase = 'REVIEW'; skill = 'corpus-build' } | ConvertTo-Json) | Set-Content -LiteralPath $tmGuardActivePath -Encoding UTF8
    $tmGuardAgentId = 'agent-selftest-tm-unresolved'

    Test-Case -Name 'corpus-token-meter: hard guard skips an unresolvable transcript' -ScriptPath $tokenMeterScript -ExpectedExit 0 `
        -Payload @{ session_id = $tmGuardSession; transcript_path = $mainTranscriptPath; agent_id = $tmGuardAgentId; agent_type = 'corpus-builder' } `
        -PostCheck {
            if (-not (Test-Path -LiteralPath $skippedPath)) { return $false }
            $lines = Get-Content -LiteralPath $skippedPath
            $match = $lines | Where-Object { $_ -match [regex]::Escape($tmGuardAgentId) }
            if ($null -eq $match) { return $false }
            if (-not (Test-Path -LiteralPath $csvPath)) { return $true }
            return ($null -eq (Import-Csv -LiteralPath $csvPath | Where-Object { $_.agent_id -eq $tmGuardAgentId }))
        }

    Remove-Item -LiteralPath $tmGuardActivePath -Force -ErrorAction SilentlyContinue

    Test-Case -Name 'corpus-token-meter: untracked session (no active file) skips silently' -ScriptPath $tokenMeterScript -ExpectedExit 0 `
        -Payload @{ session_id = "$Marker-tm-untracked"; transcript_path = $mainTranscriptPath; agent_id = 'agent-x'; agent_type = 'corpus-builder' } `
        -PostCheck {
            if (-not (Test-Path -LiteralPath $csvPath)) { return $true }
            return ($null -eq (Import-Csv -LiteralPath $csvPath | Where-Object { $_.agent_id -eq 'agent-x' }))
        }

    Test-Case -Name 'corpus-token-meter: malformed JSON fails open' -ScriptPath $tokenMeterScript -ExpectedExit 0 -Payload '{not valid json'
    Test-Case -Name 'corpus-token-meter: empty stdin fails open' -ScriptPath $tokenMeterScript -ExpectedExit 0 -Payload ''

    Remove-Item -LiteralPath $tmActivePath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $scratchTranscriptRoot -Recurse -Force -ErrorAction SilentlyContinue

    # Surgical marker-scoped cleanup of the shared ledger files (never a whole-
    # file delete when the file pre-existed -- a concurrent real corpus-build
    # run may share these files). If THIS run created the file and no real
    # data survives the marker filter, remove it entirely rather than leaving
    # a header-only/empty stub behind.
    $preExistedMap = @{ $csvPath = $csvPreExisted; $jsonlPath = $jsonlPreExisted; $skippedPath = $skippedPreExisted }
    foreach ($p in @($csvPath, $jsonlPath, $skippedPath)) {
        if (-not (Test-Path -LiteralPath $p)) { continue }
        $lines = @(Get-Content -LiteralPath $p)
        $preExisted = $preExistedMap[$p]
        if ($p -eq $csvPath -and $lines.Count -gt 0) {
            $header = $lines[0]
            $rest = @($lines | Select-Object -Skip 1 | Where-Object { $_ -notmatch [regex]::Escape($Marker) })
            if ($rest.Count -eq 0 -and -not $preExisted) {
                Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue
            } else {
                Set-Content -LiteralPath $p -Value (@($header) + $rest) -Encoding UTF8
            }
        } else {
            $kept = @($lines | Where-Object { $_ -notmatch [regex]::Escape($Marker) })
            if ($kept.Count -gt 0) { Set-Content -LiteralPath $p -Value $kept -Encoding UTF8 }
            else { Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue }
        }
    }

} finally {
    # Context-index restore/removal: ALWAYS runs, even if a test case above
    # threw, so a concurrent agent's real production index (built by W5's
    # build_tag_index.py) is never left holding this run's fixture content.
    try {
        if ($null -ne $indexPath) {
            if ($indexPreExisted -and $null -ne $indexBackupPath -and (Test-Path -LiteralPath $indexBackupPath)) {
                Copy-Item -LiteralPath $indexBackupPath -Destination $indexPath -Force
                Remove-Item -LiteralPath $indexBackupPath -Force -ErrorAction SilentlyContinue
            } elseif (-not $indexPreExisted) {
                Remove-Item -LiteralPath $indexPath -Force -ErrorAction SilentlyContinue
            }
        }
    } catch { }

    # Final ledger-dir cleanup: only remove the directory this run created,
    # and only if it is now empty (never destroys a concurrent run's files).
    try {
        if (-not $LedgerPreExisted -and (Test-Path -LiteralPath $LedgerDir)) {
            $remaining = @(Get-ChildItem -LiteralPath $LedgerDir -Force -ErrorAction SilentlyContinue)
            if ($remaining.Count -eq 0) {
                Remove-Item -LiteralPath $LedgerDir -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    } catch { }
}

$total = $script:PassCount + $script:FailCount
Write-Output ''
Write-Output "corpus-build hook self-test: $($script:PassCount)/$total passed"

if ($script:FailCount -gt 0) { exit 1 }
exit 0
