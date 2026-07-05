using System.IO;
using System.IO.Pipes;
using System.Text;
using System.Text.Json;
using System.Windows;
using System.Windows.Threading;
using WpfApplication = System.Windows.Application;

namespace Ariadne.CaptureHost;

internal sealed class HostServer : IDisposable
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    private readonly string _pipeName;
    private readonly Dispatcher _dispatcher;
    private readonly CancellationTokenSource _stop = new();
    private Task? _listenTask;

    public HostServer(string pipeName, Dispatcher dispatcher)
    {
        _pipeName = pipeName;
        _dispatcher = dispatcher;
    }

    public void Start()
    {
        _listenTask = Task.Run(ListenLoopAsync);
    }

    public void Dispose()
    {
        _stop.Cancel();
        try
        {
            _listenTask?.Wait(TimeSpan.FromSeconds(2));
        }
        catch
        {
            // Exiting the host should not surface transient pipe shutdown errors.
        }
        _stop.Dispose();
    }

    private async Task ListenLoopAsync()
    {
        while (!_stop.IsCancellationRequested)
        {
            await using var pipe = new NamedPipeServerStream(
                _pipeName,
                PipeDirection.InOut,
                1,
                PipeTransmissionMode.Byte,
                PipeOptions.Asynchronous);

            try
            {
                await pipe.WaitForConnectionAsync(_stop.Token);
                using var reader = new StreamReader(pipe, Encoding.UTF8, false, 1024, leaveOpen: true);
                using var writer = new StreamWriter(pipe, new UTF8Encoding(false), 1024, leaveOpen: true) { AutoFlush = true };
                var line = await reader.ReadLineAsync(_stop.Token);
                var request = string.IsNullOrWhiteSpace(line)
                    ? new CaptureRequest()
                    : JsonSerializer.Deserialize<CaptureRequest>(line, JsonOptions) ?? new CaptureRequest();
                var response = await HandleRequestAsync(request);
                await writer.WriteLineAsync(JsonSerializer.Serialize(response, JsonOptions).AsMemory(), _stop.Token);
            }
            catch (OperationCanceledException)
            {
                return;
            }
            catch (Exception ex)
            {
                if (pipe.IsConnected)
                {
                    var response = new CaptureResponse { Ok = false, Message = ex.Message };
                    using var writer = new StreamWriter(pipe, new UTF8Encoding(false), 1024, leaveOpen: true) { AutoFlush = true };
                    await writer.WriteLineAsync(JsonSerializer.Serialize(response, JsonOptions));
                }
            }
        }
    }

    private async Task<CaptureResponse> HandleRequestAsync(CaptureRequest request)
    {
        if (string.Equals(request.Command, "ping", StringComparison.OrdinalIgnoreCase))
        {
            return new CaptureResponse { Ok = true, Message = "ready" };
        }
        if (string.Equals(request.Command, "shutdown", StringComparison.OrdinalIgnoreCase))
        {
            _ = _dispatcher.BeginInvoke(() => WpfApplication.Current.Shutdown());
            return new CaptureResponse { Ok = true, Message = "closing" };
        }
        if (!string.Equals(request.Command, "capture", StringComparison.OrdinalIgnoreCase))
        {
            return new CaptureResponse { Ok = false, Message = "unsupported command" };
        }

        var operation = _dispatcher.InvokeAsync(() => CaptureCoordinator.CaptureAsync(request));
        return await (await operation.Task);
    }
}
