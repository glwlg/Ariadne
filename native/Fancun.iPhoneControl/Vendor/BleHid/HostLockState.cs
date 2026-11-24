namespace BleHid.Core;

/// <summary>
/// Per-host lock-key bookkeeping, kept separate from the GATT plumbing so the rules can be
/// tested without a live Bluetooth stack.
///
/// Two sources feed it. A host that writes the HID LED output report tells us its real state;
/// anything else leaves us counting the lock keys we forwarded and assuming the host started
/// clear. Measurement always wins, and once a host is measured the inference is abandoned
/// rather than run alongside: guessing next to a known-good signal only makes the display
/// flicker, and is wrong whenever the host declines to act on a key.
/// </summary>
internal sealed class HostLockState
{
    private readonly Dictionary<string, Entry> _hosts = new(StringComparer.OrdinalIgnoreCase);
    private readonly object _gate = new();

    private sealed class Entry
    {
        public LockKeys Keys;
        public bool Measured;
        public bool NumpadUsed;
    }

    public LockKeys For(string deviceId)
    {
        lock (_gate) return _hosts.TryGetValue(deviceId, out var entry) ? entry.Keys : LockKeys.None;
    }

    /// <summary>True once the host has written an LED report, so its state is fact not guesswork.</summary>
    public bool Measured(string deviceId)
    {
        lock (_gate) return _hosts.TryGetValue(deviceId, out var entry) && entry.Measured;
    }

    /// <summary>True once the numpad has been used on this host.</summary>
    public bool NumpadUsed(string deviceId)
    {
        lock (_gate) return _hosts.TryGetValue(deviceId, out var entry) && entry.NumpadUsed;
    }

    /// <summary>
    /// Records lock keys we forwarded to a host. Ignored for hosts that report their own state,
    /// which will tell us the truth in a moment anyway.
    /// </summary>
    public void NoteForwarded(string deviceId, LockKeys pressed)
    {
        if (pressed == LockKeys.None) return;
        lock (_gate)
        {
            var entry = Get(deviceId);
            if (entry.Measured) return;
            entry.Keys ^= pressed;
        }
    }

    public void NoteNumpadUsed(string deviceId)
    {
        lock (_gate) Get(deviceId).NumpadUsed = true;
    }

    /// <summary>
    /// Records an LED report from a host. Returns whether this was the first report from it and
    /// whether the state changed, so the caller can log the transitions without duplicating them.
    /// </summary>
    public (bool FirstReport, bool Changed, LockKeys State) NoteLedReport(string deviceId, byte leds)
    {
        var state = (LockKeys)(leds & HidDescriptors.LedTrackedMask);
        lock (_gate)
        {
            var entry = Get(deviceId);
            var firstReport = !entry.Measured;
            var changed = firstReport || entry.Keys != state;
            entry.Measured = true;
            entry.Keys = state;
            return (firstReport, changed, state);
        }
    }

    /// <summary>
    /// Drops everything known about a host. The host owns its lock state and can change it
    /// while we are not attached, so a reconnect must not inherit a stale belief.
    /// </summary>
    public void Forget(string deviceId)
    {
        lock (_gate) _hosts.Remove(deviceId);
    }

    private Entry Get(string deviceId)
    {
        if (_hosts.TryGetValue(deviceId, out var entry)) return entry;
        entry = new Entry();
        _hosts[deviceId] = entry;
        return entry;
    }
}
