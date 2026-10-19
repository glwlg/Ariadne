using System.ComponentModel;
using System.Diagnostics;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Interop;
using System.Windows.Threading;
using BleHid.Core;
using Windows.Devices.Bluetooth.GenericAttributeProfile;

namespace Ariadne.iPhoneControl;

public partial class ControlWindow : Window
{
    private readonly BleHidPeripheral _peripheral = new();
    private readonly MouseInput _input;
    private readonly ScreenControlBridge _screenBridge;
    private readonly DispatcherTimer _timer = new() { Interval = TimeSpan.FromMilliseconds(25) };
    private readonly Process? _owner;
    private Point? _lastPoint;
    private int _dx, _dy, _ticks;
    private bool _movePending, _ready, _closing, _closed, _refreshingDevices, _swiping, _ownerMissing;
    private Task? _startupTask;
    private long _inputEpoch;
    private DateTime _nextMove;
    private string? _hostId;
    private bool _screenPressed, _screenDragging, _changingTarget;

    public ControlWindow(int? ownerPid)
    {
        InitializeComponent();
        SetupText.Text = "在 iPhone 开启「设置 → 辅助功能 → 触控 → 辅助触控」，" +
                         $"再进入「设备 → 蓝牙设备」，连接「{Environment.MachineName}」。\n" +
                         "普通蓝牙配对成功后，仍需连接鼠标控制服务；连接后手机才会出现在下方列表。";
        _peripheral.SelectLocal();
        _peripheral.Log += ((App)Application.Current).Log;
        _input = new MouseInput(SendAsync, PositionAsync);
        _screenBridge = new ScreenControlBridge(IsReceiver, OnScreenInput);
        if (ownerPid is { } pid)
        {
            try { _owner = Process.GetProcessById(pid); }
            catch (ArgumentException) { _ownerMissing = true; }
        }
        _timer.Tick += OnTick;
    }

    private async void OnLoaded(object sender, RoutedEventArgs e)
    {
        if (_ownerMissing) { Close(); return; }
        _timer.Start();
        try
        {
            _startupTask = _peripheral.StartAsync();
            await _startupTask;
            var failure = _peripheral.Diagnostics.FirstOrDefault(d => !d.Ok);
            if (failure is not null) throw new InvalidOperationException($"{failure.Step}: {failure.Detail}");
            if (_peripheral.AdvertisementStatus != GattServiceProviderAdvertisementStatus.Started)
                throw new InvalidOperationException("蓝牙广播未启动");
            if (_closing) return;
            _ready = true;
            await RefreshDevicesAsync();
        }
        catch (Exception ex)
        {
            StatusText.Text = "蓝牙控制启动失败";
            ShowError(ex);
            await _peripheral.DisposeAsync();
        }
    }

    private async Task SendAsync(MouseButtons buttons, int dx, int dy, int wheel)
    {
        if (_hostId is null || _peripheral.IsLocalTarget ||
            _peripheral.SelectedHostId != _hostId || !_peripheral.HasMouseSubscription(_hostId))
        {
            if (buttons == MouseButtons.None && dx == 0 && dy == 0 && wheel == 0) return;
            throw new InvalidOperationException("手机控制连接已断开，请重新连接蓝牙。");
        }
        await _peripheral.SendMouseAsync(buttons, dx, dy, wheel);
    }

    private int ScreenReadiness() => !_closing && !_changingTarget && _ready && _hostId is not null &&
        !_peripheral.IsLocalTarget && _peripheral.SelectedHostId == _hostId &&
        _peripheral.HasMouseSubscription(_hostId)
            ? _peripheral.HasAbsoluteSubscription(_hostId) ? 2 : 1 : 0;

    private static bool IsReceiver(int pid)
    {
        try
        {
            using var process = Process.GetProcessById(pid);
            var expected = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "uxplay", "uxplay.exe"));
            return string.Equals(process.MainModule?.FileName, expected, StringComparison.OrdinalIgnoreCase);
        }
        catch (Exception ex) when (ex is ArgumentException or Win32Exception or InvalidOperationException)
        {
            return false;
        }
    }

    private async Task PositionAsync(MouseButtons buttons, int x, int y)
    {
        if (ScreenReadiness() != 2)
            throw new InvalidOperationException("直接操作未连接，请在 iPhone 重新配对电脑。");
        await _peripheral.SendAbsoluteMouseAsync(buttons, x, y);
    }

    private async Task OnScreenInput(ScreenPointerPacket packet)
    {
        if (packet.Kind != ScreenPointerKind.Move)
            ((App)Application.Current).Log($"[screen] {packet.Kind}");
        if (packet.Kind == ScreenPointerKind.Cancel)
        {
            await SafeAsync(() => ResetAsync(clearSource: false));
            return;
        }
        if (_swiping) return;
        var interval = _peripheral.MouseReportIntervalMs(25);
        if (packet.Kind == ScreenPointerKind.Move)
        {
            if (_screenPressed && !_screenDragging)
            {
                _screenDragging = true;
                ((App)Application.Current).Log("[screen] Drag");
            }
            await SafeAsync(() => _input.PositionAsync(packet.X, packet.Y, interval));
            return;
        }
        if (packet.Kind == ScreenPointerKind.Wheel)
        {
            if (_screenPressed || _movePending || _input.GestureRunning) return;
            _swiping = true;
            try { await SafeAsync(() => _input.ScreenSwipeAsync(packet.X, packet.Delta < 0 ? -1 : 1, interval)); }
            finally { _swiping = false; }
            return;
        }
        var down = packet.Kind == ScreenPointerKind.Down;
        _screenPressed = down;
        _screenDragging = false;
        await SafeAsync(() => _input.ScreenButtonAsync(packet.X, packet.Y, down, interval));
    }

    private async void OnTick(object? sender, EventArgs e)
    {
        if (!_screenBridge.SourceAlive)
        {
            await SafeAsync(ResetAsync);
            return;
        }
        await _screenBridge.DispatchPendingAsync();
        if (++_ticks % 40 == 0)
        {
            if (_owner is not null && _owner.HasExited) { Close(); return; }
            if (_ready) await RefreshDevicesAsync();
        }
        if (_hostId is null || _movePending || _swiping || _input.GestureRunning) return;
        if (DateTime.UtcNow < _nextMove) return;
        var x = _dx;
        var y = _dy;
        _dx = _dy = 0;
        if (x == 0 && y == 0) return;
        _nextMove = DateTime.UtcNow.AddMilliseconds(_peripheral.MouseReportIntervalMs(25));
        _movePending = true;
        try { await SafeAsync(() => _input.MoveAsync(x, y)); }
        finally { _movePending = false; }
    }

    private async Task RefreshDevicesAsync()
    {
        if (_refreshingDevices || _closing) return;
        _refreshingDevices = true;
        try
        {
            await _peripheral.RefreshHostNamesAsync();
            var hosts = _peripheral.Hosts().Where(h => _peripheral.HasMouseSubscription(h.DeviceId)).ToList();
            var selected = hosts.FirstOrDefault(h => h.DeviceId == _hostId);
            if (_hostId is not null && (selected is null || _peripheral.IsLocalTarget ||
                _peripheral.SelectedHostId != _hostId))
            {
                await ResetAsync();
                _hostId = null;
                selected = null;
                _peripheral.SelectLocal();
            }
            // Preserve explicit selection; never choose a different phone after disconnecting.
            var previous = (Devices.ItemsSource as List<HostTarget>) ?? [];
            if (!hosts.SequenceEqual(previous))
            {
                Devices.ItemsSource = hosts;
            }
            Devices.SelectedItem = selected;
            InputPanel.IsEnabled = _ready && _hostId is not null;
            _screenBridge.Readiness = ScreenReadiness();
            StatusText.Text = _hostId is not null ? ScreenStatus() :
                hosts.Count > 0 ? "已连接，请选择控制设备" : $"等待 iPhone 鼠标控制连接 · {Environment.MachineName}";
        }
        catch (Exception ex) { _screenBridge.Readiness = 0; ShowError(ex); }
        finally { _refreshingDevices = false; }
    }

    private async void OnDeviceChanged(object sender, SelectionChangedEventArgs e)
    {
        if (_refreshingDevices || _closing) return;
        _changingTarget = true;
        _screenBridge.Readiness = 0;
        InputPanel.IsEnabled = false;
        await SafeAsync(async () =>
        {
            await ResetAsync();
            var target = Devices.SelectedItem as HostTarget;
            _hostId = null;
            _peripheral.SelectLocal();
            if (target is not null)
            {
                if (_peripheral.SelectMouseHost(target.DeviceId)) _hostId = target.DeviceId;
            }
            _changingTarget = false;
            _screenBridge.Readiness = ScreenReadiness();
        });
        _changingTarget = false;
        InputPanel.IsEnabled = _hostId is not null;
        StatusText.Text = _hostId is not null ? ScreenStatus() : "请选择控制设备";
        if (_hostId is not null) ErrorText.Text = "";
    }

    private string ScreenStatus() => _screenBridge.Readiness == 2
        ? "已连接，可直接在投屏画面点击、拖动和滚动"
        : "触控板已连接；直接操作画面需在 iPhone 重新配对电脑";

    private void OnPointerEnter(object sender, MouseEventArgs e) => _lastPoint = e.GetPosition(Touchpad);
    private void OnPointerLeave(object sender, MouseEventArgs e) { if (!Touchpad.IsMouseCaptured) _lastPoint = null; }
    private void OnPointerMove(object sender, MouseEventArgs e)
    {
        var point = e.GetPosition(Touchpad);
        if (_lastPoint is { } last && !_swiping)
        {
            _dx += (int)Math.Round(point.X - last.X);
            _dy += (int)Math.Round(point.Y - last.Y);
        }
        _lastPoint = point;
    }

    private async void OnPointerDown(object sender, MouseButtonEventArgs e)
    {
        if (_swiping) return;
        var epoch = _inputEpoch;
        Touchpad.Focus();
        Touchpad.CaptureMouse();
        e.Handled = true;
        await FlushMovementAsync();
        if (_inputEpoch != epoch || !IsActive) return;
        await SafeAsync(() => _input.ButtonAsync(true));
    }

    private async void OnPointerUp(object sender, MouseButtonEventArgs e)
    {
        e.Handled = true;
        await FlushMovementAsync();
        await SafeAsync(() => _input.ButtonAsync(false));
        Touchpad.ReleaseMouseCapture();
    }

    private async Task FlushMovementAsync()
    {
        var x = _dx; var y = _dy;
        _dx = _dy = 0;
        if (x != 0 || y != 0) await SafeAsync(() => _input.MoveAsync(x, y));
    }

    private async void OnHoldDown(object sender, MouseButtonEventArgs e)
    {
        if (_swiping) return;
        e.Handled = true;
        ((UIElement)sender).CaptureMouse();
        await SafeAsync(() => _input.ButtonAsync(true));
    }

    private async void OnHoldUp(object sender, MouseButtonEventArgs e)
    {
        e.Handled = true;
        await SafeAsync(() => _input.ButtonAsync(false));
        ((UIElement)sender).ReleaseMouseCapture();
    }

    private async void OnLostCapture(object sender, MouseEventArgs e) => await SafeAsync(ResetAsync);
    private async void OnDeactivated(object? sender, EventArgs e) => await SafeAsync(ResetAsync);
    private async void OnClick(object sender, RoutedEventArgs e)
    {
        if (!_swiping) await SafeAsync(_input.ClickAsync);
    }
    private async void OnSwipeUp(object sender, RoutedEventArgs e) => await SwipeAsync(-1);
    private async void OnSwipeDown(object sender, RoutedEventArgs e) => await SwipeAsync(1);
    private async void OnWheel(object sender, MouseWheelEventArgs e)
    {
        e.Handled = true;
        await SwipeAsync(e.Delta < 0 ? -1 : 1);
    }

    private async Task SwipeAsync(int direction)
    {
        if (_hostId is null || _swiping || _closing) return;
        _swiping = true;
        _dx = _dy = 0;
        try
        {
            await SafeAsync(() => _input.SwipeAsync(direction, (int)SwipeDistance.Value,
                _peripheral.MouseReportIntervalMs(25)));
        }
        finally { _swiping = false; }
    }

    private async void OnKeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == Key.Escape)
        {
            e.Handled = true;
            await SafeAsync(ResetAsync);
        }
        else if (Touchpad.IsKeyboardFocused && e.Key is Key.Up or Key.Down or Key.Space)
        {
            e.Handled = true;
            if (e.IsRepeat) return;
            if (e.Key == Key.Space) await SafeAsync(_input.ClickAsync);
            else await SwipeAsync(e.Key == Key.Down ? -1 : 1);
        }
    }

    private Task ResetAsync() => ResetAsync(clearSource: true);

    private Task ResetAsync(bool clearSource)
    {
        _inputEpoch++;
        _nextMove = DateTime.MinValue;
        _dx = _dy = 0;
        _lastPoint = null;
        _screenPressed = false;
        _screenDragging = false;
        if (clearSource) _screenBridge.ClearSource();
        return _input.ResetAsync();
    }

    private async Task SafeAsync(Func<Task> action)
    {
        try { await action(); }
        catch (Exception ex)
        {
            ShowError(ex);
            try { await ResetAsync(); }
            catch (Exception releaseError) { ((App)Application.Current).Log(releaseError.ToString()); }
        }
    }

    private void ShowError(Exception ex)
    {
        ErrorText.Text = ex.Message;
        ((App)Application.Current).Log(ex.ToString());
    }

    private void OnBluetoothSettings(object sender, RoutedEventArgs e)
    {
        try { Process.Start(new ProcessStartInfo("ms-settings:bluetooth") { UseShellExecute = true }); }
        catch (Exception ex) { ShowError(ex); }
    }

    private void OnTopmostChanged(object sender, RoutedEventArgs e) => Topmost = ((CheckBox)sender).IsChecked == true;
    private void OnSourceInitialized(object? sender, EventArgs e) =>
        HwndSource.FromHwnd(new WindowInteropHelper(this).Handle)?.AddHook(WindowMessage);
    private IntPtr WindowMessage(IntPtr hwnd, int msg, IntPtr wParam, IntPtr lParam, ref bool handled)
    {
        if (msg == 0x004A) return _screenBridge.Handle(wParam, lParam, ref handled);
        if (msg == App.ActivateMessage)
        {
            if (WindowState == WindowState.Minimized) WindowState = WindowState.Normal;
            Show();
            Activate();
            handled = true;
        }
        return IntPtr.Zero;
    }

    private async void OnClosing(object? sender, CancelEventArgs e)
    {
        if (_closed) return;
        e.Cancel = true;
        if (_closing) return;
        _closing = true;
        _screenBridge.Readiness = 0;
        _timer.Stop();
        InputPanel.IsEnabled = false;
        if (_startupTask is not null)
        {
            try { await _startupTask; }
            catch (Exception ex) { ((App)Application.Current).Log(ex.ToString()); }
        }
        await SafeAsync(ResetAsync);
        _hostId = null;
        _peripheral.SelectLocal();
        await _peripheral.DisposeAsync();
        _owner?.Dispose();
        _closed = true;
        // Cleanup can finish synchronously inside Closing; close on the next dispatch.
        _ = Dispatcher.BeginInvoke(Close);
    }
}
