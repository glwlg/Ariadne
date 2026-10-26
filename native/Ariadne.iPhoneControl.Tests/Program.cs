using Ariadne.iPhoneControl;
using BleHid.Core;

static void Assert(bool condition, string message)
{
    if (!condition) throw new Exception(message);
}

var sent = new List<(MouseButtons Buttons, int X, int Y)>();
Task Send(MouseButtons buttons, int x, int y, int wheel)
{
    sent.Add((buttons, x, y));
    return Task.CompletedTask;
}
Task IgnorePosition(MouseButtons buttons, int x, int y) => Task.CompletedTask;
var input = new MouseInput(Send, IgnorePosition);
await input.ButtonAsync(true);
await input.MoveAsync(10, 20);
await input.ButtonAsync(false);
Assert(sent.SequenceEqual(new[]
{
    (MouseButtons.Left, 0, 0), (MouseButtons.Left, 10, 20), (MouseButtons.None, 0, 0)
}), "Pointer movement must retain the held button until release.");
Console.WriteLine("PASS pointer press / move / release");

sent.Clear();
await input.SwipeAsync(-1, 317, 1);
Assert(sent.First().Buttons == MouseButtons.Left, "Swipe must start with a press.");
Assert(sent.Where(r => r.Buttons == MouseButtons.Left).Sum(r => r.Y) == -317,
    "Swipe distance must not lose remainder pixels.");
Assert(sent.FindIndex(r => r.Buttons == MouseButtons.None) == 13,
    "Release must precede cursor repositioning.");
Assert(sent.Sum(r => r.Y) == 0 && sent.Last().Buttons == MouseButtons.None,
    "Reposition must return the pointer without a held button.");
Console.WriteLine("PASS swipe direction / distance / release / reposition");

var entered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
var unblock = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
sent.Clear();
var cancellable = new MouseInput(async (buttons, x, y, wheel) =>
{
    sent.Add((buttons, x, y));
    if (buttons == MouseButtons.Left && y != 0)
    {
        entered.TrySetResult();
        await unblock.Task;
    }
}, IgnorePosition);
var swipe = cancellable.SwipeAsync(1, 300, 1);
await entered.Task.WaitAsync(TimeSpan.FromSeconds(5));
var queuedPress = cancellable.ButtonAsync(true);
var reset = cancellable.ResetAsync();
unblock.SetResult();
await Task.WhenAll(swipe, queuedPress, reset).WaitAsync(TimeSpan.FromSeconds(5));
var firstRelease = sent.FindIndex(r => r.Buttons == MouseButtons.None);
Assert(firstRelease >= 0 && sent.Skip(firstRelease).All(r => r.Buttons == MouseButtons.None),
    "Cancelled queued input must never press again after reset.");
Console.WriteLine("PASS cancellation clears queued presses and releases a running swipe");

var attemptedRelease = false;
var failing = new MouseInput((buttons, x, y, wheel) =>
{
    if (buttons == MouseButtons.None) attemptedRelease = true;
    if (y != 0) throw new IOException("Simulated Bluetooth failure");
    return Task.CompletedTask;
}, IgnorePosition);
try { await failing.SwipeAsync(-1, 300, 1); }
catch (IOException) { }
Assert(attemptedRelease && !failing.GestureRunning, "Failed gesture must attempt release and clear busy state.");
Console.WriteLine("PASS failed transmission releases button");

var report = HidReports.Mouse(MouseButtons.Left, -400, 317, 0);
Assert(report.Length == 6 && BitConverter.ToInt16(report, 1) == -400 &&
       BitConverter.ToInt16(report, 3) == 317 && report[0] == 1,
    "HID coordinates must use signed 16-bit little endian values without a report ID prefix.");
Console.WriteLine("PASS HID report encoding");

var absolute = HidReports.AbsoluteMouse(MouseButtons.Left, 0x1234, 32767);
Assert(absolute.SequenceEqual(new byte[] { 1, 0x34, 0x12, 0xff, 0x7f }),
    "Absolute report must carry button bits and unsigned 16-bit coordinates without an ID prefix.");
var rejected = false;
try { HidReports.AbsoluteMouse(MouseButtons.None, -1, 5); }
catch (ArgumentOutOfRangeException) { rejected = true; }
Assert(rejected, "Invalid screen coordinates must be rejected.");
Assert(HidDescriptors.ReportMapValue.AsSpan().IndexOf(new byte[] { 0x85, HidDescriptors.AbsoluteReportId }) >= 0,
    "Report map must declare the absolute input report.");
Assert(new ScreenPointerPacket(1, ScreenPointerKind.Wheel, 123, 456, -1).IsValid &&
       !new ScreenPointerPacket(2, ScreenPointerKind.Move, 123, 456, 0).IsValid &&
       !new ScreenPointerPacket(1, ScreenPointerKind.Move, 32768, 456, 0).IsValid &&
       !new ScreenPointerPacket(1, ScreenPointerKind.Down, 123, 456, 1).IsValid &&
       !new ScreenPointerPacket(1, (ScreenPointerKind)100, 123, 456, 0).IsValid &&
       System.Runtime.InteropServices.Marshal.SizeOf<ScreenPointerPacket>() == 20,
    "Screen input wire format must validate version, type, ranges and payload size.");
Console.WriteLine("PASS absolute report and screen packet validation");

var screenEvents = new List<(string Kind, MouseButtons Buttons, int X, int Y)>();
var screen = new MouseInput(
    (buttons, x, y, wheel) => { screenEvents.Add(("relative", buttons, x, y)); return Task.CompletedTask; },
    (buttons, x, y) => { screenEvents.Add(("absolute", buttons, x, y)); return Task.CompletedTask; });
await screen.ScreenButtonAsync(1000, 2000, true, 1);
await screen.PositionAsync(4000, 5000, 1);
await screen.ScreenButtonAsync(6000, 7000, false, 1);
Assert(screenEvents.SequenceEqual(new[]
{
    ("absolute", MouseButtons.None, 1000, 2000),
    ("relative", MouseButtons.Left, 0, 0),
    ("absolute", MouseButtons.Left, 4000, 5000),
    ("absolute", MouseButtons.Left, 6000, 7000),
    ("relative", MouseButtons.None, 0, 0)
}), "Direct drag must position before pressing, retain press during movement, and release at the final position.");
screenEvents.Clear();
await screen.ScreenSwipeAsync(16384, -1, 1);
Assert(screenEvents.First() == ("absolute", MouseButtons.None, 16384, 24575) &&
       screenEvents.Last() == ("relative", MouseButtons.None, 0, 0) &&
       screenEvents[^2] == ("absolute", MouseButtons.Left, 16384, 8192),
    "Wheel down must swipe upward in absolute screen space and release the relative mouse.");
Console.WriteLine("PASS direct click, drag and wheel swipe sequencing");

var positioning = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
var continuePosition = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
var screenButtons = new List<MouseButtons>();
var cancelScreen = new MouseInput(
    (buttons, x, y, wheel) => { screenButtons.Add(buttons); return Task.CompletedTask; },
    async (buttons, x, y) => { positioning.TrySetResult(); await continuePosition.Task; });
var pendingDown = cancelScreen.ScreenButtonAsync(500, 600, true, 1);
await positioning.Task.WaitAsync(TimeSpan.FromSeconds(5));
var screenReset = cancelScreen.ResetAsync();
continuePosition.SetResult();
await Task.WhenAll(pendingDown, screenReset).WaitAsync(TimeSpan.FromSeconds(5));
Assert(screenButtons.SequenceEqual(new[] { MouseButtons.None }),
    "Focus loss while positioning must cancel the queued press and send a release.");
Console.WriteLine("PASS screen cancellation before press");

var screenReleaseAttempted = false;
var failedScreen = new MouseInput(
    (buttons, x, y, wheel) => { if (buttons == MouseButtons.None) screenReleaseAttempted = true; return Task.CompletedTask; },
    (buttons, x, y) => buttons == MouseButtons.Left
        ? throw new IOException("Simulated absolute notification failure") : Task.CompletedTask);
try { await failedScreen.ScreenSwipeAsync(10000, -1, 1); }
catch (IOException) { }
Assert(screenReleaseAttempted && !failedScreen.GestureRunning,
    "Failure during an absolute swipe must release the relative mouse and clear busy state.");
Console.WriteLine("PASS absolute transmission failure releases button");
if (OperatingSystem.IsWindows())
{
    await StatusTests.RunAsync();
    BridgeTests.Run();
    SynchronousMessageTests.Run();
}
TargetPreferenceTests.Run();
await KeyboardTests.RunAsync();
