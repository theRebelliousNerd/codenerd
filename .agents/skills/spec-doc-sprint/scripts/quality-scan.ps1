#!/usr/bin/env pwsh
# Quality scoring for spec documents. Grades each doc A-F and produces overall subsystem grade.
# Usage: .\quality-scan.ps1 -Subsystem "internal/campaign" [-All] [-Detailed] [-JsonOutput]
#
# Dynamic minimum line counts:
#   - Counts non-test .go files in the source package
#   - Small packages (1-10 files): per-doc minimum = source_files * 10
#   - Large packages (11+ files): per-doc minimum = source_files * 20
#   - Floor of 30 lines per doc (even tiny packages need substance)
#
# Anti-cheating checks:
#   1. Placeholder evasion ("varies", "TBD", "N/A" in table cells)
#   2. Filler phrase density (word salad detection)
#   3. Repetitive sentence detection
#   4. Code file reference validation (must cite real .go files)
#   5. Specificity score (concrete nouns vs abstract filler)
#   6. Table completeness (no empty/dash-only cells)
#   7. Keyword density abuse
#   8. Cross-document consistency (narrative arc)

param(
    [string]$Subsystem,
    [switch]$All,
    [switch]$Detailed,
    [switch]$JsonOutput
)

$SpecRoot = "C:\CodeProjects\codeNERD\Docs\Spec"
$SourceRoot = "C:\CodeProjects\codeNERD"

$docFiles = @(
    "README.md", "north-star.md", "current-state.md", "gap-analysis.md",
    "data-flow.md", "dependencies.md", "wiring.md", "api-contract.md",
    "test-strategy.md", "failure-modes.md", "error-taxonomy.md",
    "safety-model.md", "performance-profile.md", "observability.md",
    "configuration.md", "design-decisions.md", "todos.md", "glossary.md"
)

# Pillar documents get 2x weight in overall grade
$PillarDocs = @("current-state.md", "north-star.md", "gap-analysis.md")

# Documents that MUST reference .go source files
$MustReferenceCode = @(
    "current-state.md", "data-flow.md", "dependencies.md", "wiring.md",
    "api-contract.md", "error-taxonomy.md", "performance-profile.md",
    "failure-modes.md", "configuration.md"
)

# Filler phrases that indicate word salad
$FillerPhrases = @(
    'provides essential', 'ensures proper', 'handles various',
    'facilitates the', 'responsible for', 'plays a critical role',
    'key component', 'integral part', 'seamlessly integrates',
    'robust implementation', 'comprehensive solution', 'leverages the',
    'in a manner consistent with', 'as appropriate', 'when necessary',
    'provides robust', 'critical component', 'essential functionality',
    'core operations', 'fundamental', 'holistic approach'
)

# Placeholder evasion patterns in table cells
$EvasionPatterns = @(
    '\|\s*varies\s*\|',
    '\|\s*TBD\s*\|',
    '\|\s*N/?A\s*\|',
    '\|\s*-+\s*\|',
    '\|\s*\?\s*\|',
    '\|\s*see above\s*\|',
    '\|\s*as needed\s*\|',
    '\|\s*various\s*\|',
    '\|\s*unknown\s*\|',
    '\|\s*TODO\s*\|'
)

# Suspect high-frequency vague words
$SuspectWords = @(
    'system', 'subsystem', 'component', 'module', 'provides',
    'manages', 'handles', 'ensures', 'supports', 'enables',
    'facilitates', 'leverages', 'comprehensive', 'robust'
)

# Scoring deductions per issue type
$Deductions = @{
    "MISSING_SECTION" = 20
    "HEADER"          = 15
    "PLACEHOLDERS"    = 10
    "LENGTH"          = 15
    "THIN_SECTION"    = 10
    "EVASION"         = 15
    "WORDSALAD"       = 20
    "DENSITY"         = 10
    "REPETITIVE"      = 15
    "GROUNDING"       = 15
    "TRACE"           = 10
    "ALIGNMENT"       = 20
    "OVERLAP"         = 10
    "CODE"            = 10
    "STATUS"          = 5
    "ABSTRACT"        = 10
    "EMPTYCELLS"      = 10
    "INVENTORY"       = 10
}

function Get-Grade {
    param([int]$Score)
    if ($Score -ge 90) { return "A" }
    if ($Score -ge 80) { return "B" }
    if ($Score -ge 70) { return "C" }
    if ($Score -ge 60) { return "D" }
    return "F"
}

function Get-GradeColor {
    param([string]$Grade)
    switch ($Grade) {
        "A" { return "Green" }
        "B" { return "Cyan" }
        "C" { return "Yellow" }
        "D" { return "DarkYellow" }
        "F" { return "Red" }
    }
}

function Get-SourceFileCount {
    param([string]$SubsystemPath)
    $srcDir = Join-Path $SourceRoot $SubsystemPath
    if (-not (Test-Path $srcDir)) { return 0 }
    $goFiles = Get-ChildItem -Path $srcDir -Filter "*.go" -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -notmatch '_test\.go$' }
    return @($goFiles).Count
}

function Get-MinLinesPerDoc {
    param([int]$SourceFileCount)
    if ($SourceFileCount -le 0) { return 30 }
    if ($SourceFileCount -le 10) {
        # Small package: multiply by 10
        return [math]::Max(30, $SourceFileCount * 10)
    }
    else {
        # Large package: multiply by 20
        return [math]::Max(30, $SourceFileCount * 20)
    }
}

function Get-ContentLines {
    param([string[]]$Lines)
    # Skip header block (first 6 lines), count non-blank lines
    $content = $Lines | Select-Object -Skip 6 | Where-Object { $_.Trim() -ne "" }
    return @($content).Count
}

function Test-DocQuality {
    param(
        [string]$FilePath,
        [string]$FileName,
        [int]$MinLines,
        [string]$SourceDir
    )

    $result = @{
        Score  = 100
        Issues = @()
        Grade  = "A"
    }

    if (-not (Test-Path $FilePath)) {
        $result.Score = 0
        $result.Issues += @{ Type = "MISSING"; Msg = "File does not exist"; Deduction = 100 }
        $result.Grade = "F"
        return $result
    }

    $content = Get-Content $FilePath -Raw -Encoding UTF8
    $lines = Get-Content $FilePath -Encoding UTF8

    # --- Check 1: Status ---
    if ($content -match "Not Started") {
        $d = $Deductions["STATUS"]
        $result.Score -= $d
        $result.Issues += @{ Type = "STATUS"; Msg = "Still marked as Not Started"; Deduction = $d }
    }

    # --- Check 2: Unfilled placeholders ---
    $placeholders = ([regex]::Matches($content, '<!--[^>]*-->')).Count
    if ($placeholders -gt 0) {
        $d = [math]::Min($Deductions["PLACEHOLDERS"] * $placeholders, 30)
        $result.Score -= $d
        $result.Issues += @{ Type = "PLACEHOLDERS"; Msg = "$placeholders unfilled placeholders"; Deduction = $d }
    }

    # --- Check 3: Minimum line count ---
    $contentLines = Get-ContentLines $lines
    if ($contentLines -lt $MinLines) {
        $d = $Deductions["LENGTH"]
        $result.Score -= $d
        $result.Issues += @{ Type = "LENGTH"; Msg = "Only $contentLines content lines (minimum: $MinLines)"; Deduction = $d }
    }

    # --- Check 4: Code blocks (non-mermaid) ---
    $codeBlocks = ([regex]::Matches($content, '```(?!mermaid)[a-z]+\r?\n')).Count
    if ($codeBlocks -gt 0) {
        $d = $Deductions["CODE"]
        $result.Score -= $d
        $result.Issues += @{ Type = "CODE"; Msg = "$codeBlocks non-mermaid code blocks (natural language only)"; Deduction = $d }
    }

    # --- Check 5: Placeholder evasion ---
    $evasionCount = 0
    foreach ($pattern in $EvasionPatterns) {
        $evasionCount += ([regex]::Matches($content, $pattern,
                [System.Text.RegularExpressions.RegexOptions]::IgnoreCase -bor
                [System.Text.RegularExpressions.RegexOptions]::Multiline)).Count
    }
    if ($evasionCount -gt 2) {
        $d = $Deductions["EVASION"]
        $result.Score -= $d
        $result.Issues += @{ Type = "EVASION"; Msg = "$evasionCount placeholder-evasion cells (varies/TBD/N-A)"; Deduction = $d }
    }

    # --- Check 6: Word salad / filler phrase density ---
    $fillerCount = 0
    foreach ($phrase in $FillerPhrases) {
        $fillerCount += ([regex]::Matches($content, [regex]::Escape($phrase),
                [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)).Count
    }
    if ($fillerCount -gt 5) {
        $d = $Deductions["WORDSALAD"]
        $result.Score -= $d
        $result.Issues += @{ Type = "WORDSALAD"; Msg = "$fillerCount filler phrases detected"; Deduction = $d }
    }

    # --- Check 6b: GIBBERISH hard-fail (adversarial word padding) ---
    # Fails on 5+ consecutive 'ly' adverbs, or if any adverb appears >= 4 times in a 25-word window
    foreach ($line in $lines) {
        # Check for 5+ consecutive adverbs
        if ($line -match '(?i)(\b\w+ly\b\s*[,.]?\s*){5,}') {
            $result.Score = 0
            $result.Issues += @{ Type = "GIBBERISH"; Msg = "HARD FAIL: Detected 5+ consecutive adverbs ('ly' words). Adversarial padding."; Deduction = 100 }
            $result.Grade = "F"
            return $result
        }

        # Check for adverb repetition in sliding window
        $lineWords = ($line -replace '[^\w\s]', '' -split '\s+' | Where-Object { $_.Length -gt 4 })
        if ($lineWords.Count -ge 20) {
            for ($i = 0; $i -le ($lineWords.Count - 25); $i += 5) {
                $chunk = $lineWords[$i..[math]::Min($i + 24, $lineWords.Count - 1)]
                $freq = @{}
                foreach ($w in $chunk) {
                    if ($w -match 'ly$') {
                        $lw = $w.ToLower()
                        if (-not $freq.ContainsKey($lw)) { $freq[$lw] = 0 }
                        $freq[$lw]++
                        if ($freq[$lw] -ge 4) {
                            $result.Score = 0
                            $result.Issues += @{ Type = "GIBBERISH"; Msg = "HARD FAIL: Repetitive adverb padding ('$lw' appears $($freq[$lw]) times in 25-word window)."; Deduction = 100 }
                            $result.Grade = "F"
                            return $result
                        }
                    }
                }
            }
        }
    }

    # --- Check 7: Repetitive sentences ---
    $sentences = $content -split '[.!?]\s+' | Where-Object { $_.Trim().Length -gt 20 }
    $sentenceHashes = @{}
    $duplicateSentences = 0
    foreach ($s in $sentences) {
        $normalized = ($s.Trim().ToLower() -replace '\s+', ' ')
        if ($sentenceHashes.ContainsKey($normalized)) {
            $duplicateSentences++
        }
        $sentenceHashes[$normalized] = $true
    }
    if ($duplicateSentences -gt 2) {
        $d = $Deductions["REPETITIVE"]
        $result.Score -= $d
        $result.Issues += @{ Type = "REPETITIVE"; Msg = "$duplicateSentences duplicate sentences"; Deduction = $d }
    }

    # --- Check 8: Keyword density abuse ---
    $words = ($content -replace '[^\w\s]', '' -split '\s+' | Where-Object { $_.Length -gt 2 })
    $totalWords = $words.Count
    if ($totalWords -gt 0) {
        $wordFreq = @{}
        foreach ($w in $words) {
            $lower = $w.ToLower()
            if ($wordFreq.ContainsKey($lower)) { $wordFreq[$lower]++ }
            else { $wordFreq[$lower] = 1 }
        }
        foreach ($sw in $SuspectWords) {
            if ($wordFreq.ContainsKey($sw)) {
                $density = [math]::Round(($wordFreq[$sw] / $totalWords) * 100, 2)
                if ($density -gt 3.0) {
                    $d = $Deductions["DENSITY"]
                    $result.Score -= $d
                    $result.Issues += @{ Type = "DENSITY"; Msg = "'$sw' at $density% density ($($wordFreq[$sw])x)"; Deduction = $d }
                }
            }
        }
    }

    # --- Check 9: Code file reference validation ---
    if ($MustReferenceCode -contains $FileName) {
        $goFileRefs = [regex]::Matches($content, '\w+\.go')
        $refCount = $goFileRefs.Count
        if ($refCount -lt 2) {
            $d = $Deductions["GROUNDING"]
            $result.Score -= $d
            $result.Issues += @{ Type = "GROUNDING"; Msg = "Only $refCount .go references (must cite source files)"; Deduction = $d }
        }
        elseif ($SourceDir -and (Test-Path $SourceDir)) {
            # Validate that referenced .go files actually exist
            $validRefs = 0
            $totalRefs = 0
            foreach ($ref in $goFileRefs) {
                $totalRefs++
                $refFile = Join-Path $SourceDir $ref.Value
                if (Test-Path $refFile) { $validRefs++ }
            }
            if ($totalRefs -gt 0) {
                $validPct = [math]::Round(($validRefs / $totalRefs) * 100, 0)
                if ($validPct -lt 60) {
                    $d = $Deductions["GROUNDING"]
                    $result.Score -= $d
                    $result.Issues += @{ Type = "GROUNDING"; Msg = "Only $validPct% of .go references are valid files ($validRefs/$totalRefs)"; Deduction = $d }
                }
            }
        }
    }

    # --- Check 10: Specificity score ---
    $concreteRefs = ([regex]::Matches($content, '\w+\.(go|mg|json|md)|~?\d+[KMG]?\b|\b[A-Z][a-z]+[A-Z]\w+')).Count
    if ($totalWords -gt 0) {
        $specificityRatio = [math]::Round($concreteRefs / $totalWords * 100, 2)
        if ($specificityRatio -lt 1.0 -and $MustReferenceCode -contains $FileName) {
            $d = $Deductions["ABSTRACT"]
            $result.Score -= $d
            $result.Issues += @{ Type = "ABSTRACT"; Msg = "Specificity $specificityRatio% -- too abstract"; Deduction = $d }
        }
    }

    # --- Check 11: Table completeness ---
    $tableRows = [regex]::Matches($content, '^\|[^-].*\|$',
        [System.Text.RegularExpressions.RegexOptions]::Multiline)
    $emptyTableCells = 0
    foreach ($row in $tableRows) {
        $cells = $row.Value -split '\|' | Where-Object { $_.Trim() -ne "" }
        foreach ($cell in $cells) {
            if ($cell.Trim() -match '^(--|—|\s*)$') { $emptyTableCells++ }
        }
    }
    if ($emptyTableCells -gt 3) {
        $d = $Deductions["EMPTYCELLS"]
        $result.Score -= $d
        $result.Issues += @{ Type = "EMPTYCELLS"; Msg = "$emptyTableCells empty/dash-only table cells"; Deduction = $d }
    }

    # --- File-specific checks ---
    switch ($FileName) {
        "current-state.md" {
            $variesCount = ([regex]::Matches($content, '\|\s*varies\s*\|',
                    [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)).Count
            if ($variesCount -gt 0) {
                $d = $Deductions["INVENTORY"]
                $result.Score -= $d
                $result.Issues += @{ Type = "INVENTORY"; Msg = "$variesCount 'varies' cells in file inventory"; Deduction = $d }
            }
            if ($content -notmatch '(Step\s+\d|step\s+\d|\d+\.\s+\w)') {
                $d = $Deductions["TRACE"]
                $result.Score -= $d
                $result.Issues += @{ Type = "TRACE"; Msg = "No step-by-step behavior trace found"; Deduction = $d }
            }
        }
        "north-star.md" {
            $goalCount = ([regex]::Matches($content, '###\s+Goal\s+\d')).Count
            if ($goalCount -lt 3) {
                $d = $Deductions["THIN_SECTION"]
                $result.Score -= $d
                $result.Issues += @{ Type = "THIN_SECTION"; Msg = "Only $goalCount goals (minimum 3)"; Deduction = $d }
            }
            if ($content -notmatch 'Non.?Goals') {
                $d = $Deductions["MISSING_SECTION"]
                $result.Score -= $d
                $result.Issues += @{ Type = "MISSING_SECTION"; Msg = "No Non-Goals section"; Deduction = $d }
            }
            if ($content -notmatch '(Roadmap|Uplift|Phase\s+\d)') {
                $d = $Deductions["MISSING_SECTION"]
                $result.Score -= $d
                $result.Issues += @{ Type = "MISSING_SECTION"; Msg = "No Uplift Roadmap section"; Deduction = $d }
            }
            if ($content -notmatch 'Relationship to.*Architecture') {
                $d = $Deductions["MISSING_SECTION"]
                $result.Score -= $d
                $result.Issues += @{ Type = "MISSING_SECTION"; Msg = "No Relationship to Architecture section"; Deduction = $d }
            }
        }
        "gap-analysis.md" {
            $requiredSections = @(
                @{ Pattern = "Built But Not Spec"; Name = "Built But Not Spec'd" },
                @{ Pattern = "Not Built"; Name = "Spec'd But Not Built" },
                @{ Pattern = "Partially Implemented"; Name = "Partially Implemented" },
                @{ Pattern = "Recommendations"; Name = "Recommendations" }
            )
            foreach ($section in $requiredSections) {
                if ($content -notmatch $section.Pattern) {
                    $d = $Deductions["MISSING_SECTION"]
                    $result.Score -= $d
                    $result.Issues += @{ Type = "MISSING_SECTION"; Msg = "Missing '$($section.Name)' section"; Deduction = $d }
                }
            }
            if ($content -notmatch '(north.?star|goal\s+\d|alignment)') {
                $d = $Deductions["ALIGNMENT"]
                $result.Score -= $d
                $result.Issues += @{ Type = "ALIGNMENT"; Msg = "No north-star goal references"; Deduction = $d }
            }
        }
        "failure-modes.md" {
            if ($content -notmatch '\*\*Trigger\*\*') {
                $d = $Deductions["MISSING_SECTION"]
                $result.Score -= $d
                $result.Issues += @{ Type = "MISSING_SECTION"; Msg = "No Trigger/Symptoms/Impact structure"; Deduction = $d }
            }
        }
        "todos.md" {
            $checkboxes = ([regex]::Matches($content, '- \[([ x/])\]')).Count
            if ($checkboxes -eq 0) {
                $d = $Deductions["MISSING_SECTION"]
                $result.Score -= $d
                $result.Issues += @{ Type = "MISSING_SECTION"; Msg = "No checkbox items found"; Deduction = $d }
            }
        }
        "design-decisions.md" {
            if ($content -notmatch 'ADR-\d+') {
                $d = $Deductions["MISSING_SECTION"]
                $result.Score -= $d
                $result.Issues += @{ Type = "MISSING_SECTION"; Msg = "No ADR entries found"; Deduction = $d }
            }
        }
    }

    # Floor score at 0
    $result.Score = [math]::Max(0, $result.Score)
    $result.Grade = Get-Grade $result.Score

    return $result
}

function Test-CrossDocConsistency {
    param([string]$SubDir)

    $crossIssues = @()

    $northStarPath = Join-Path $SubDir "north-star.md"
    $gapPath = Join-Path $SubDir "gap-analysis.md"
    $currentPath = Join-Path $SubDir "current-state.md"
    $todosPath = Join-Path $SubDir "todos.md"

    # Check north-star <-> gap-analysis alignment
    if ((Test-Path $northStarPath) -and (Test-Path $gapPath)) {
        $nsContent = Get-Content $northStarPath -Raw -Encoding UTF8
        $gapContent = Get-Content $gapPath -Raw -Encoding UTF8

        $goalHeaders = [regex]::Matches($nsContent, '###\s+Goal\s+\d+:\s*(.+)')
        $goalsMentionedInGap = 0
        foreach ($m in $goalHeaders) {
            $goalName = $m.Groups[1].Value.Trim()
            $keywords = $goalName -split '\s+' | Where-Object { $_.Length -gt 4 }
            foreach ($kw in $keywords) {
                if ($gapContent -match [regex]::Escape($kw)) {
                    $goalsMentionedInGap++
                    break
                }
            }
        }
        if ($goalHeaders.Count -gt 0 -and $goalsMentionedInGap -eq 0) {
            $crossIssues += "ALIGNMENT: No north-star goals referenced in gap-analysis.md"
        }
        if ($gapContent -notmatch '(north.?star|goal\s+\d|roadmap|alignment)') {
            $crossIssues += "ALIGNMENT: gap-analysis.md has no north-star references at all"
        }
    }

    # Check gap-analysis <-> todos alignment
    if ((Test-Path $gapPath) -and (Test-Path $todosPath)) {
        $gapContent = Get-Content $gapPath -Raw -Encoding UTF8
        $todosContent = Get-Content $todosPath -Raw -Encoding UTF8

        if ($gapContent -match 'Critical' -and $todosContent -notmatch '(P0|Critical)') {
            $crossIssues += "ALIGNMENT: gap-analysis has Critical gaps but todos.md has no P0 items"
        }
    }

    # Check current-state <-> north-star overlap (copy-paste detection)
    if ((Test-Path $currentPath) -and (Test-Path $northStarPath)) {
        $csContent = Get-Content $currentPath -Raw -Encoding UTF8
        $nsContent = Get-Content $northStarPath -Raw -Encoding UTF8

        $csLines = ($csContent -split "`n" | Where-Object { $_.Trim().Length -gt 30 })
        $nsLines = ($nsContent -split "`n" | Where-Object { $_.Trim().Length -gt 30 })
        $overlap = 0
        foreach ($csLine in $csLines) {
            foreach ($nsLine in $nsLines) {
                $csNorm = $csLine.Trim().ToLower()
                $nsNorm = $nsLine.Trim().ToLower()
                if ($csNorm -eq $nsNorm -and $csNorm -notmatch '^\|.*\|$' -and $csNorm -notmatch '^#{1,3}\s') {
                    $overlap++
                }
            }
        }
        if ($overlap -gt 3) {
            $crossIssues += "OVERLAP: $overlap identical lines between current-state.md and north-star.md"
        }
    }

    return $crossIssues
}

function Write-SourceMandate {
    param(
        [string]$SubsystemPath,
        [string]$OverallGrade,
        [hashtable]$DocResults
    )

    $srcDir = Join-Path $SourceRoot $SubsystemPath
    if (-not (Test-Path $srcDir)) {
        Write-Host "`n    WARNING: SOURCE MANDATE: Source directory '$srcDir' not found -- cannot generate read list." -ForegroundColor Red
        return
    }

    # Only emit mandate if the subsystem needs work (grade below A)
    if ($OverallGrade -eq "A") { return }

    Write-Host ""
    Write-Host "    +========================================================+" -ForegroundColor Yellow
    Write-Host "    |              SOURCE MANDATE (READ FIRST)                |" -ForegroundColor Yellow
    Write-Host "    +========================================================+" -ForegroundColor Yellow
    Write-Host "    | You MUST read these source files BEFORE rewriting       |" -ForegroundColor Yellow
    Write-Host "    | any spec doc. Docs not grounded in source code will     |" -ForegroundColor Yellow
    Write-Host "    | be rejected by the GIBBERISH and GROUNDING checks.      |" -ForegroundColor Yellow
    Write-Host "    +========================================================+" -ForegroundColor Yellow

    # List source .go files (non-test)
    $goFiles = Get-ChildItem -Path $srcDir -Filter "*.go" -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -notmatch '_test\.go$' } | Sort-Object Name
    if ($goFiles.Count -gt 0) {
        Write-Host ""
        Write-Host "    [SOURCE] Source Files ($($goFiles.Count)):" -ForegroundColor Cyan
        foreach ($f in $goFiles) {
            $sizeKB = [math]::Round($f.Length / 1024, 1)
            $label = if ($sizeKB -ge 10) { "LARGE" } elseif ($sizeKB -ge 5) { "medium" } else { "" }
            $labelStr = if ($label) { " [$label]" } else { "" }
            Write-Host "       -> $($f.Name) ($sizeKB KB)$labelStr" -ForegroundColor White
        }
    }

    # List test files
    $testFiles = Get-ChildItem -Path $srcDir -Filter "*_test.go" -ErrorAction SilentlyContinue | Sort-Object Name
    if ($testFiles.Count -gt 0) {
        Write-Host ""
        Write-Host "    [TESTS] Test Files ($($testFiles.Count)):" -ForegroundColor Cyan
        foreach ($f in $testFiles) {
            $sizeKB = [math]::Round($f.Length / 1024, 1)
            Write-Host "       -> $($f.Name) ($sizeKB KB)" -ForegroundColor DarkGray
        }
    }

    # List non-Go artifacts
    $otherFiles = Get-ChildItem -Path $srcDir -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Extension -notin @('.go') -and $_.Name -ne '.gitkeep' } | Sort-Object Name
    if ($otherFiles.Count -gt 0) {
        Write-Host ""
        Write-Host "    [OTHER] Other Artifacts ($($otherFiles.Count)):" -ForegroundColor Cyan
        foreach ($f in $otherFiles) {
            Write-Host "       -> $($f.Name) ($($f.Extension))" -ForegroundColor DarkGray
        }
    }

    # Emit which docs specifically need rewriting
    $failingDocs = @()
    foreach ($docName in $DocResults.Keys) {
        $docGrade = $DocResults[$docName].Grade
        if ($docGrade -notin @("A")) {
            $issues = ($DocResults[$docName].Issues | ForEach-Object { $_.Type }) -join ", "
            $failingDocs += @{ Name = $docName; Grade = $docGrade; Score = $DocResults[$docName].Score; Issues = $issues }
        }
    }
    if ($failingDocs.Count -gt 0) {
        Write-Host ""
        Write-Host "    [FAILING] Docs Needing Rewrite ($($failingDocs.Count)):" -ForegroundColor Red
        foreach ($fd in ($failingDocs | Sort-Object { $_.Score })) {
            Write-Host "       X $($fd.Name) (Grade: $($fd.Grade), Score: $($fd.Score)) -- $($fd.Issues)" -ForegroundColor Red
        }
    }

    # Emit the read order directive
    Write-Host ""
    Write-Host "    +--------------------------------------------------------+" -ForegroundColor Yellow
    Write-Host "    | DIRECTIVE: Read ALL source files listed above, then     |" -ForegroundColor Yellow
    Write-Host "    | rewrite ONLY the failing docs using facts from the      |" -ForegroundColor Yellow
    Write-Host "    | source code. Reference specific filenames, types,       |" -ForegroundColor Yellow
    Write-Host "    | functions, and behaviors. NO word salad.                |" -ForegroundColor Yellow
    Write-Host "    +--------------------------------------------------------+" -ForegroundColor Yellow
    Write-Host ""
}

function Get-Subsystems {
    if ($All) {
        $subs = @()
        $cmdDirs = Get-ChildItem -Path (Join-Path $SpecRoot "cmd") -Directory -ErrorAction SilentlyContinue
        foreach ($d in $cmdDirs) { $subs += "cmd/$($d.Name)" }
        $intDirs = Get-ChildItem -Path (Join-Path $SpecRoot "internal") -Directory -ErrorAction SilentlyContinue
        foreach ($d in $intDirs) { $subs += "internal/$($d.Name)" }
        return $subs | Sort-Object
    }
    elseif ($Subsystem) {
        return @($Subsystem)
    }
    else {
        Write-Host "Usage: .\quality-scan.ps1 -Subsystem 'internal/campaign' or -All" -ForegroundColor Yellow
        exit 1
    }
}

# --- Main ---

Write-Host ""
Write-Host "=== Spec Quality Scan ===" -ForegroundColor Magenta
Write-Host "  Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm')" -ForegroundColor Gray

$subsystems = Get-Subsystems
$allResults = @{}

foreach ($sub in $subsystems) {
    $specDir = Join-Path $SpecRoot $sub
    $sourceDir = Join-Path $SourceRoot $sub
    if (-not (Test-Path $specDir)) {
        Write-Host "`n  [$sub] Spec directory not found" -ForegroundColor Red
        continue
    }

    # Dynamic minimum lines based on source file count
    $sourceFileCount = Get-SourceFileCount $sub
    $minLines = Get-MinLinesPerDoc $sourceFileCount
    $sizeLabel = if ($sourceFileCount -le 10) { "small" } else { "large" }

    Write-Host "`n  [$sub] ($sourceFileCount source files, $sizeLabel, min $minLines lines/doc)" -ForegroundColor White

    $docResults = @{}
    $totalScore = 0
    $totalWeight = 0

    foreach ($doc in $docFiles) {
        $filePath = Join-Path $specDir $doc
        $qResult = Test-DocQuality -FilePath $filePath -FileName $doc -MinLines $minLines -SourceDir $sourceDir

        $docResults[$doc] = $qResult

        # Pillar docs get 2x weight
        $weight = if ($PillarDocs -contains $doc) { 2 } else { 1 }
        $totalScore += $qResult.Score * $weight
        $totalWeight += $weight

        $gradeColor = Get-GradeColor $qResult.Grade
        $issueStr = ""
        if ($qResult.Issues.Count -gt 0) {
            if ($Detailed) {
                $issueStr = ($qResult.Issues | ForEach-Object { "$($_.Type)(-$($_.Deduction)): $($_.Msg)" }) -join "; "
            }
            else {
                $issueStr = ($qResult.Issues | ForEach-Object { $_.Type }) -join ", "
            }
        }

        $docPad = $doc.PadRight(26)
        $scorePad = "$($qResult.Score)".PadLeft(3)
        Write-Host "    $docPad $scorePad  $($qResult.Grade)" -ForegroundColor $gradeColor -NoNewline
        if ($issueStr) {
            Write-Host "  $issueStr" -ForegroundColor DarkGray
        }
        else {
            Write-Host ""
        }
    }

    # Cross-document consistency
    $crossIssues = Test-CrossDocConsistency $specDir
    if ($crossIssues.Count -gt 0) {
        Write-Host "    --- Cross-Document ---" -ForegroundColor DarkGray
        foreach ($ci in $crossIssues) {
            Write-Host "    $ci" -ForegroundColor Red
        }
    }

    # Overall grade
    $overallScore = if ($totalWeight -gt 0) { [math]::Round($totalScore / $totalWeight) } else { 0 }
    $overallGrade = Get-Grade $overallScore
    $overallColor = Get-GradeColor $overallGrade

    # Pillar average
    $pillarScore = 0
    $pillarCount = 0
    foreach ($pd in $PillarDocs) {
        if ($docResults.ContainsKey($pd)) {
            $pillarScore += $docResults[$pd].Score
            $pillarCount++
        }
    }
    $pillarAvg = if ($pillarCount -gt 0) { [math]::Round($pillarScore / $pillarCount) } else { 0 }
    $pillarGrade = Get-Grade $pillarAvg
    $pillarColor = Get-GradeColor $pillarGrade

    Write-Host "    --------------------------" -ForegroundColor DarkGray
    Write-Host "    Overall (weighted):     $overallScore  $overallGrade" -ForegroundColor $overallColor
    Write-Host "    Pillar docs avg:        $pillarAvg  $pillarGrade" -ForegroundColor $pillarColor

    $readiness = if ($overallGrade -match '[AB]' -and $pillarGrade -match '[AB]') {
        "READY FOR COMPLETION"
    }
    else {
        "NOT READY -- needs quality improvements"
    }
    $readyColor = if ($readiness -match "READY FOR") { "Green" } else { "Red" }
    Write-Host "    Status: $readiness" -ForegroundColor $readyColor

    # Emit source mandate if subsystem needs work
    Write-SourceMandate -SubsystemPath $sub -OverallGrade $overallGrade -DocResults $docResults

    $allResults[$sub] = @{
        Overall      = $overallScore
        OverallGrade = $overallGrade
        Pillar       = $pillarAvg
        PillarGrade  = $pillarGrade
        Docs         = $docResults
        CrossIssues  = $crossIssues
    }
}

# Summary across all subsystems
if ($subsystems.Count -gt 1) {
    Write-Host "`n=== Sprint Quality Summary ===" -ForegroundColor Magenta
    $gradeDistribution = @{ A = 0; B = 0; C = 0; D = 0; F = 0 }
    foreach ($sub in $allResults.Keys) {
        $g = $allResults[$sub].OverallGrade
        $gradeDistribution[$g]++
    }
    Write-Host "  A: $($gradeDistribution['A'])  B: $($gradeDistribution['B'])  C: $($gradeDistribution['C'])  D: $($gradeDistribution['D'])  F: $($gradeDistribution['F'])" -ForegroundColor Gray

    $ready = ($allResults.Values | Where-Object { $_.OverallGrade -match '[AB]' -and $_.PillarGrade -match '[AB]' }).Count
    Write-Host "  Ready for completion: $ready / $($allResults.Count)" -ForegroundColor Cyan
}

if ($JsonOutput) {
    $allResults | ConvertTo-Json -Depth 5 | Write-Host
}
