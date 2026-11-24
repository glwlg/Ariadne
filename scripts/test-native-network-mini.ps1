param(
    [Parameter(Mandatory = $true)][string]$ExePath,
    [int]$DurationSeconds = 120,
    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
$pipeName = 'fancun-network-mini-test-' + [Guid]::NewGuid().ToString('N')
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class NetworkMiniSmokeNative {
    private delegate bool EnumWindowCallback(IntPtr hwnd, IntPtr parameter);
    [DllImport("user32.dll")] private static extern bool EnumWindows(EnumWindowCallback callback, IntPtr parameter);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint pid);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] private static extern int GetWindowText(IntPtr hwnd, System.Text.StringBuilder text, int maxCount);
    public static IntPtr FindWindowForProcess(int processId) {
        IntPtr result=IntPtr.Zero;
        EnumWindows((hwnd, parameter) => {
            uint pid; GetWindowThreadProcessId(hwnd, out pid);
            if(pid != processId) return true;
            var text=new System.Text.StringBuilder(128); GetWindowText(hwnd,text,text.Capacity);
            if(!text.ToString().StartsWith("Fancun - ")) return true;
            result=hwnd; return false;
        },IntPtr.Zero);
        return result;
    }
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll", EntryPoint="GetWindowLongPtrW")]
    public static extern IntPtr GetWindowLongPtr(IntPtr hwnd, int index);
    [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hwnd, uint msg, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out Rect rect);
    [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left,Top,Right,Bottom; }
}
'@

function Connect-Pipe([string]$Name, [int]$Timeout = 3000) {
    $pipe = [IO.Pipes.NamedPipeClientStream]::new('.', $Name, [IO.Pipes.PipeDirection]::InOut, [IO.Pipes.PipeOptions]::Asynchronous)
    try { $pipe.Connect($Timeout); return $pipe }
    catch { $pipe.Dispose(); throw }
}

function Send-Mini($State) {
    $pipe = Connect-Pipe ($pipeName + '-network-mini')
    try {
        $writer = [IO.StreamWriter]::new($pipe, [Text.UTF8Encoding]::new($false), 1024, $true)
        $reader = [IO.StreamReader]::new($pipe, [Text.Encoding]::UTF8, $false, 1024, $true)
        $writer.AutoFlush = $true
        $writer.WriteLine(($State | ConvertTo-Json -Compress))
        $read = $reader.ReadLineAsync()
        if (!$read.Wait(3000)) { throw 'Native mini response timed out' }
        $reply = $read.Result | ConvertFrom-Json
        if (!$reply.ok) { throw "Native mini rejected state: $($reply.message)" }
        return $reply
    } finally { $pipe.Dispose() }
}

function Assert-Check([bool]$Condition, [string]$Message) {
    if (!$Condition) { throw $Message }
}

$process = Start-Process -FilePath ([IO.Path]::GetFullPath($ExePath)) -ArgumentList @('--pipe', $pipeName) -PassThru
$capturePipe = $null
try {
    # Connect waits for startup. Deliberately hold the capture listener halfway
    # through a ping while updating the mini through its independent pipe.
    $capturePipe = Connect-Pipe $pipeName 15000
    $captureWriter = [IO.StreamWriter]::new($capturePipe, [Text.UTF8Encoding]::new($false), 1024, $true)
    $captureWriter.AutoFlush = $true
    $captureWriter.Write('{"command":')
    $foreground = [NetworkMiniSmokeNative]::GetForegroundWindow()
    $state = @{
        visible = $true; x = 30; y = 60; width = 154; height = 38
        uploadBytesPerSecond = 1280; downloadBytesPerSecond = 1258291.2
        adapterName = 'Test adapter'; error = ''
    }
    $null = Send-Mini $state
    $hwnd = [NetworkMiniSmokeNative]::FindWindowForProcess($process.Id)
    Assert-Check ($hwnd -ne [IntPtr]::Zero) 'Native mini window was not created'
    Assert-Check ([NetworkMiniSmokeNative]::IsWindowVisible($hwnd)) 'Mini was not visible'
    Assert-Check ([NetworkMiniSmokeNative]::GetForegroundWindow() -eq $foreground) 'Mini stole foreground focus'
    $style = [NetworkMiniSmokeNative]::GetWindowLongPtr($hwnd, -20).ToInt64()
    Assert-Check (($style -band 0x08000080) -eq 0x08000080) 'Mini must be a non-activating tool window'
    Assert-Check (($style -band 0x00040000) -eq 0) 'Mini must not have a taskbar button'
    $rect = [NetworkMiniSmokeNative+Rect]::new()
    $null = [NetworkMiniSmokeNative]::GetWindowRect($hwnd, [ref]$rect)
    Assert-Check ($rect.Left -eq 30 -and $rect.Top -eq 60 -and ($rect.Right-$rect.Left) -eq 154 -and ($rect.Bottom-$rect.Top) -eq 38) 'Native mini physical placement was incorrect'

    $state.visible = $false
    $null = Send-Mini $state
    Assert-Check (![NetworkMiniSmokeNative]::IsWindowVisible($hwnd)) 'Capture hiding did not hide mini'
    $captureWriter.WriteLine('"ping"}')
    $captureReader = [IO.StreamReader]::new($capturePipe, [Text.Encoding]::UTF8, $false, 1024, $true)
    Assert-Check (($captureReader.ReadLine() | ConvertFrom-Json).ok) 'Capture listener did not remain responsive'
    $capturePipe.Dispose(); $capturePipe = $null
    $state.visible = $true
    $null = Send-Mini $state
    Assert-Check ([NetworkMiniSmokeNative]::FindWindowForProcess($process.Id) -eq $hwnd) 'Mini recreation leaked a window'

    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    $element = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
    $textCondition = [System.Windows.Automation.PropertyCondition]::new(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Text)
    $texts = $element.FindAll([System.Windows.Automation.TreeScope]::Descendants, $textCondition)
    $names = @($texts | ForEach-Object { $_.Current.Name })
    Assert-Check ($names -contains '1.3 KB/s' -and $names -contains '1.2 MB/s') 'Native mini did not display the supplied rates'

    # Address the isolated HWND directly, without moving the user's cursor.
    for ($click = 0; $click -lt 2; $click++) {
        $null = [NetworkMiniSmokeNative]::PostMessage($hwnd, 0x0201, [IntPtr]1, [IntPtr]0x00080008)
        $null = [NetworkMiniSmokeNative]::PostMessage($hwnd, 0x0202, [IntPtr]0, [IntPtr]0x00080008)
        Start-Sleep -Milliseconds 50
    }
    Start-Sleep -Milliseconds 100
    Assert-Check ((Send-Mini $state).action -eq 'open-center') 'Double click did not request the process network center'
    Assert-Check ([NetworkMiniSmokeNative]::GetForegroundWindow() -eq $foreground) 'Double click activated the mini window'

    $samples = [Collections.Generic.List[object]]::new()
    $watch = [Diagnostics.Stopwatch]::StartNew()
    $nextSample = 0
    while ($watch.Elapsed.TotalSeconds -lt $DurationSeconds) {
        $state.uploadBytesPerSecond = 1280 + $watch.ElapsedMilliseconds
        $null = Send-Mini $state
        if ($watch.Elapsed.TotalSeconds -ge $nextSample) {
            $process.Refresh()
            $sample = [pscustomobject]@{
                seconds = [math]::Round($watch.Elapsed.TotalSeconds, 1)
                workingSetMB = [math]::Round($process.WorkingSet64 / 1MB, 1)
                privateMB = [math]::Round($process.PrivateMemorySize64 / 1MB, 1)
                handles = $process.HandleCount
            }
            $samples.Add($sample)
            Write-Host ($sample | ConvertTo-Json -Compress)
            $nextSample += 10
        }
        Start-Sleep -Milliseconds 500
    }
    $children = @(Get-CimInstance Win32_Process -Filter "ParentProcessId = $($process.Id)")
    Assert-Check (@($children | Where-Object { $_.Name -like '*webview*' }).Count -eq 0) 'Native host launched a WebView process'

    # Invoke the native context menu in this test process only. A queued close
    # must win over the still-visible traffic frame that collects the action.
    $null = [NetworkMiniSmokeNative]::PostMessage($hwnd, 0x007B, $hwnd, [IntPtr](-1))
    Start-Sleep -Milliseconds 100
    $ownProcess = [System.Windows.Automation.PropertyCondition]::new(
        [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
    $menuCondition = [System.Windows.Automation.PropertyCondition]::new(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::MenuItem)
    $roots = [System.Windows.Automation.AutomationElement]::RootElement.FindAll(
        [System.Windows.Automation.TreeScope]::Children, $ownProcess)
    $menuItems = @()
    foreach ($root in $roots) {
        foreach ($item in $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $menuCondition)) {
            $menuItems += $item
        }
    }
    Assert-Check ($menuItems.Count -eq 2) 'Native context menu did not expose open and close actions'
    $invoke = $menuItems[1].GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
    $invoke.Invoke()
    for ($attempt = 0; $attempt -lt 20 -and [NetworkMiniSmokeNative]::IsWindowVisible($hwnd); $attempt++) {
        Start-Sleep -Milliseconds 50
    }
    $closeReply = Send-Mini $state
    Assert-Check ($closeReply.action -eq 'close') "Native close action was not returned to the backend: $($closeReply.action)"
    Assert-Check (![NetworkMiniSmokeNative]::IsWindowVisible($hwnd)) 'An in-flight visible frame reopened the closed mini'
    $state.visible = $false
    $null = Send-Mini $state
    Assert-Check (![NetworkMiniSmokeNative]::IsWindowVisible($hwnd)) 'Final close did not hide mini'
    $report = [pscustomobject]@{
        passed = $true; durationSeconds = $DurationSeconds
        independentCapturePipe = $true; noFocusSteal = $true; ratesVerified = $true
        physicalPlacementVerified = $true; doubleClickVerified = $true
        contextMenuCloseVerified = $true
        webviewChildCount = 0; samples = $samples.ToArray()
    }
    if ($OutputPath) {
        $fullOutputPath = [IO.Path]::GetFullPath($OutputPath)
        $null = [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($fullOutputPath))
        $report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $fullOutputPath -Encoding UTF8
    }
    $report | ConvertTo-Json -Depth 6 -Compress
} finally {
    if ($capturePipe) {
        $captureWriter.WriteLine('"ping"}')
        $capturePipe.Dispose()
    }
    if (!$process.HasExited) {
        $shutdown = Connect-Pipe $pipeName
        try {
            $writer = [IO.StreamWriter]::new($shutdown, [Text.UTF8Encoding]::new($false), 1024, $true)
            $writer.AutoFlush = $true
            $writer.WriteLine('{"command":"shutdown"}')
            if (!$process.WaitForExit(5000)) { $process.Kill(); throw 'Isolated native host did not shut down' }
        } finally { $shutdown.Dispose() }
    }
    $process.Dispose()
}
