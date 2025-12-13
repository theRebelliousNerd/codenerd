param(
  [string]$RepoRoot = (Split-Path -Parent $PSScriptRoot)
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Push-Location $RepoRoot
try {
  Write-Host "[preflight] go test ./..."
  go test ./...

  Write-Host "[preflight] action_linter (fail on warn)"
  go run ./cmd/tools/action_linter -fail-on-warn -exempt-file Docs/plans/action_linter_exemptions.txt

  Write-Host "[preflight] OK"
} finally {
  Pop-Location
}

