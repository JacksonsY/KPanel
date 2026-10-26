# One native verification entry point, shared by candidate CI and Release.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$OutputDirectory)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'Native Windows verification requires Windows.' }
if (-not [IO.Path]::IsPathFullyQualified($OutputDirectory) -or (Test-Path -LiteralPath $OutputDirectory)) { throw 'Use a new absolute output directory.' }
Push-Location (Join-Path $PSScriptRoot '..')
try {
    $candidate=(& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or (& git status --porcelain)) { throw 'Use a clean candidate checkpoint.' }
    foreach ($path in @('deploy/windows/install.ps1','internal/cluster/bootstrap-windows.ps1','scripts/windows-node-release.ps1')) {
        $tokens=$null; $parseErrors=$null
        $null=[Management.Automation.Language.Parser]::ParseFile((Join-Path (Get-Location) $path),[ref]$tokens,[ref]$parseErrors)
        if ($parseErrors.Count -ne 0) { throw "Invalid PowerShell syntax: $path" }
    }
    go test ./internal/windowsnode
    if ($LASTEXITCODE -ne 0) { throw 'Windows service storage and update tests failed.' }
    go test ./cmd/kejilion-node ./internal/systeminfo ./internal/hostpty ./internal/terminal ./internal/filemanager ./internal/monitoring -run '^TestWindows' -count=1
    if ($LASTEXITCODE -ne 0) { throw 'Windows node runtime tests failed.' }
    go vet ./cmd/kejilion-node
    if ($LASTEXITCODE -ne 0) { throw 'Windows node vet failed.' }
    New-Item -ItemType Directory -Path $OutputDirectory | Out-Null
    $version='v'+(Get-Content VERSION -Raw).Trim()
    & ./scripts/windows-node-release.ps1 -Mode Build -OutputDirectory $OutputDirectory -Version $version
    & ./scripts/windows-node-release.ps1 -Mode Verify -OutputDirectory $OutputDirectory -Version $version
    if ((& git rev-parse HEAD).Trim() -cne $candidate -or (& git status --porcelain)) { throw 'Candidate source changed during native verification.' }
    Write-Output "windows_node_verification=pass candidate=$candidate"
} finally { Pop-Location }
