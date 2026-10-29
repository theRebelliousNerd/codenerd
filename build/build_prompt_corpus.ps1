# Build Prompt Corpus Database
# This script generates the embedded prompt_corpus.db for JIT prompt compilation.
#
# Source of truth for built-in prompt atoms is `internal/prompt/atoms/`.
#
# Usage:
#   .\build\build_prompt_corpus.ps1
#   .\build\build_prompt_corpus.ps1 -SkipEmbeddings  # For faster testing
#
# Prerequisites:
#   - Set GEMINI_API_KEY environment variable, or
#   - Configure .nerd/config.json with genai_api_key

param(
    [switch]$SkipEmbeddings = $false
)

$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "  Building Prompt Corpus Database" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Set CGO flags for sqlite-vec support
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"

# Build and run the prompt builder
$buildArgs = @(
    "run"
    "-tags=sqlite_vec"
    "./cmd/tools/prompt_builder"
    "-input", "internal/prompt/atoms"
    "-output", "internal/core/defaults/prompt_corpus.db"
)

if ($SkipEmbeddings) {
    $buildArgs += "-skip-embeddings"
    Write-Host "[INFO] Skipping embedding generation (testing mode)" -ForegroundColor Yellow
}

Write-Host "[INFO] Running prompt builder..." -ForegroundColor Green
Write-Host "go $($buildArgs -join ' ')" -ForegroundColor Gray
Write-Host ""

& go @buildArgs

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Prompt builder failed with exit code $LASTEXITCODE" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host ""
Write-Host "[SUCCESS] Prompt corpus database built successfully" -ForegroundColor Green
Write-Host "Output: internal/core/defaults/prompt_corpus.db" -ForegroundColor Gray
