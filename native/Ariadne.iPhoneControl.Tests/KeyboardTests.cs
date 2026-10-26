using Ariadne.iPhoneControl;
using BleHid.Core;

internal static class KeyboardTests
{
    public static async Task RunAsync()
    {
        var reports = new List<byte[]>();
        var input = new KeyboardInput((modifiers, keys) =>
        {
            reports.Add(HidReports.Keyboard(modifiers, keys));
            return Task.CompletedTask;
        });
        await input.KeyAsync(224, true); // Left Control
        await input.KeyAsync(4, true);   // A
        await input.KeyAsync(4, true);   // Desktop auto-repeat must not duplicate presses.
        await input.KeyAsync(228, true); // Right Control
        await input.KeyAsync(224, false);
        await input.KeyAsync(4, false);
        await input.KeyAsync(228, false);
        Check(reports.Count == 6 && reports[1][0] == 1 && reports[1][2] == 4 &&
            reports[2][0] == 0x11 && reports[3][0] == 0x10 &&
            reports[^1].All(b => b == 0), "Combos must retain independent left/right modifiers and release all keys.");
        Console.WriteLine("PASS keyboard modifiers, combos, repeat suppression and release encoding");

        reports.Clear();
        for (var key = 4; key <= 10; key++) await input.KeyAsync(key, true);
        Check(reports[^1].Skip(2).SequenceEqual(new byte[] { 1, 1, 1, 1, 1, 1 }),
            "More than six held keys must report HID rollover.");
        await input.KeyAsync(4, false);
        Check(reports[^1].Skip(2).SequenceEqual(new byte[] { 5, 6, 7, 8, 9, 10 }),
            "Leaving rollover must restore the actual held keys.");
        await input.ResetAsync();
        Check(reports[^1].All(b => b == 0), "Reset must release every key.");

        var entered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var unblock = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        reports.Clear();
        input = new KeyboardInput(async (modifiers, keys) =>
        {
            reports.Add(HidReports.Keyboard(modifiers, keys));
            if (reports.Count == 1) { entered.TrySetResult(); await unblock.Task; }
        });
        var pressing = input.KeyAsync(225, true);
        await entered.Task.WaitAsync(TimeSpan.FromSeconds(5));
        var queued = input.KeyAsync(4, true);
        var reset = input.ResetAsync();
        unblock.SetResult();
        await Task.WhenAll(pressing, queued, reset).WaitAsync(TimeSpan.FromSeconds(5));
        Check(reports.Count == 2 && reports[^1].All(b => b == 0),
            "Focus loss must discard queued keys while BLE is blocked, then release the held modifier.");
        Console.WriteLine("PASS keyboard rollover and cancellation under Bluetooth backpressure");

        var releaseAttempted = false;
        input = new KeyboardInput((modifiers, keys) =>
        {
            if (keys.Length > 0) throw new IOException("Simulated key transmission failure");
            releaseAttempted = modifiers == KeyModifiers.None;
            return Task.CompletedTask;
        });
        try { await input.KeyAsync(4, true); }
        catch (IOException) { }
        await input.ResetAsync();
        Check(releaseAttempted, "Failed keyboard input must still allow a release report.");
        Check(new ScreenPointerPacket(1, ScreenPointerKind.KeyDown, 4, 0, 0).IsValid &&
            new ScreenPointerPacket(1, ScreenPointerKind.KeyUp, 231, 0, 0).IsValid &&
            !new ScreenPointerPacket(1, ScreenPointerKind.KeyDown, 0, 0, 0).IsValid &&
            !new ScreenPointerPacket(1, ScreenPointerKind.KeyDown, 102, 0, 0).IsValid &&
            !new ScreenPointerPacket(1, ScreenPointerKind.KeyDown, 232, 0, 0).IsValid &&
            !new ScreenPointerPacket(1, ScreenPointerKind.KeyDown, 4, 1, 0).IsValid &&
            !new ScreenPointerPacket(1, ScreenPointerKind.KeyUp, 4, 0, 1).IsValid,
            "Keyboard IPC must reject invalid usages and extra payload.");
        Console.WriteLine("PASS keyboard transmission failure recovery and IPC validation");
    }

    private static void Check(bool condition, string message)
    {
        if (!condition) throw new Exception(message);
    }
}
