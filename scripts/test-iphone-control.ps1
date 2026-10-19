param([string]$Executable = (Join-Path $PSScriptRoot '..\bin\iphone-control\Ariadne.iPhoneControl.exe'))
$ErrorActionPreference = 'Stop'
$Executable = (Resolve-Path -LiteralPath $Executable).Path
if (Get-Process -Name Ariadne.iPhoneControl -ErrorAction SilentlyContinue) {
    throw 'Close the existing iPhone control window before running the lifecycle smoke.'
}
function Wait-Window($process) {
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    do {
        $process.Refresh()
        if ($process.HasExited) { throw "Control exited before showing its window: $($process.ExitCode)" }
        if ($process.MainWindowHandle -ne 0) { return }
        Start-Sleep -Milliseconds 100
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'Control window did not appear.'
}
function Assert-Exit($process) {
    if (-not $process.WaitForExit(15000)) { throw 'Control did not exit.' }
    $process.Refresh()
    if ($process.ExitCode -ne 0) { throw "Control crashed: $($process.ExitCode)" }
}
$control = $null
$owner = $null
try {
    $control = Start-Process -FilePath $Executable -PassThru
    Wait-Window $control
    $second = Start-Process -FilePath $Executable -PassThru
    Assert-Exit $second
    $control.Refresh()
    if ($control.HasExited) { throw 'Second invocation closed the first control.' }
    Write-Output 'PASS singleton activation'
    if (-not $control.CloseMainWindow()) { throw 'Cannot close control window.' }
    Assert-Exit $control
    Write-Output 'PASS normal window close without crash'
    $owner = Start-Process powershell.exe -ArgumentList @('-NoProfile', '-Command', 'Start-Sleep -Seconds 30') -WindowStyle Hidden -PassThru
    $control = Start-Process -FilePath $Executable -ArgumentList @('--owner-pid', $owner.Id) -PassThru
    Wait-Window $control
    Stop-Process -Id $owner.Id
    Assert-Exit $control
    Write-Output 'PASS owner exit closes control'
} finally {
    if ($control -and -not $control.HasExited) {
        $null = $control.CloseMainWindow()
        if (-not $control.WaitForExit(15000)) { Stop-Process -Id $control.Id }
    }
    if ($owner -and -not $owner.HasExited) { Stop-Process -Id $owner.Id }
}
