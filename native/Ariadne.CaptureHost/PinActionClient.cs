using System.IO;
using System.IO.Pipes;
using System.Text;
using System.Text.Json;

namespace Ariadne.CaptureHost;

internal static class PinActionClient
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    public static async Task<PinActionResponse> SendAsync(string pipeName, PinActionRequest request)
    {
        pipeName = pipeName.Trim();
        if (pipeName.Length == 0)
        {
            return new PinActionResponse { Ok = false, Message = "OCR 服务不可用" };
        }

        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(70));
        await using var pipe = new NamedPipeClientStream(".", pipeName, PipeDirection.InOut, PipeOptions.Asynchronous);
        await pipe.ConnectAsync(2000, timeout.Token);
        using var reader = new StreamReader(pipe, Encoding.UTF8, false, 1024, leaveOpen: true);
        await using var writer = new StreamWriter(pipe, new UTF8Encoding(false), 1024, leaveOpen: true) { AutoFlush = true };

        await writer.WriteLineAsync(JsonSerializer.Serialize(request, JsonOptions).AsMemory(), timeout.Token);
        var line = await reader.ReadLineAsync(timeout.Token);
        return string.IsNullOrWhiteSpace(line)
            ? new PinActionResponse { Ok = false, Message = "OCR 服务无响应" }
            : JsonSerializer.Deserialize<PinActionResponse>(line, JsonOptions) ?? new PinActionResponse { Ok = false, Message = "OCR 返回无效" };
    }
}
