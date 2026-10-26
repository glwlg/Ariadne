using System.Runtime.InteropServices;

namespace Ariadne.iPhoneControl;

// WM_COPYDATA transports input; the selected HID host remains owned by the control window.
internal sealed class ScreenControlBridge(
    Func<int, bool> isReceiver,
    Func<ScreenPointerPacket, Task> receive)
{
    private IntPtr _source;
    private int _processId;
    private readonly LinkedList<ScreenPointerPacket> _pending = new();
    private bool _dispatching;

    // WM_COPYDATA is input-synchronous: neither COM queries nor Bluetooth sends are allowed here.
    public int Readiness { get; set; }
    public bool KeyboardReady { get; set; }

    public async Task DispatchPendingAsync()
    {
        if (_dispatching) return;
        _dispatching = true;
        try
        {
            while (_pending.First is { } first)
            {
                _pending.RemoveFirst();
                await receive(first.Value);
            }
        }
        finally { _dispatching = false; }
    }

    public bool SourceAlive => _source == IntPtr.Zero ||
        IsWindow(_source) && GetWindowThreadProcessId(_source, out var pid) != 0 && pid == _processId;

    public void ClearSource() { _source = IntPtr.Zero; _processId = 0; _pending.Clear(); }

    public IntPtr Handle(IntPtr source, IntPtr data, ref bool handled)
    {
        handled = true;
        if (source == IntPtr.Zero || data == IntPtr.Zero) return IntPtr.Zero;
        var copy = Marshal.PtrToStructure<CopyData>(data);
        if (copy.Tag != ScreenPointerPacket.DataTag ||
            copy.Length != Marshal.SizeOf<ScreenPointerPacket>() || copy.Data == IntPtr.Zero)
            return IntPtr.Zero;
        var packet = Marshal.PtrToStructure<ScreenPointerPacket>(copy.Data);
        if (!packet.IsValid || GetWindowThreadProcessId(source, out var pid) == 0)
            return IntPtr.Zero;
        if (source != _source || pid != _processId)
        {
            if (!isReceiver(pid)) return IntPtr.Zero;
            // A second video window cannot take over a gesture already in progress.
            if (_source != IntPtr.Zero && SourceAlive) return IntPtr.Zero;
        }
        if (packet.Kind == ScreenPointerKind.Query) return new IntPtr(Readiness);
        if (packet.Kind == ScreenPointerKind.KeyboardQuery)
            return new IntPtr(Readiness == 2 && KeyboardReady ? 1 : 0);
        if (packet.Kind == ScreenPointerKind.Cancel)
        {
            if (source == _source) { ClearSource(); _pending.AddLast(packet); }
            return new IntPtr(1);
        }
        if (Readiness != 2) return IntPtr.Zero;
        if (packet.IsKeyboard && !KeyboardReady) return IntPtr.Zero;
        _source = source;
        _processId = pid;
        // Coalesce movement only within one press/release segment, including under BLE backpressure.
        if (packet.Kind == ScreenPointerKind.Move && _pending.Last is { Value.Kind: ScreenPointerKind.Move } last)
            last.Value = packet;
        else _pending.AddLast(packet);
        return new IntPtr(1);
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct CopyData { public nuint Tag; public int Length; public IntPtr Data; }

    [DllImport("user32.dll")]
    private static extern bool IsWindow(IntPtr window);
    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr window, out int processId);
}
