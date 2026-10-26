using System.IO.Pipes;
using System.Text.Json;
using Ariadne.iPhoneControl;

internal static class StatusTests
{
    public static async Task RunAsync()
    {
        var pipeName = "ariadne-control-test-" + Guid.NewGuid().ToString("N");
        await using var server = new ControlStatusServer(pipeName,
            new ControlStatus("starting", Environment.ProcessId), Console.WriteLine);
        async Task<JsonElement> ReadAsync()
        {
            await using var client = new NamedPipeClientStream(".", pipeName, PipeDirection.InOut,
                PipeOptions.Asynchronous);
            using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(3));
            await client.ConnectAsync(timeout.Token);
            using var reader = new StreamReader(client);
            var line = await reader.ReadLineAsync(timeout.Token);
            return JsonSerializer.Deserialize<JsonElement>(line!);
        }
        var starting = await ReadAsync();
        if (starting.GetProperty("phase").GetString() != "starting" ||
            !starting.GetProperty("running").GetBoolean())
            throw new Exception("Status pipe must report startup without touching Bluetooth.");
        server.Update(new ControlStatus("direct", Environment.ProcessId, true, 1, "Test iPhone", 2));
        var connected = await ReadAsync();
        if (connected.GetProperty("readiness").GetInt32() != 2 ||
            connected.GetProperty("targetName").GetString() != "Test iPhone")
            throw new Exception("Status pipe must return the latest selected device and input readiness.");
        server.Update(new ControlStatus("waiting", Environment.ProcessId, true));
        var disconnected = await ReadAsync();
        if (disconnected.GetProperty("connectedDevices").GetInt32() != 0 ||
            disconnected.GetProperty("readiness").GetInt32() != 0 ||
            disconnected.GetProperty("targetName").GetString() != "")
            throw new Exception("Disconnect must clear cached input readiness and target.");
        server.Update(new ControlStatus("error", Environment.ProcessId, LastError: "Bluetooth unavailable"));
        var failed = await ReadAsync();
        if (failed.GetProperty("lastError").GetString() != "Bluetooth unavailable")
            throw new Exception("Bluetooth failures must be visible to the management window.");
        Console.WriteLine("PASS isolated control status pipe, startup, readiness, disconnect and errors");
    }
}
