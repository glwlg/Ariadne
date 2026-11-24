using System.Globalization;

namespace Fancun.CaptureHost;

internal sealed class NetworkMiniState
{
    public bool Visible { get; set; }
    public int X { get; set; }
    public int Y { get; set; }
    public int Width { get; set; }
    public int Height { get; set; }
    public double UploadBytesPerSecond { get; set; }
    public double DownloadBytesPerSecond { get; set; }
    public string AdapterName { get; set; } = "";
    public string Error { get; set; } = "";
}

internal sealed class NetworkMiniReply
{
    public bool Ok { get; set; }
    public string Message { get; set; } = "";
    public string Action { get; set; } = "";
}

internal static class NetworkMiniRate
{
    private static readonly string[] Units = ["B", "KB", "MB", "GB", "TB"];

    public static string Format(double bytesPerSecond)
    {
        var value = double.IsFinite(bytesPerSecond) ? Math.Max(0, bytesPerSecond) : 0;
        var index = 0;
        while (value >= 1024 && index < Units.Length - 1)
        {
            value /= 1024;
            index++;
        }
        var format = index == 0 || value >= 100 ? "0" : "0.0";
        return value.ToString(format, CultureInfo.InvariantCulture) + " " + Units[index] + "/s";
    }
}
