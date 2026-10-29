param([switch]$Full)
$ErrorActionPreference = 'Stop'
if (-not $IsWindows) { throw 'This reproducer requires Windows short file names.' }
Add-Type @'
using System.Text;
using System.Runtime.InteropServices;
public static class CodeNerdShortPath {
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    public static extern uint GetShortPathName(string path, StringBuilder result, uint length);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    public static extern uint GetLongPathName(string path, StringBuilder result, uint length);
}
'@
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$probeRoot = Join-Path ([IO.Path]::GetTempPath()) ('codenerd-path-identity-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $probeRoot | Out-Null
$buffer = [Text.StringBuilder]::new(4096)
if ([CodeNerdShortPath]::GetShortPathName($probeRoot, $buffer, 4096) -eq 0) { throw 'Cannot resolve short path.' }
$shortRoot = $buffer.ToString()
$longBuffer = [Text.StringBuilder]::new(4096)
if ([CodeNerdShortPath]::GetLongPathName($probeRoot, $longBuffer, 4096) -eq 0) { throw 'Cannot resolve long path.' }
if ($shortRoot -eq $longBuffer.ToString()) { throw '8.3 names unavailable on this volume; no alias test was run.' }
$headerBuffer = [Text.StringBuilder]::new(4096)
if ([CodeNerdShortPath]::GetShortPathName((Join-Path $repoRoot 'sqlite_headers'), $headerBuffer, 4096) -eq 0) { throw 'Cannot resolve SQLite header path.' }
$priorTemp, $priorTmp, $priorFlags = $env:TEMP, $env:TMP, $env:CGO_CFLAGS
try {
    $env:TEMP = $shortRoot
    $env:TMP = $shortRoot
    $env:CGO_CFLAGS = '-I' + $headerBuffer.ToString().Replace('\', '/')
    Push-Location $repoRoot
    try {
        $packages = @('./internal/browser/security', './internal/browser/specs', './internal/session', './internal/system', './internal/tools/...', './internal/world')
        if ($Full) { $packages = @('./...') }
        Write-Host "Testing through short-path TEMP: $shortRoot"
        & go test -p 2 -tags sqlite_vec @packages
        $testExit = $LASTEXITCODE
    } finally { Pop-Location }
} finally {
    $env:TEMP, $env:TMP, $env:CGO_CFLAGS = $priorTemp, $priorTmp, $priorFlags
    # Do not recursively remove a test's leftovers. An empty probe directory
    # can be removed directly; retained artifacts remain available to inspect.
    if (-not (Get-ChildItem -LiteralPath $probeRoot -Force | Select-Object -First 1)) {
        Remove-Item -LiteralPath $probeRoot
    } else { Write-Host "Retained test artifacts: $probeRoot" }
}
exit $testExit
