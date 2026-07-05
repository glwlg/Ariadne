using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media.Imaging;

namespace Ariadne.CaptureHost;

internal static partial class NativeMethods
{
    public const int HtCaption = 0x0002;
    public const int SwpNoZOrder = 0x0004;
    public const int SwpShowWindow = 0x0040;
    public const int WmNcLButtonDown = 0x00A1;
    private const int LogPixelsX = 88;
    private const int LogPixelsY = 90;
    private const uint Srccopy = 0x00CC0020;
    private const uint CaptureBlt = 0x40000000;

    private static readonly IntPtr DpiAwarenessContextPerMonitorAwareV2 = new(-4);

    public static void EnablePerMonitorDpiAwareness()
    {
        try
        {
            SetProcessDpiAwarenessContext(DpiAwarenessContextPerMonitorAwareV2);
        }
        catch
        {
            // Older Windows builds still run correctly with the manifest DPI setting.
        }
    }

    public static void PlaceWindowInPhysicalPixels(System.Windows.Window window, int x, int y, int width, int height)
    {
        var hwnd = new WindowInteropHelper(window).Handle;
        if (hwnd == IntPtr.Zero)
        {
            return;
        }
        SetWindowPos(hwnd, IntPtr.Zero, x, y, Math.Max(1, width), Math.Max(1, height), SwpNoZOrder | SwpShowWindow);
    }

    public static Int32Rect WindowRect(System.Windows.Window window)
    {
        var hwnd = new WindowInteropHelper(window).Handle;
        if (hwnd == IntPtr.Zero || !GetWindowRect(hwnd, out var rect))
        {
            return new Int32Rect(0, 0, 1, 1);
        }
        return new Int32Rect(rect.Left, rect.Top, Math.Max(1, rect.Right - rect.Left), Math.Max(1, rect.Bottom - rect.Top));
    }

    public static (double ScaleX, double ScaleY) DpiScaleForPoint(int x, int y)
    {
        try
        {
            var point = new POINT { X = x, Y = y };
            var monitor = MonitorFromPoint(point, 2);
            if (monitor != IntPtr.Zero && GetDpiForMonitor(monitor, 0, out var dpiX, out var dpiY) == 0)
            {
                return (dpiX / 96.0, dpiY / 96.0);
            }
        }
        catch
        {
            // Fall through to the desktop DC fallback.
        }

        var desktopDc = GetDC(IntPtr.Zero);
        if (desktopDc != IntPtr.Zero)
        {
            try
            {
                var dpiX = GetDeviceCaps(desktopDc, LogPixelsX);
                var dpiY = GetDeviceCaps(desktopDc, LogPixelsY);
                if (dpiX > 0 && dpiY > 0)
                {
                    return (dpiX / 96.0, dpiY / 96.0);
                }
            }
            finally
            {
                ReleaseDC(IntPtr.Zero, desktopDc);
            }
        }
        return (1, 1);
    }

    public static IReadOnlyList<Int32Rect> MonitorBounds()
    {
        var bounds = new List<Int32Rect>();
        MonitorEnumProc callback = (IntPtr monitor, IntPtr dc, ref RECT rect, IntPtr data) =>
        {
            bounds.Add(new Int32Rect(rect.Left, rect.Top, Math.Max(1, rect.Right - rect.Left), Math.Max(1, rect.Bottom - rect.Top)));
            return true;
        };
        if (!EnumDisplayMonitors(IntPtr.Zero, IntPtr.Zero, callback, IntPtr.Zero))
        {
            throw LastWin32Error("EnumDisplayMonitors");
        }
        return bounds;
    }

    public static BitmapSource CaptureRegion(Int32Rect bounds)
    {
        var screenDc = GetDC(IntPtr.Zero);
        if (screenDc == IntPtr.Zero)
        {
            throw LastWin32Error("GetDC");
        }

        var memoryDc = IntPtr.Zero;
        var bitmap = IntPtr.Zero;
        var previous = IntPtr.Zero;
        try
        {
            memoryDc = CreateCompatibleDC(screenDc);
            if (memoryDc == IntPtr.Zero)
            {
                throw LastWin32Error("CreateCompatibleDC");
            }
            bitmap = CreateCompatibleBitmap(screenDc, bounds.Width, bounds.Height);
            if (bitmap == IntPtr.Zero)
            {
                throw LastWin32Error("CreateCompatibleBitmap");
            }
            previous = SelectObject(memoryDc, bitmap);
            if (previous == IntPtr.Zero)
            {
                throw LastWin32Error("SelectObject");
            }
            if (!BitBlt(memoryDc, 0, 0, bounds.Width, bounds.Height, screenDc, bounds.X, bounds.Y, Srccopy | CaptureBlt))
            {
                throw LastWin32Error("BitBlt");
            }
            var source = Imaging.CreateBitmapSourceFromHBitmap(bitmap, IntPtr.Zero, Int32Rect.Empty, BitmapSizeOptions.FromEmptyOptions());
            source.Freeze();
            return source;
        }
        finally
        {
            if (previous != IntPtr.Zero && memoryDc != IntPtr.Zero)
            {
                SelectObject(memoryDc, previous);
            }
            if (bitmap != IntPtr.Zero)
            {
                DeleteObject(bitmap);
            }
            if (memoryDc != IntPtr.Zero)
            {
                DeleteDC(memoryDc);
            }
            ReleaseDC(IntPtr.Zero, screenDc);
        }
    }

    public static Win32Exception LastWin32Error(string operation)
    {
        return new Win32Exception(Marshal.GetLastWin32Error(), operation);
    }

    [LibraryImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool SetProcessDpiAwarenessContext(IntPtr dpiContext);

    [LibraryImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static partial bool SetWindowPos(IntPtr hwnd, IntPtr hwndInsertAfter, int x, int y, int cx, int cy, uint flags);

    [LibraryImport("user32.dll")]
    public static partial IntPtr MonitorFromPoint(POINT point, uint flags);

    [LibraryImport("shcore.dll")]
    public static partial int GetDpiForMonitor(IntPtr hmonitor, int dpiType, out uint dpiX, out uint dpiY);

    [LibraryImport("gdi32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static partial bool DeleteObject(IntPtr hObject);

    [LibraryImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static partial bool ReleaseCapture();

    [LibraryImport("user32.dll")]
    public static partial IntPtr SendMessageW(IntPtr hwnd, uint msg, IntPtr wParam, IntPtr lParam);

    [LibraryImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool GetWindowRect(IntPtr hwnd, out RECT rect);

    private delegate bool MonitorEnumProc(IntPtr hMonitor, IntPtr hdcMonitor, ref RECT lprcMonitor, IntPtr dwData);

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool EnumDisplayMonitors(IntPtr hdc, IntPtr clipRect, MonitorEnumProc callback, IntPtr data);

    [DllImport("user32.dll", SetLastError = true)]
    private static extern IntPtr GetDC(IntPtr hwnd);

    [DllImport("user32.dll")]
    private static extern int ReleaseDC(IntPtr hwnd, IntPtr hdc);

    [DllImport("gdi32.dll", SetLastError = true)]
    private static extern IntPtr CreateCompatibleDC(IntPtr hdc);

    [DllImport("gdi32.dll", SetLastError = true)]
    private static extern IntPtr CreateCompatibleBitmap(IntPtr hdc, int cx, int cy);

    [DllImport("gdi32.dll", SetLastError = true)]
    private static extern IntPtr SelectObject(IntPtr hdc, IntPtr hObject);

    [DllImport("gdi32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool BitBlt(IntPtr hdc, int x, int y, int cx, int cy, IntPtr hdcSrc, int x1, int y1, uint rop);

    [DllImport("gdi32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool DeleteDC(IntPtr hdc);

    [DllImport("gdi32.dll")]
    private static extern int GetDeviceCaps(IntPtr hdc, int index);

    [StructLayout(LayoutKind.Sequential)]
    public struct POINT
    {
        public int X;
        public int Y;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct RECT
    {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }
}
