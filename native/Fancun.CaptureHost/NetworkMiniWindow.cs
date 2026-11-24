using System.Runtime.InteropServices;
using System.Text;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Interop;
using System.Windows.Media;

namespace Fancun.CaptureHost;

internal sealed class NetworkMiniWindow : Window
{
    private readonly TextBlock _upload = RateText();
    private readonly TextBlock _download = RateText();
    private readonly NetworkMiniClickTracker _clicks = new();
    private string _pendingAction = "";
    private DateTime _layerRefreshedAt;
    private NetworkMiniState? _previous;

    public NetworkMiniWindow()
    {
        Title = "Fancun - 网速小窗";
        WindowStyle = WindowStyle.None;
        ResizeMode = ResizeMode.NoResize;
        ShowInTaskbar = false;
        ShowActivated = false;
        Focusable = false;
        Topmost = true;
        AllowsTransparency = true;
        Background = Brushes.Transparent;
        UseLayoutRounding = true;
        SnapsToDevicePixels = true;

        var rows = new Grid { Margin = new Thickness(6, 2, 6, 2) };
        rows.RowDefinitions.Add(new RowDefinition());
        rows.RowDefinitions.Add(new RowDefinition());
        AddRateRow(rows, 0, "↑", _upload);
        AddRateRow(rows, 1, "↓", _download);
        Content = new Border
        {
            Background = new SolidColorBrush(Color.FromArgb(235, 72, 72, 76)),
            CornerRadius = new CornerRadius(4),
            Child = rows
        };
        var menu = new ContextMenu();
        var open = new MenuItem { Header = "查看进程网络" };
        open.Click += (_, _) => _pendingAction = "open-center";
        menu.Items.Add(open);
        menu.Items.Add(new Separator());
        var close = new MenuItem { Header = "关闭网速小窗" };
        close.Click += (_, _) => RequestClose();
        menu.Items.Add(close);
        ContextMenu = menu;
        SourceInitialized += (_, _) =>
        {
            var hwnd = new WindowInteropHelper(this).Handle;
            var style = GetWindowLongPtr(hwnd, -20).ToInt64();
            SetWindowLongPtr(hwnd, -20, new IntPtr((style | 0x08000080L) & ~0x00040000L));
            HwndSource.FromHwnd(hwnd)?.AddHook(WindowMessage);
        };
    }

    public NetworkMiniReply Apply(NetworkMiniState state)
    {
        var action = _pendingAction;
        _pendingAction = "";
        // A queued close wins over the telemetry frame that collects it.
        if (!state.Visible || action == "close")
        {
            if (ContextMenu != null) ContextMenu.IsOpen = false;
            Hide();
            _previous = null;
            return new NetworkMiniReply { Ok = true, Action = action };
        }
        if (state.Width <= 0 || state.Height <= 0)
        {
            return new NetworkMiniReply { Message = "网络小窗尺寸无效" };
        }

        _upload.Text = state.Error.Length == 0 ? NetworkMiniRate.Format(state.UploadBytesPerSecond) : "—";
        _download.Text = state.Error.Length == 0 ? NetworkMiniRate.Format(state.DownloadBytesPerSecond) : "—";
        ToolTip = state.Error.Length == 0
            ? $"{state.AdapterName} 上传 {_upload.Text} / 下载 {_download.Text}"
            : state.Error;
        var scale = NativeMethods.DpiScaleForPoint(state.X, state.Y);
        Width = state.Width / scale.ScaleX;
        Height = state.Height / scale.ScaleY;
        if (!IsVisible)
        {
            Left = state.X / scale.ScaleX;
            Top = state.Y / scale.ScaleY;
            Show();
        }
        var hwnd = new WindowInteropHelper(this).Handle;
        var moved = _previous == null || _previous.X != state.X || _previous.Y != state.Y
            || _previous.Width != state.Width || _previous.Height != state.Height;
        var foreground = GetForegroundWindow();
        var className = new StringBuilder(128);
        GetClassName(foreground, className, className.Capacity);
        var taskbarForeground = className.ToString() is "Shell_TrayWnd" or "Shell_SecondaryTrayWnd";
        if (moved || taskbarForeground || DateTime.UtcNow - _layerRefreshedAt >= TimeSpan.FromSeconds(30))
        {
            // Physical coordinates, topmost, and SWP_NOACTIVATE: updating this
            // utility must never steal focus from the launcher or capture.
            NativeMethods.SetWindowPos(hwnd, new IntPtr(-1), state.X, state.Y, state.Width, state.Height, 0x0010);
            _layerRefreshedAt = DateTime.UtcNow;
        }
        _previous = state;
        return new NetworkMiniReply { Ok = true, Action = action };
    }

    private void RequestClose()
    {
        _pendingAction = "close";
        Hide();
    }

    private static TextBlock RateText() => new()
    {
        Text = "—",
        Foreground = Brushes.White,
        FontFamily = new FontFamily("Consolas"),
        FontWeight = FontWeights.SemiBold,
        FontSize = 11,
        VerticalAlignment = VerticalAlignment.Center
    };

    private static void AddRateRow(Grid rows, int row, string arrow, TextBlock value)
    {
        var panel = new StackPanel { Orientation = Orientation.Horizontal };
        panel.Children.Add(new TextBlock
        {
            Text = arrow,
            Foreground = new SolidColorBrush(Color.FromRgb(210, 215, 222)),
            Width = 14,
            FontSize = 11,
            VerticalAlignment = VerticalAlignment.Center
        });
        panel.Children.Add(value);
        Grid.SetRow(panel, row);
        rows.Children.Add(panel);
    }

    private IntPtr WindowMessage(IntPtr hwnd, int message, IntPtr wParam, IntPtr lParam, ref bool handled)
    {
        if (message == 0x0021) // WM_MOUSEACTIVATE
        {
            handled = true;
            return new IntPtr(3); // MA_NOACTIVATE
        }
        // WPF HWND classes do not request CS_DBLCLKS. Recognize the two
        // non-activating button messages using the user's Windows settings.
        if (message == 0x0201 && _clicks.Register( // WM_LBUTTONDOWN
            unchecked((uint)GetMessageTime()),
            (short)(lParam.ToInt64() & 0xffff),
            (short)((lParam.ToInt64() >> 16) & 0xffff),
            GetDoubleClickTime(), GetSystemMetrics(36), GetSystemMetrics(37)))
        {
            _pendingAction = "open-center";
            handled = true;
        }
        if (message == 0x007B && ContextMenu != null) // WM_CONTEXTMENU
        {
            ContextMenu.PlacementTarget = this;
            ContextMenu.IsOpen = true;
            handled = true;
        }
        return IntPtr.Zero;
    }

    [DllImport("user32.dll", EntryPoint = "GetWindowLongPtrW")]
    private static extern IntPtr GetWindowLongPtr(IntPtr hwnd, int index);
    [DllImport("user32.dll", EntryPoint = "SetWindowLongPtrW")]
    private static extern IntPtr SetWindowLongPtr(IntPtr hwnd, int index, IntPtr value);
    [DllImport("user32.dll")]
    private static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll", CharSet = CharSet.Unicode, EntryPoint = "GetClassNameW")]
    private static extern int GetClassName(IntPtr hwnd, StringBuilder className, int maxCount);
    [DllImport("user32.dll")]
    private static extern int GetMessageTime();
    [DllImport("user32.dll")]
    private static extern uint GetDoubleClickTime();
    [DllImport("user32.dll")]
    private static extern int GetSystemMetrics(int index);
}
