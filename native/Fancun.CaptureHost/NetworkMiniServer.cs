using System.IO;
using System.IO.Pipes;
using System.Text;
using System.Text.Json;
using System.Windows.Threading;

namespace Fancun.CaptureHost;

// The capture pipe can wait for user interaction for minutes. Mini updates use
// their own listener and marshal only a bounded, current frame to the UI thread.
internal sealed class NetworkMiniServer(string pipeName, Dispatcher dispatcher) : IDisposable
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);
    private readonly CancellationTokenSource _stop = new();
    private Task? _listenTask;
    private NetworkMiniWindow? _window;

    public void Start() => _listenTask = Task.Run(ListenAsync);

    public void Dispose()
    {
        _stop.Cancel();
        try { _listenTask?.GetAwaiter().GetResult(); }
        catch (Exception) { }
        _stop.Dispose();
    }

    private async Task ListenAsync()
    {
        while (!_stop.IsCancellationRequested)
        {
            await using var pipe = new NamedPipeServerStream(
                pipeName + "-network-mini", PipeDirection.InOut, 1,
                PipeTransmissionMode.Byte, PipeOptions.Asynchronous | PipeOptions.CurrentUserOnly);
            try
            {
                await pipe.WaitForConnectionAsync(_stop.Token);
                // A half-written request must not block subsequent hide frames.
                using var timeout = CancellationTokenSource.CreateLinkedTokenSource(_stop.Token);
                timeout.CancelAfter(TimeSpan.FromSeconds(2));
                using var reader = new StreamReader(pipe, Encoding.UTF8, false, 1024, leaveOpen: true);
                await using var writer = new StreamWriter(pipe, new UTF8Encoding(false), 1024, leaveOpen: true) { AutoFlush = true };
                var line = await reader.ReadLineAsync(timeout.Token);
                var state = line == null ? null : JsonSerializer.Deserialize<NetworkMiniState>(line, JsonOptions);
                NetworkMiniReply reply;
                if (state == null)
                {
                    reply = new NetworkMiniReply { Message = "网络小窗数据无效" };
                }
                else
                {
                    var operation = dispatcher.InvokeAsync(() =>
                    {
                        // Hidden updates must not allocate a window at startup.
                        if (_window == null && !state.Visible) return new NetworkMiniReply { Ok = true };
                        _window ??= new NetworkMiniWindow();
                        return _window.Apply(state);
                    }, DispatcherPriority.Normal, timeout.Token);
                    reply = await operation.Task.WaitAsync(timeout.Token);
                }
                await writer.WriteLineAsync(JsonSerializer.Serialize(reply, JsonOptions).AsMemory(), timeout.Token);
            }
            catch (OperationCanceledException) when (_stop.IsCancellationRequested) { return; }
            catch (OperationCanceledException) { }
            catch (IOException) { }
            catch (JsonException) { }
        }
    }
}
