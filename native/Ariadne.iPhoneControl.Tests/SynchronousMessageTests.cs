using System.Diagnostics;
using System.Runtime.InteropServices;
using Ariadne.iPhoneControl;

internal static class SynchronousMessageTests
{
    public static void Run()
    {
        var window = CreateWindowExW(0, "STATIC", "Screen IPC regression", 0, 0, 0, 1, 1,
            IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero);
        if (window == IntPtr.Zero) throw new Exception("Cannot create IPC test window.");
        var received = new List<ScreenPointerKind>();
        var bridge = new ScreenControlBridge(pid => pid == Environment.ProcessId, packet =>
        {
            if (InSendMessageEx(IntPtr.Zero) != 0)
                throw new Exception("Bluetooth callback ran inside a synchronous message.");
            received.Add(packet.Kind);
            return Task.CompletedTask;
        }) { Readiness = 2 };
        var synchronous = false;
        IntPtr original = IntPtr.Zero;
        WindowProc callback = (hwnd, message, wParam, lParam) =>
        {
            if (message != 0x004A) return CallWindowProcW(original, hwnd, message, wParam, lParam);
            synchronous |= InSendMessageEx(IntPtr.Zero) != 0;
            var handled = false;
            return bridge.Handle(wParam, lParam, ref handled);
        };
        original = SetWindowLongPtrW(window, -4, Marshal.GetFunctionPointerForDelegate(callback));
        try
        {
            foreach (var kind in new[] { ScreenPointerKind.Query, ScreenPointerKind.Down, ScreenPointerKind.Up })
            {
                var sending = Task.Run(() =>
                {
                    var packet = Marshal.AllocHGlobal(20);
                    var header = Marshal.AllocHGlobal(Marshal.SizeOf<CopyData>());
                    try
                    {
                        Marshal.StructureToPtr(new ScreenPointerPacket(1, kind, 1234, 2345, 0), packet, false);
                        Marshal.StructureToPtr(new CopyData
                        {
                            Tag = ScreenPointerPacket.DataTag, Length = 20, Data = packet
                        }, header, false);
                        if (SendMessageTimeoutW(window, 0x004A, window, header, 0x0003, 2000, out var reply) == IntPtr.Zero ||
                            reply != (nuint)(kind == ScreenPointerKind.Query ? 2 : 1))
                            throw new Exception("Cross-thread screen IPC failed.");
                    }
                    finally { Marshal.FreeHGlobal(packet); Marshal.FreeHGlobal(header); }
                });
                var deadline = Stopwatch.StartNew();
                while (!sending.IsCompleted && deadline.Elapsed < TimeSpan.FromSeconds(3))
                {
                    // PeekMessage dispatches incoming sent messages before inspecting the posted queue.
                    PeekMessageW(out _, IntPtr.Zero, 0, 0, 0);
                    Thread.Sleep(1);
                }
                sending.GetAwaiter().GetResult();
                if (received.Count != 0) throw new Exception("Input ran before the IPC sender returned.");
            }
            bridge.DispatchPendingAsync().GetAwaiter().GetResult();
            if (!synchronous || !received.SequenceEqual(new[] { ScreenPointerKind.Down, ScreenPointerKind.Up }))
                throw new Exception("Deferred IPC must preserve copied press/release packets after the sender frees them.");
        }
        finally
        {
            SetWindowLongPtrW(window, -4, original);
            DestroyWindow(window);
            GC.KeepAlive(callback);
        }
        Console.WriteLine("PASS real cross-thread WM_COPYDATA defers Bluetooth work and preserves press/release order");
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct CopyData { public nuint Tag; public int Length; public IntPtr Data; }
    [StructLayout(LayoutKind.Sequential)]
    private struct Message { public IntPtr Window; public uint Id; public nuint WParam; public IntPtr LParam; public uint Time; public int X, Y; public uint Private; }
    private delegate IntPtr WindowProc(IntPtr window, uint message, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern IntPtr CreateWindowExW(uint exStyle, string className, string title,
        uint style, int x, int y, int w, int h, IntPtr parent, IntPtr menu, IntPtr instance, IntPtr param);
    [DllImport("user32.dll")] private static extern bool DestroyWindow(IntPtr window);
    [DllImport("user32.dll")] private static extern uint InSendMessageEx(IntPtr reserved);
    [DllImport("user32.dll")] private static extern IntPtr SetWindowLongPtrW(IntPtr window, int index, IntPtr value);
    [DllImport("user32.dll")] private static extern IntPtr CallWindowProcW(IntPtr proc, IntPtr window, uint message, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] private static extern IntPtr SendMessageTimeoutW(IntPtr window, uint message, IntPtr wParam, IntPtr lParam, uint flags, uint timeout, out nuint result);
    [DllImport("user32.dll")] private static extern bool PeekMessageW(out Message message, IntPtr window, uint first, uint last, uint remove);
}
