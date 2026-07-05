using System.Threading;
using System.Windows;
using WpfApplication = System.Windows.Application;

namespace Ariadne.CaptureHost;

internal static class Program
{
    [STAThread]
    public static void Main(string[] args)
    {
        NativeMethods.EnablePerMonitorDpiAwareness();

        var pipeName = PipeNameFromArgs(args);
        if (string.IsNullOrWhiteSpace(pipeName))
        {
            return;
        }

        using var mutex = new Mutex(true, @"Local\Ariadne.CaptureHost." + pipeName, out _);
        var app = new WpfApplication { ShutdownMode = ShutdownMode.OnExplicitShutdown };
        using var server = new HostServer(pipeName, app.Dispatcher);

        app.Startup += (_, _) => server.Start();
        app.Run();
    }

    private static string PipeNameFromArgs(string[] args)
    {
        for (var i = 0; i < args.Length; i++)
        {
            if (string.Equals(args[i], "--pipe", StringComparison.OrdinalIgnoreCase) && i + 1 < args.Length)
            {
                return args[i + 1].Trim();
            }
        }
        return "";
    }
}
