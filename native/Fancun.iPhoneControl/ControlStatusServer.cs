using System.IO.Pipes;
using System.Text;
using System.Text.Json;

namespace Fancun.iPhoneControl;

internal sealed record ControlStatus(
    string Phase, int ProcessId, bool Advertising = false, int ConnectedDevices = 0,
    string TargetName = "", int Readiness = 0, string LastError = "", string Diagnostics = "")
{
    public bool Running => true;
}

internal static class ControlStatusFactory
{
    public static ControlStatus Create(int processId, bool advertising, bool incompleteAdvertisement,
        int connectedDevices, string targetName, int readiness, string error, string diagnostics)
    {
        var phase = readiness == 2 ? "direct" : readiness == 1 ? "touchpad" :
            connectedDevices > 0 ? "select-device" : advertising ? "waiting" : "error";
        var warning = !advertising ? "蓝牙广播已中断，正在自动恢复。" :
            incompleteAdvertisement ? "蓝牙广播数据不完整，可能影响设备发现。" : "";
        var message = string.Join(" ", new[] { error, warning }.Where(value => !string.IsNullOrWhiteSpace(value)));
        return new ControlStatus(phase, processId, advertising, connectedDevices, targetName,
            readiness, message, diagnostics);
    }
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
