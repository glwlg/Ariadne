param([string]$Executable = (Join-Path $PSScriptRoot '..\bin\iphone-control\Ariadne.iPhoneControl.exe'))
$ErrorActionPreference = 'Stop'
$Executable = (Resolve-Path -LiteralPath $Executable).Path
if (Get-Process -Name Ariadne.iPhoneControl -ErrorAction SilentlyContinue) {
    throw 'Stop the existing iPhone control service before running the lifecycle smoke.'
}
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class AriadneControlWindowSmoke {
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern IntPtr FindWindow(string className, string title);
    [DllImport("user32.dll")]
    public static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll")]
    public static extern bool PostMessage(IntPtr hwnd, uint msg, IntPtr wp, IntPtr lp);
    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint pid);
    public static IntPtr FindControl() { return FindWindow(null, "Ariadne · iPhone 控制"); }
}
'@
function Wait-Window($process, [bool]$Visible) {
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    do {
        $process.Refresh()
        if ($process.HasExited) { throw "Control exited before its window was ready: $($process.ExitCode)" }
        $handle = [AriadneControlWindowSmoke]::FindControl()
        [uint32]$windowProcess = 0
        if ($handle -ne [IntPtr]::Zero) {
            $null = [AriadneControlWindowSmoke]::GetWindowThreadProcessId($handle, [ref]$windowProcess)
            if ($windowProcess -eq $process.Id -and [AriadneControlWindowSmoke]::IsWindowVisible($handle) -eq $Visible) {
                return $handle
            }
        }
        Start-Sleep -Milliseconds 100
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "Control window did not reach visible=$Visible."
}
function Assert-Exit($process) {
    if (-not $process.WaitForExit(15000)) { throw 'Control did not exit.' }
    $process.Refresh()
    if ($process.ExitCode -ne 0) { throw "Control crashed: $($process.ExitCode)" }
}
function Assert-Status($process) {
    $pipeName = 'ariadne-iphone-control-' + (Get-Process -Id $PID).SessionId
    $pipe = [System.IO.Pipes.NamedPipeClientStream]::new('.', $pipeName, [System.IO.Pipes.PipeDirection]::InOut)
    try {
        $pipe.Connect(5000)
        $reader = [System.IO.StreamReader]::new($pipe)
        try {
            $reading = $reader.ReadLineAsync()
            if (-not $reading.Wait(5000)) { throw 'Background control status timed out.' }
            $status = $reading.Result | ConvertFrom-Json
            if (-not $status.running -or $status.processId -ne $process.Id -or $status.phase -eq 'closing') {
                throw 'Background input service lost its status endpoint.'
            }
        } finally { $reader.Dispose() }
    } finally { $pipe.Dispose() }
}
$control = $null
$owner = $null
try {
    $owner = Start-Process powershell.exe -ArgumentList @('-NoProfile', '-Command', 'Start-Sleep -Seconds 120') -WindowStyle Hidden -PassThru
    $control = Start-Process -FilePath $Executable -ArgumentList @('--owner-pid', $owner.Id, '--background') -WindowStyle Hidden -PassThru
    $handle = Wait-Window $control $false
    Assert-Status $control
    Write-Output 'PASS background startup creates a hidden input IPC window'
    $second = Start-Process -FilePath $Executable -ArgumentList @('--owner-pid', $owner.Id, '--background') -WindowStyle Hidden -PassThru
    Assert-Exit $second
    $null = Wait-Window $control $false
    Write-Output 'PASS background singleton does not reveal the control window'
    $second = Start-Process -FilePath $Executable -ArgumentList @('--owner-pid', $owner.Id) -WindowStyle Hidden -PassThru
    Assert-Exit $second
    $handle = Wait-Window $control $true
    $control.Refresh()
    if ($control.HasExited) { throw 'Second invocation closed the first control.' }
    Write-Output 'PASS singleton activation'
    if (-not [AriadneControlWindowSmoke]::PostMessage($handle, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)) {
        throw 'Cannot close control window.'
    }
    $null = Wait-Window $control $false
    if ($control.WaitForExit(1000)) { throw 'Closing settings stopped the control service.' }
    Assert-Status $control
    Write-Output 'PASS closing settings hides the window and keeps the control service alive'
    $second = Start-Process -FilePath $Executable -WindowStyle Hidden -PassThru
    Assert-Exit $second
    $null = Wait-Window $control $true
    Write-Output 'PASS hidden settings can be reopened'
    Stop-Process -Id $owner.Id
    Assert-Exit $control
    Write-Output 'PASS owner exit closes control'
} finally {
    if ($control -and -not $control.HasExited) {
        if ($owner -and -not $owner.HasExited) { Stop-Process -Id $owner.Id }
        if (-not $control.WaitForExit(15000)) { Stop-Process -Id $control.Id }
    }
    if ($owner -and -not $owner.HasExited) { Stop-Process -Id $owner.Id }
}
