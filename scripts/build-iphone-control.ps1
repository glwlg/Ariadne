param([switch]$Test)
$ErrorActionPreference = 'Stop'
$workspace = Split-Path -Parent $PSScriptRoot
$sdk = Join-Path $env:USERPROFILE '.codex\tools\dotnet-sdk-8.0\dotnet.exe'
if (-not (Test-Path -LiteralPath $sdk)) { $sdk = (Get-Command dotnet -ErrorAction Stop).Source }
if ($Test) {
    & $sdk run --project (Join-Path $workspace 'native\Ariadne.iPhoneControl.Tests\Ariadne.iPhoneControl.Tests.csproj') -c Release
    if ($LASTEXITCODE -ne 0) { throw "iPhone control tests failed: $LASTEXITCODE" }
}
& $sdk publish (Join-Path $workspace 'native\Ariadne.iPhoneControl\Ariadne.iPhoneControl.csproj') `
    -c Release -r win-x64 --self-contained true `
    -p:PublishSingleFile=true -p:EnableCompressionInSingleFile=true `
    -p:IncludeNativeLibrariesForSelfExtract=true -p:DebugType=none -p:DebugSymbols=false `
    -o (Join-Path $workspace 'bin\iphone-control')
if ($LASTEXITCODE -ne 0) { throw "iPhone control publish failed: $LASTEXITCODE" }
