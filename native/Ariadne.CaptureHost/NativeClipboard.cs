using System.IO;
using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Media;
using System.Windows.Media.Imaging;

namespace Ariadne.CaptureHost;

internal static class NativeClipboard
{
    private const uint CfDib = 8;
    private const uint GmemMoveable = 0x0002;
    private const int BitmapInfoHeaderSize = 40;
    private const short Planes = 1;
    private const short BitsPerPixel = 32;
    private const int BiRgb = 0;

    public static void WriteTextWithRetry(string text)
    {
        Exception? lastError = null;
        for (var attempt = 0; attempt < 12; attempt++)
        {
            try
            {
                Clipboard.SetText(text);
                return;
            }
            catch (Exception ex)
            {
                lastError = ex;
                Thread.Sleep(15 + attempt * 10);
            }
        }
        throw new InvalidOperationException(lastError?.Message ?? "剪贴板不可用", lastError);
    }

    public static void WritePngWithRetry(byte[] png)
    {
        var dib = PngToDib(png);
        Exception? lastError = null;
        for (var attempt = 0; attempt < 12; attempt++)
        {
            try
            {
                WriteDibOnce(dib);
                return;
            }
            catch (Exception ex)
            {
                lastError = ex;
                Thread.Sleep(15 + attempt * 10);
            }
        }
        throw new InvalidOperationException(lastError?.Message ?? "剪贴板不可用", lastError);
    }

    private static byte[] PngToDib(byte[] png)
    {
        var source = DecodePng(png);
        BitmapSource bitmap = source.Format == PixelFormats.Bgra32
            ? source
            : new FormatConvertedBitmap(source, PixelFormats.Bgra32, null, 0);
        bitmap.Freeze();

        var width = bitmap.PixelWidth;
        var height = bitmap.PixelHeight;
        if (width <= 0 || height <= 0)
        {
            throw new InvalidOperationException("剪贴板图片尺寸无效");
        }

        var stride = checked(width * 4);
        var pixels = new byte[checked(stride * height)];
        bitmap.CopyPixels(pixels, stride, 0);

        var dib = new byte[checked(BitmapInfoHeaderSize + pixels.Length)];
        WriteInt32(dib, 0, BitmapInfoHeaderSize);
        WriteInt32(dib, 4, width);
        WriteInt32(dib, 8, height);
        WriteInt16(dib, 12, Planes);
        WriteInt16(dib, 14, BitsPerPixel);
        WriteInt32(dib, 16, BiRgb);
        WriteInt32(dib, 20, pixels.Length);

        var targetOffset = BitmapInfoHeaderSize;
        for (var y = height - 1; y >= 0; y--)
        {
            Buffer.BlockCopy(pixels, y * stride, dib, targetOffset, stride);
            targetOffset += stride;
        }
        return dib;
    }

    private static BitmapSource DecodePng(byte[] png)
    {
        using var stream = new MemoryStream(png);
        var image = new BitmapImage();
        image.BeginInit();
        image.CacheOption = BitmapCacheOption.OnLoad;
        image.StreamSource = stream;
        image.EndInit();
        image.Freeze();
        return image;
    }

    private static void WriteDibOnce(byte[] dib)
    {
        if (!OpenClipboard(IntPtr.Zero))
        {
            throw NativeMethods.LastWin32Error("OpenClipboard");
        }

        var handle = IntPtr.Zero;
        try
        {
            if (!EmptyClipboard())
            {
                throw NativeMethods.LastWin32Error("EmptyClipboard");
            }

            handle = GlobalAlloc(GmemMoveable, new UIntPtr((uint)dib.Length));
            if (handle == IntPtr.Zero)
            {
                throw NativeMethods.LastWin32Error("GlobalAlloc");
            }

            var pointer = GlobalLock(handle);
            if (pointer == IntPtr.Zero)
            {
                throw NativeMethods.LastWin32Error("GlobalLock");
            }
            try
            {
                Marshal.Copy(dib, 0, pointer, dib.Length);
            }
            finally
            {
                GlobalUnlock(handle);
            }

            if (SetClipboardData(CfDib, handle) == IntPtr.Zero)
            {
                throw NativeMethods.LastWin32Error("SetClipboardData");
            }
            handle = IntPtr.Zero;

            if (!IsClipboardFormatAvailable(CfDib))
            {
                throw new InvalidOperationException("剪贴板未接受图片");
            }
        }
        finally
        {
            if (handle != IntPtr.Zero)
            {
                GlobalFree(handle);
            }
            CloseClipboard();
        }
    }

    private static void WriteInt16(byte[] data, int offset, short value)
    {
        var bytes = BitConverter.GetBytes(value);
        Buffer.BlockCopy(bytes, 0, data, offset, bytes.Length);
    }

    private static void WriteInt32(byte[] data, int offset, int value)
    {
        var bytes = BitConverter.GetBytes(value);
        Buffer.BlockCopy(bytes, 0, data, offset, bytes.Length);
    }

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool OpenClipboard(IntPtr hwnd);

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CloseClipboard();

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool EmptyClipboard();

    [DllImport("user32.dll", SetLastError = true)]
    private static extern IntPtr SetClipboardData(uint format, IntPtr handle);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool IsClipboardFormatAvailable(uint format);

    [DllImport("kernel32.dll", SetLastError = true)]
    private static extern IntPtr GlobalAlloc(uint flags, UIntPtr bytes);

    [DllImport("kernel32.dll", SetLastError = true)]
    private static extern IntPtr GlobalLock(IntPtr handle);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GlobalUnlock(IntPtr handle);

    [DllImport("kernel32.dll", SetLastError = true)]
    private static extern IntPtr GlobalFree(IntPtr handle);
}
