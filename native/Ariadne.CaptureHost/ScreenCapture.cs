using System.IO;
using System.Windows;
using System.Windows.Media;
using System.Windows.Media.Imaging;

namespace Ariadne.CaptureHost;

internal sealed class ScreenCapture : IDisposable
{
    private ScreenCapture(Int32Rect bounds, BitmapSource source, double scaleX, double scaleY)
    {
        Bounds = bounds;
        Source = source;
        ScaleX = scaleX;
        ScaleY = scaleY;
    }

    public BitmapSource Source { get; }
    public double ScaleX { get; }
    public double ScaleY { get; }
    public Int32Rect Bounds { get; }
    public double DipWidth => Bounds.Width / ScaleX;
    public double DipHeight => Bounds.Height / ScaleY;

    public static IReadOnlyList<ScreenCapture> CaptureAll()
    {
        var captures = new List<ScreenCapture>();
        foreach (var bounds in NativeMethods.MonitorBounds())
        {
            captures.Add(Capture(bounds));
        }
        return captures;
    }

    public byte[] CropPng(Int32Rect physicalRect)
    {
        var crop = CropSource(physicalRect);
        var encoder = new PngBitmapEncoder();
        encoder.Frames.Add(BitmapFrame.Create(crop));
        using var stream = new MemoryStream();
        encoder.Save(stream);
        return stream.ToArray();
    }

    public BitmapSource CropSource(Int32Rect physicalRect)
    {
        var rect = ClampRect(physicalRect);
        var crop = new CroppedBitmap(Source, rect);
        crop.Freeze();
        return crop;
    }

    public Color SamplePixel(int physicalX, int physicalY)
    {
        var x = Math.Clamp(physicalX, 0, Source.PixelWidth - 1);
        var y = Math.Clamp(physicalY, 0, Source.PixelHeight - 1);
        BitmapSource source = Source;
        if (source.Format != PixelFormats.Bgra32)
        {
            var converted = new FormatConvertedBitmap(source, PixelFormats.Bgra32, null, 0);
            converted.Freeze();
            source = converted;
        }
        var buffer = new byte[4];
        source.CopyPixels(new Int32Rect(x, y, 1, 1), buffer, 4, 0);
        return Color.FromRgb(buffer[2], buffer[1], buffer[0]);
    }

    public void Dispose()
    {
    }

    private Int32Rect ClampRect(Int32Rect physicalRect)
    {
        var x = Math.Clamp(physicalRect.X, 0, Source.PixelWidth - 1);
        var y = Math.Clamp(physicalRect.Y, 0, Source.PixelHeight - 1);
        var width = Math.Clamp(physicalRect.Width, 1, Source.PixelWidth - x);
        var height = Math.Clamp(physicalRect.Height, 1, Source.PixelHeight - y);
        return new Int32Rect(x, y, width, height);
    }

    private static ScreenCapture Capture(Int32Rect bounds)
    {
        var source = NativeMethods.CaptureRegion(bounds);
        var scaleX = source.PixelWidth / (double)Math.Max(1, bounds.Width);
        var scaleY = source.PixelHeight / (double)Math.Max(1, bounds.Height);
        return new ScreenCapture(bounds, source, scaleX, scaleY);
    }
}
