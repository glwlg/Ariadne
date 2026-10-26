using System.IO.Pipes;
using System.Text;
using System.Text.Json;

namespace Ariadne.iPhoneControl;

internal sealed record ControlStatus(
    string Phase, int ProcessId, bool Advertising = false, int ConnectedDevices = 0,
    string TargetName = "", int Readiness = 0, string LastError = "", string Diagnostics = "")
{
    public bool Running => true;
}

// Serve a cached snapshot: pipe requests never enter the UI dispatcher or call WinRT.
internal sealed class ControlStatusServer : IAsyncDisposable
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);
    private readonly CancellationTokenSource _stop = new();
    private readonly string _pipeName;
    private readonly Action<string> _log;
    private readonly Task _listener;
    private string _snapshot;

    public ControlStatusServer(string pipeName, ControlStatus initial, Action<string> log)
    {
        _pipeName = pipeName;
        _log = log;
        _snapshot = JsonSerializer.Serialize(initial, JsonOptions);
        _listener = Task.Run(ListenAsync);
    }

    public void Update(ControlStatus status) =>
        Volatile.Write(ref _snapshot, JsonSerializer.Serialize(status, JsonOptions));

    private async Task ListenAsync()
    {
        try
        {
            while (!_stop.IsCancellationRequested)
            {
                await using var pipe = new NamedPipeServerStream(_pipeName, PipeDirection.InOut, 1,
                    PipeTransmissionMode.Byte, PipeOptions.Asynchronous | PipeOptions.CurrentUserOnly);
                try
                {
                    await pipe.WaitForConnectionAsync(_stop.Token);
                    using var timeout = CancellationTokenSource.CreateLinkedTokenSource(_stop.Token);
                    timeout.CancelAfter(TimeSpan.FromSeconds(1));
                    // Write directly so a disconnected reader cannot throw again during writer disposal.
                    var bytes = Encoding.UTF8.GetBytes(Volatile.Read(ref _snapshot) + "\n");
                    await pipe.WriteAsync(bytes, timeout.Token);
                }
                catch (OperationCanceledException) when (_stop.IsCancellationRequested) { return; }
                catch (Exception ex) when (ex is IOException or OperationCanceledException)
                {
                    _log($"[status] {ex.Message}");
                }
            }
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            _log($"[status] server stopped: {ex.Message}");
        }
    }

    public async ValueTask DisposeAsync()
    {
        _stop.Cancel();
        await _listener;
        _stop.Dispose();
    }
}
