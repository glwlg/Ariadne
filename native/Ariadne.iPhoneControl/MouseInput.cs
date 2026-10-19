using BleHid.Core;

namespace Ariadne.iPhoneControl;

// One writer preserves press/move/release order; cancellation drops queued input.
public sealed class MouseInput(
    Func<MouseButtons, int, int, int, Task> send,
    Func<MouseButtons, int, int, Task> position)
{
    private readonly SemaphoreSlim _gate = new(1, 1);
    private CancellationTokenSource _generation = new();
    private MouseButtons _buttons;
    public bool GestureRunning { get; private set; }

    public Task MoveAsync(int x, int y) =>
        RunAsync(async token =>
        {
            token.ThrowIfCancellationRequested();
            await send(_buttons, x, y, 0);
        });

    public Task ButtonAsync(bool down) =>
        RunAsync(async token =>
        {
            token.ThrowIfCancellationRequested();
            _buttons = down ? MouseButtons.Left : MouseButtons.None;
            await send(_buttons, 0, 0, 0);
        });

    public Task ClickAsync() =>
        RunAsync(async token =>
        {
            try
            {
                token.ThrowIfCancellationRequested();
                await send(MouseButtons.Left, 0, 0, 0);
                await Task.Delay(60, token);
            }
            finally
            {
                _buttons = MouseButtons.None;
                await send(MouseButtons.None, 0, 0, 0);
            }
        });

    public Task PositionAsync(int x, int y, int intervalMs) =>
        RunAsync(async token =>
        {
            token.ThrowIfCancellationRequested();
            await position(_buttons, x, y);
            await Task.Delay(intervalMs, token);
        });

    public Task ScreenButtonAsync(int x, int y, bool down, int intervalMs) =>
        RunAsync(async token =>
        {
            token.ThrowIfCancellationRequested();
            await position(_buttons, x, y);
            // iOS must apply the absolute position before the relative button report.
            await Task.Delay(down ? intervalMs * 2 : intervalMs, token);
            token.ThrowIfCancellationRequested();
            _buttons = down ? MouseButtons.Left : MouseButtons.None;
            await send(_buttons, 0, 0, 0);
            await Task.Delay(intervalMs, token);
        });

    public Task ScreenSwipeAsync(int x, int direction, int intervalMs)
    {
        if (direction is not (-1 or 1)) throw new ArgumentOutOfRangeException(nameof(direction));
        return RunAsync(async token =>
        {
            GestureRunning = true;
            var start = direction < 0 ? 24575 : 8192;
            var end = direction < 0 ? 8192 : 24575;
            try
            {
                await position(MouseButtons.None, x, start);
                await Task.Delay(intervalMs * 2, token);
                _buttons = MouseButtons.Left;
                await send(_buttons, 0, 0, 0);
                await Task.Delay(intervalMs, token);
                for (var i = 1; i <= 12; i++)
                {
                    token.ThrowIfCancellationRequested();
                    await position(_buttons, x, start + (end - start) * i / 12);
                    await Task.Delay(intervalMs, token);
                }
            }
            finally
            {
                _buttons = MouseButtons.None;
                try { await send(_buttons, 0, 0, 0); }
                finally { GestureRunning = false; }
            }
        });
    }

    // Reposition after lifting so repeated swipes don't leave the pointer at an edge.
    public Task SwipeAsync(int direction, int distance, int intervalMs)
    {
        if (direction is not (-1 or 1)) throw new ArgumentOutOfRangeException(nameof(direction));
        if (distance is < 120 or > 600) throw new ArgumentOutOfRangeException(nameof(distance));
        return RunAsync(async token =>
        {
            GestureRunning = true;
            var steps = SwipeSteps(direction, distance);
            try
            {
                await send(MouseButtons.Left, 0, 0, 0);
                await Task.Delay(intervalMs, token);
                foreach (var step in steps)
                {
                    token.ThrowIfCancellationRequested();
                    await send(MouseButtons.Left, 0, step, 0);
                    await Task.Delay(intervalMs, token);
                }
            }
            finally
            {
                _buttons = MouseButtons.None;
                try { await send(MouseButtons.None, 0, 0, 0); }
                finally { GestureRunning = false; }
            }
            await Task.Delay(80, token);
            foreach (var step in steps.Reverse())
            {
                token.ThrowIfCancellationRequested();
                await send(MouseButtons.None, 0, -step, 0);
                await Task.Delay(intervalMs, token);
            }
        });
    }

    public static int[] SwipeSteps(int direction, int distance)
    {
        const int count = 12;
        return Enumerable.Range(0, count)
            .Select(i => direction * ((i + 1) * distance / count - i * distance / count)).ToArray();
    }

    public async Task ResetAsync()
    {
        var old = _generation;
        _generation = new CancellationTokenSource();
        old.Cancel();
        await _gate.WaitAsync();
        try
        {
            _buttons = MouseButtons.None;
            await send(MouseButtons.None, 0, 0, 0);
        }
        finally { _gate.Release(); }
        // Queued operations may still own a token from the cancelled generation.
    }

    private async Task RunAsync(Func<CancellationToken, Task> action)
    {
        var token = _generation.Token;
        await _gate.WaitAsync();
        try
        {
            token.ThrowIfCancellationRequested();
            await action(token);
        }
        catch (OperationCanceledException) when (token.IsCancellationRequested) { }
        finally { _gate.Release(); }
    }
}
