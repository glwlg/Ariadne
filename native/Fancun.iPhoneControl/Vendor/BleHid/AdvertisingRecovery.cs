using System.Threading.Channels;

namespace BleHid.Core;

// One worker owns all retry attempts; repeated Windows status events are coalesced.
internal sealed class AdvertisingRecovery : IAsyncDisposable
{
    private readonly Func<bool> _isAdvertising;
    private readonly Action _startAdvertising;
    private readonly Action<string> _log;
    private readonly Func<TimeSpan, CancellationToken, Task> _delay;
    private readonly object _gate = new();
    private readonly CancellationTokenSource _stop = new();
    private readonly Channel<bool> _signals = Channel.CreateBounded<bool>(new BoundedChannelOptions(1)
    {
        SingleReader = true,
        FullMode = BoundedChannelFullMode.DropWrite
    });
    private readonly Task _worker;
    private bool _stopping;

    public AdvertisingRecovery(Func<bool> isAdvertising, Action startAdvertising, Action<string> log,
        Func<TimeSpan, CancellationToken, Task>? delay = null)
    {
        _isAdvertising = isAdvertising;
        _startAdvertising = startAdvertising;
        _log = log;
        _delay = delay ?? Task.Delay;
        _worker = Task.Run(RunAsync);
    }

    public void NotifyStopped() => _signals.Writer.TryWrite(true);

    private async Task RunAsync()
    {
        try
        {
            while (await _signals.Reader.WaitToReadAsync(_stop.Token))
            {
                while (_signals.Reader.TryRead(out _)) { }
                var attempt = 0;
                while (!IsAdvertising())
                {
                    var seconds = Math.Min(30, 1 << Math.Min(attempt, 5));
                    await _delay(TimeSpan.FromSeconds(seconds), _stop.Token);
                    if (IsAdvertising()) break;

                    lock (_gate)
                    {
                        if (_stopping) return;
                    }
                    try
                    {
                        _log($"[adv ] recovery attempt {attempt + 1}");
                        _startAdvertising();
                    }
                    catch (Exception ex)
                    {
                        _log($"[adv ] recovery failed: {ex.Message}");
                    }
                    if (attempt < 30) attempt++;
                }
                _log("[adv ] advertising available");
            }
        }
        catch (OperationCanceledException) when (_stop.IsCancellationRequested) { }
    }

    private bool IsAdvertising()
    {
        try { return _isAdvertising(); }
        catch (Exception ex)
        {
            _log($"[adv ] could not read status: {ex.Message}");
            return false;
        }
    }

    public async ValueTask DisposeAsync()
    {
        bool first;
        lock (_gate)
        {
            first = !_stopping;
            _stopping = true;
        }
        if (first)
        {
            _signals.Writer.TryComplete();
            _stop.Cancel();
        }
        await _worker;
        if (first) _stop.Dispose();
    }
}
