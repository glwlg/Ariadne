using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Windows;
using System.Windows.Interop;
using BleHid.Core;
using Windows.Devices.Bluetooth.GenericAttributeProfile;

namespace Ariadne.iPhoneControl;

public partial class App : Application
{
    private Mutex? _instance;
    private bool _ownsMutex;
    private readonly object _logLock = new();
    public static string DataDirectory => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Ariadne", "airplay");
    public static string LogPath => Path.Combine(DataDirectory, "control.log");
    public const string WindowTitle = "Ariadne · iPhone 控制";
    public const int ActivateMessage = 0x8001;

    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        Directory.CreateDirectory(DataDirectory);
        _instance = new Mutex(true, @"Local\Ariadne.iPhoneControl", out _ownsMutex);
        if (!_ownsMutex)
        {
            var existing = FindWindow(null, WindowTitle);
            if (existing != IntPtr.Zero && !e.Args.Contains("--background"))
                PostMessage(existing, ActivateMessage, IntPtr.Zero, IntPtr.Zero);
            Shutdown(e.Args.Contains("--probe") ? 2 : 0);
            return;
        }
        if (e.Args.Length == 2 && e.Args[0] == "--probe")
        {
            var ok = false;
            await using var peripheral = new BleHidPeripheral();
            peripheral.SelectLocal();
            peripheral.Log += Log;
            try
            {
                await peripheral.StartAsync();
                ok = peripheral.Diagnostics.All(d => d.Ok) &&
                     peripheral.AdvertisementStatus == GattServiceProviderAdvertisementStatus.Started;
                File.WriteAllText(e.Args[1], JsonSerializer.Serialize(new
                {
                    ok, diagnostics = peripheral.Diagnostics,
                    advertising = peripheral.AdvertisementStatus.ToString()
                }, new JsonSerializerOptions { WriteIndented = true }));
            }
            catch (Exception ex)
            {
                File.WriteAllText(e.Args[1], JsonSerializer.Serialize(new { ok = false, error = ex.ToString() }));
            }
            await peripheral.DisposeAsync();
            Shutdown(ok ? 0 : 1);
            return;
        }
        int? ownerPid = null;
        var args = e.Args.Where(arg => arg != "--background").ToArray();
        if (args.Length != 0)
        {
            if (args.Length != 2 || args[0] != "--owner-pid" ||
                !int.TryParse(args[1], out var pid) || pid <= 0)
            {
                Shutdown(2);
                return;
            }
            ownerPid = pid;
        }
        var window = new ControlWindow(ownerPid);
        MainWindow = window;
        // Create the input IPC endpoint without flashing a taskbar window.
        new WindowInteropHelper(window).EnsureHandle();
        if (!e.Args.Contains("--background")) window.Show();
        await window.StartAsync();
    }

    public void Log(string line)
    {
        lock (_logLock)
        {
            try
            {
                if (File.Exists(LogPath) && new FileInfo(LogPath).Length > 2 * 1024 * 1024)
                    File.Move(LogPath, LogPath + ".previous", true);
                File.AppendAllText(LogPath, $"{DateTimeOffset.Now:O} {line}{Environment.NewLine}");
            }
            catch (IOException) { }
            catch (UnauthorizedAccessException) { }
        }
    }

    protected override void OnExit(ExitEventArgs e)
    {
        if (_ownsMutex) _instance?.ReleaseMutex();
        _instance?.Dispose();
        base.OnExit(e);
    }

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern IntPtr FindWindow(string? className, string windowName);
    [DllImport("user32.dll")]
    private static extern bool PostMessage(IntPtr hwnd, int message, IntPtr wParam, IntPtr lParam);
}
