using System.Runtime.InteropServices;
using Ariadne.iPhoneControl;
using BleHid.Core;

internal static class BridgeTests
{
    public static void Run()
    {
        var first = CreateWindowExW(0, "STATIC", "Screen input test", 0, 0, 0, 1, 1,
            IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero);
        var second = CreateWindowExW(0, "STATIC", "Second screen input test", 0, 0, 0, 1, 1,
            IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero);
        if (first == IntPtr.Zero || second == IntPtr.Zero) throw new Exception("Cannot create test source windows.");
        var received = new List<ScreenPointerPacket>();
        Task Receive(ScreenPointerPacket packet) { received.Add(packet); return Task.CompletedTask; }
        var bridge = new ScreenControlBridge(pid => pid == Environment.ProcessId, Receive) { Readiness = 1 };
        var data = Marshal.AllocHGlobal(20);
        var header = Marshal.AllocHGlobal(Marshal.SizeOf<CopyData>());
        try
        {
            int Dispatch(ScreenPointerKind kind, IntPtr source, uint tag = ScreenPointerPacket.DataTag,
                int x = 1000, int y = 2000)
            {
                Marshal.StructureToPtr(new ScreenPointerPacket(1, kind, x, y, 0), data, false);
                Marshal.StructureToPtr(new CopyData { Tag = tag, Length = 20, Data = data }, header, false);
                var handled = false;
                var response = bridge.Handle(source, header, ref handled);
                if (!handled) throw new Exception("Screen IPC message was not handled.");
                return response.ToInt32();
            }
            Check(Dispatch(ScreenPointerKind.Query, first) == 1, "Relative-only status must require re-pairing.");
            Check(Dispatch(ScreenPointerKind.Down, first) == 0 && received.Count == 0,
                "Relative-only host must not receive direct clicks.");
            bridge.Readiness = 2;
            Check(Dispatch(ScreenPointerKind.Down, first, 0) == 0, "Unknown IPC tag must be rejected.");
            Check(Dispatch(ScreenPointerKind.Down, first) == 1 && received.Count == 0,
                "Selected absolute host must accept direct clicks.");
            bridge.DispatchPendingAsync().GetAwaiter().GetResult();
            Check(received.Count == 1, "Bluetooth work must wait until the synchronous message returns.");
            Check(Dispatch(ScreenPointerKind.Down, second) == 0 && received.Count == 1,
                "A second live window must not take over the selected source.");
            Check(Dispatch(ScreenPointerKind.Cancel, first) == 1 && received.Count == 1,
                "Cancel must release the source.");
            bridge.DispatchPendingAsync().GetAwaiter().GetResult();
            Check(received[^1].Kind == ScreenPointerKind.Cancel, "Cancel must be dispatched outside the message.");
            Check(Dispatch(ScreenPointerKind.Down, second) == 1, "A released source must allow a new video window.");
            bridge.ClearSource();
            bridge.DispatchPendingAsync().GetAwaiter().GetResult();
            Check(received.Count == 2, "Changing targets must discard undelivered input.");
            Check(Dispatch(ScreenPointerKind.Down, second) == 1, "Source must bind again after a reset.");
            DestroyWindow(second);
            second = IntPtr.Zero;
            Check(!bridge.SourceAlive, "Destroyed video window must trigger a release.");
            var reject = new ScreenControlBridge(_ => false, Receive) { Readiness = 2 };
            var handled = false;
            Check(reject.Handle(first, header, ref handled) == IntPtr.Zero,
                "Unapproved process must not control the selected phone.");

            // Real transport backpressure: enqueue a complete drag while the first BLE position is blocked.
            var unblock = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
            var events = new List<(string Kind, MouseButtons Buttons, int X, int Y)>();
            var input = new MouseInput(
                (buttons, x, y, wheel) => { events.Add(("relative", buttons, x, y)); return Task.CompletedTask; },
                async (buttons, x, y) =>
                {
                    events.Add(("absolute", buttons, x, y));
                    if (events.Count == 1) await unblock.Task;
                });
            bridge = new ScreenControlBridge(pid => pid == Environment.ProcessId, async packet =>
            {
                if (packet.Kind == ScreenPointerKind.Move) await input.PositionAsync(packet.X, packet.Y, 1);
                else await input.ScreenButtonAsync(packet.X, packet.Y, packet.Kind == ScreenPointerKind.Down, 1);
            }) { Readiness = 2 };
            Dispatch(ScreenPointerKind.Down, first);
            var draining = bridge.DispatchPendingAsync();
            Dispatch(ScreenPointerKind.Move, first, x: 3000, y: 4000);
            Dispatch(ScreenPointerKind.Move, first, x: 5000, y: 6000);
            Dispatch(ScreenPointerKind.Up, first, x: 7000, y: 8000);
            Dispatch(ScreenPointerKind.Move, first, x: 9000, y: 10000);
            bridge.DispatchPendingAsync().GetAwaiter().GetResult();
            Check(events.Count == 1, "Reentrant timer ticks must not dispatch past a blocked press.");
            unblock.SetResult();
            draining.WaitAsync(TimeSpan.FromSeconds(5)).GetAwaiter().GetResult();
            Check(events.SequenceEqual(new[]
            {
                ("absolute", MouseButtons.None, 1000, 2000),
                ("relative", MouseButtons.Left, 0, 0),
                ("absolute", MouseButtons.Left, 5000, 6000),
                ("absolute", MouseButtons.Left, 7000, 8000),
                ("relative", MouseButtons.None, 0, 0),
                ("absolute", MouseButtons.None, 9000, 10000)
            }), "A fast drag must preserve its final held movement before release, even with a slow BLE press.");
            Console.WriteLine("PASS delayed BLE press preserves drag movement, coalesces adjacent moves and releases before hover");
        }
        finally
        {
            Marshal.FreeHGlobal(data);
            Marshal.FreeHGlobal(header);
            DestroyWindow(first);
            if (second != IntPtr.Zero) DestroyWindow(second);
        }
        Console.WriteLine("PASS screen bridge readiness, source validation, cancellation and window lifetime");
    }

    private static void Check(bool ok, string message) { if (!ok) throw new Exception(message); }
    [StructLayout(LayoutKind.Sequential)]
    private struct CopyData { public nuint Tag; public int Length; public IntPtr Data; }
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern IntPtr CreateWindowExW(uint exStyle, string className, string title,
        uint style, int x, int y, int w, int h, IntPtr parent, IntPtr menu, IntPtr instance, IntPtr param);
    [DllImport("user32.dll")]
    private static extern bool DestroyWindow(IntPtr window);
}
