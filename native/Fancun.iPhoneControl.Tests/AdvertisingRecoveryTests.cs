using System.Threading.Channels;
using BleHid.Core;

internal static class AdvertisingRecoveryTests
{
    public static async Task RunAsync()
    {
        await SpontaneousRecoveryAsync();
        await RetryAndShutdownAsync();
        await InFlightShutdownAsync();
        Console.WriteLine("PASS advertising recovery, backoff, duplicate events and shutdown");
    }

    private static async Task InFlightShutdownAsync()
    {
        var clock = new TestClock();
        var entered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var disposeReturned = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        using var unblock = new ManualResetEventSlim();
        var recovery = new AdvertisingRecovery(() => false, () =>
        {
            entered.TrySetResult();
            if (!unblock.Wait(TimeSpan.FromSeconds(5)))
                throw new TimeoutException("Test did not release the simulated Bluetooth call.");
        }, _ => { }, clock.DelayAsync);
        recovery.NotifyStopped();
        (await clock.NextAsync()).Release();
        await entered.Task.WaitAsync(TimeSpan.FromSeconds(5));
        var shutdown = Task.Run(async () =>
        {
            var stopping = recovery.DisposeAsync();
            disposeReturned.TrySetResult();
            await stopping;
        });
        try
        {
            await disposeReturned.Task.WaitAsync(TimeSpan.FromSeconds(2));
            Assert(!shutdown.IsCompleted, "Shutdown must wait for an in-flight Bluetooth call.");
        }
        finally { unblock.Set(); }
        await shutdown.WaitAsync(TimeSpan.FromSeconds(5));
    }

    private static async Task SpontaneousRecoveryAsync()
    {
        var clock = new TestClock();
        var logs = Channel.CreateUnbounded<string>();
        var advertising = 0;
        var attempts = 0;
        await using var recovery = new AdvertisingRecovery(
            () => Volatile.Read(ref advertising) != 0,
            () => Interlocked.Increment(ref attempts),
            message => logs.Writer.TryWrite(message), clock.DelayAsync);
        recovery.NotifyStopped();
        var delay = await clock.NextAsync();
        Assert(delay.Duration == TimeSpan.FromSeconds(1), "The first retry must wait one second.");
        Volatile.Write(ref advertising, 1);
        delay.Release();
        await WaitForLogAsync(logs.Reader, "advertising available");
        Assert(Volatile.Read(ref attempts) == 0, "A spontaneous recovery must not be restarted.");
    }

    private static async Task RetryAndShutdownAsync()
    {
        var clock = new TestClock();
        var logs = Channel.CreateUnbounded<string>();
        var calls = Channel.CreateUnbounded<int>();
        var advertising = 0;
        var attempts = 0;
        var recovery = new AdvertisingRecovery(
            () => Volatile.Read(ref advertising) != 0,
            () =>
            {
                var count = Interlocked.Increment(ref attempts);
                calls.Writer.TryWrite(count);
                if (count == 1) throw new InvalidOperationException("simulated Bluetooth error");
            },
            message => logs.Writer.TryWrite(message), clock.DelayAsync);

        recovery.NotifyStopped();
        foreach (var seconds in new[] { 1, 2, 4, 8, 16, 30, 30 })
        {
            var delay = await clock.NextAsync();
            Assert(delay.Duration == TimeSpan.FromSeconds(seconds), "Retry backoff must be capped at 30 seconds.");
            for (var i = 0; i < 10; i++) recovery.NotifyStopped();
            delay.Release();
            var count = await calls.Reader.ReadAsync().AsTask().WaitAsync(TimeSpan.FromSeconds(5));
            Assert(count == attempts, "Each delay must trigger only one attempt.");
        }

        var pending = await clock.NextAsync();
        Volatile.Write(ref advertising, 1);
        pending.Release();
        await WaitForLogAsync(logs.Reader, "advertising available");
        Assert(Volatile.Read(ref attempts) == 7,
            "Retries must stop after Windows reports advertising is available.");

        Volatile.Write(ref advertising, 0);
        recovery.NotifyStopped();
        var nextOutage = await clock.NextAsync();
        Assert(nextOutage.Duration == TimeSpan.FromSeconds(1), "A new outage must reset the backoff.");
        await Task.WhenAll(recovery.DisposeAsync().AsTask(), recovery.DisposeAsync().AsTask())
            .WaitAsync(TimeSpan.FromSeconds(5));
        nextOutage.Release();
        recovery.NotifyStopped();
        Assert(Volatile.Read(ref attempts) == 7, "Shutdown must cancel pending attempts.");
    }

    private static async Task WaitForLogAsync(ChannelReader<string> logs, string value)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(5));
        while (!(await logs.ReadAsync(timeout.Token)).Contains(value, StringComparison.Ordinal)) { }
    }

    private static void Assert(bool condition, string message)
    {
        if (!condition) throw new Exception(message);
    }

    private sealed class TestClock
    {
        private readonly Channel<PendingDelay> _delays = Channel.CreateUnbounded<PendingDelay>();

        public Task DelayAsync(TimeSpan duration, CancellationToken cancellationToken)
        {
            var delay = new PendingDelay(duration);
            _delays.Writer.TryWrite(delay);
            return delay.WaitAsync(cancellationToken);
        }

        public async Task<PendingDelay> NextAsync() =>
            await _delays.Reader.ReadAsync().AsTask().WaitAsync(TimeSpan.FromSeconds(5));
    }

    private sealed class PendingDelay(TimeSpan duration)
    {
        private readonly TaskCompletionSource _release = new(TaskCreationOptions.RunContinuationsAsynchronously);
        public TimeSpan Duration { get; } = duration;
        public Task WaitAsync(CancellationToken cancellationToken) => _release.Task.WaitAsync(cancellationToken);
        public void Release() => _release.TrySetResult();
    }
}
