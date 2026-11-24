using System.Globalization;
using System.Text.Json;

namespace Fancun.CaptureHost;

internal static class NetworkMiniTests
{
    public static void Run()
    {
        var culture = CultureInfo.CurrentCulture;
        try
        {
            CultureInfo.CurrentCulture = CultureInfo.GetCultureInfo("de-DE");
            foreach (var (input, expected) in new (double, string)[]
            {
                (0, "0 B/s"), (-1, "0 B/s"), (double.NaN, "0 B/s"),
                (double.PositiveInfinity, "0 B/s"), (1023, "1023 B/s"),
                (1024, "1.0 KB/s"), (1280, "1.3 KB/s"),
                (175 * 1024, "175 KB/s"), (1.2 * 1024 * 1024, "1.2 MB/s")
            })
            {
                var actual = NetworkMiniRate.Format(input);
                if (actual != expected)
                    throw new InvalidOperationException($"format {input}: expected {expected}, got {actual}");
            }
            var options = new JsonSerializerOptions(JsonSerializerDefaults.Web);
            var state = JsonSerializer.Deserialize<NetworkMiniState>(
                """{"visible":true,"x":-1911,"y":875,"width":231,"height":48,"uploadBytesPerSecond":1280,"downloadBytesPerSecond":1258291.2,"adapterName":"以太网","error":""}""", options)!;
            if (!state.Visible || state.X != -1911 || state.Width != 231
                || NetworkMiniRate.Format(state.UploadBytesPerSecond) != "1.3 KB/s"
                || state.AdapterName != "以太网")
                throw new InvalidOperationException("Go network mini state did not deserialize correctly");
            var reply = JsonSerializer.Serialize(new NetworkMiniReply { Ok = true, Action = "close" }, options);
            if (!reply.Contains("\"ok\":true") || !reply.Contains("\"action\":\"close\""))
                throw new InvalidOperationException("Network mini response does not match the Go protocol");
            var clicks = new NetworkMiniClickTracker();
            if (clicks.Register(100, 8, 8, 500, 4, 4)
                || !clicks.Register(200, 8, 8, 500, 4, 4)
                || clicks.Register(250, 8, 8, 500, 4, 4)
                || clicks.Register(900, 8, 8, 500, 4, 4)
                || clicks.Register(950, 20, 8, 500, 4, 4))
                throw new InvalidOperationException("Native double click ignored timing, movement, or click pairing");
            var wrappingClicks = new NetworkMiniClickTracker();
            if (wrappingClicks.Register(uint.MaxValue - 50, 8, 8, 500, 4, 4)
                || !wrappingClicks.Register(50, 8, 8, 500, 4, 4))
                throw new InvalidOperationException("Native double click failed across the Windows timestamp rollover");
        }
        finally { CultureInfo.CurrentCulture = culture; }
    }
}
