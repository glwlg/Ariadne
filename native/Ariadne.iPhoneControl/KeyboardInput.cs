using BleHid.Core;

namespace Ariadne.iPhoneControl;

// Keep physical keys held until release; the phone owns layout, input method and repeat.
public sealed class KeyboardInput(Func<KeyModifiers, byte[], Task> send)
{
    private readonly SemaphoreSlim _gate = new(1, 1);
    private readonly List<byte> _keys = [];
    private KeyModifiers _modifiers;
    private long _generation;

    public static bool IsUsage(int usage) => usage is >= 4 and <= 101 or >= 224 and <= 231;

    public async Task KeyAsync(int usage, bool down)
    {
        if (!IsUsage(usage)) throw new ArgumentOutOfRangeException(nameof(usage));
        var generation = _generation;
        await _gate.WaitAsync();
        try
        {
            if (generation != _generation) return;
            if (usage >= 224)
            {
                var bit = (KeyModifiers)(1 << (usage - 224));
                if (_modifiers.HasFlag(bit) == down) return;
                if (down) _modifiers |= bit;
                else _modifiers &= ~bit;
            }
            else
            {
                if (_keys.Contains((byte)usage) == down) return;
                if (down) _keys.Add((byte)usage);
                else _keys.Remove((byte)usage);
            }
            // HID boot keyboards report rollover rather than silently losing held keys.
            await send(_modifiers, _keys.Count > 6 ? [1, 1, 1, 1, 1, 1] : _keys.ToArray());
        }
        finally { _gate.Release(); }
    }

    public async Task ResetAsync()
    {
        _generation++;
        await _gate.WaitAsync();
        try
        {
            _keys.Clear();
            _modifiers = KeyModifiers.None;
            await send(KeyModifiers.None, []);
        }
        finally { _gate.Release(); }
    }
}
