$ErrorActionPreference = 'Stop'
$workspace = Split-Path -Parent $PSScriptRoot
$sdk = Join-Path $env:USERPROFILE '.codex\tools\dotnet-sdk-8.0\dotnet.exe'
if (-not (Test-Path -LiteralPath $sdk)) { $sdk = (Get-Command dotnet -ErrorAction Stop).Source }

function Invoke-Check([string]$Name, [scriptblock]$Run) {
    Write-Output "Checking $Name"
    & $Run
    if ($LASTEXITCODE -ne 0) { throw "$Name failed with exit code $LASTEXITCODE" }
}

Push-Location $workspace
try {
    Invoke-Check 'backend, isolated setup upgrade and shortcut repair' { go test -count=1 . ./cmd/... ./internal/... }
    Invoke-Check 'frontend regression tests' { pnpm --dir frontend test }
    Invoke-Check 'native screenshot regression tests' {
        & $sdk run --project native/Ariadne.CaptureHost.Tests/Ariadne.CaptureHost.Tests.csproj -c Release
    }
    Invoke-Check 'iPhone input, drag, IPC and status regression tests' {
        & $sdk run --project native/Ariadne.iPhoneControl.Tests/Ariadne.iPhoneControl.Tests.csproj -c Release
    }
    Invoke-Check 'desktop bindings' { wails3 generate bindings -clean=false }
    Invoke-Check 'frontend type check and production build' { pnpm --dir frontend build }
    Write-Output 'Release verification passed.'
} finally {
    Pop-Location
}
