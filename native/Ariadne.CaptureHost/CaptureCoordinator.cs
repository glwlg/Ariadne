using System.IO;
using System.Windows;
using System.Windows.Media.Imaging;

namespace Ariadne.CaptureHost;

internal static class CaptureCoordinator
{
    public static CaptureResponse PinClipboard()
    {
        try
        {
            if (!Clipboard.ContainsImage())
            {
                return new CaptureResponse { Ok = false, Message = "当前剪贴板没有可贴图的图片" };
            }
            var image = Clipboard.GetImage();
            if (image == null || image.PixelWidth <= 0 || image.PixelHeight <= 0)
            {
                return new CaptureResponse { Ok = false, Message = "当前剪贴板没有可贴图的图片" };
            }
            image.Freeze();
            var png = EncodePng(image);
            var cursor = NativeMethods.CursorPos();
            var nativePinId = Guid.NewGuid().ToString("N");
            new PinWindow(
                png,
                cursor.X,
                cursor.Y,
                image.PixelWidth,
                image.PixelHeight,
                nativePinId,
                "").Show();
            return new CaptureResponse
            {
                Ok = true,
                Message = "已创建贴图",
                Action = "pin",
                Pinned = true,
                PinPositioned = true,
                PinX = cursor.X,
                PinY = cursor.Y,
                Width = image.PixelWidth,
                Height = image.PixelHeight,
                NativePinId = nativePinId
            };
        }
        catch (Exception ex)
        {
            return new CaptureResponse { Ok = false, Message = "贴图失败: " + ex.Message };
        }
    }

    private static byte[] EncodePng(BitmapSource image)
    {
        var encoder = new PngBitmapEncoder();
        encoder.Frames.Add(BitmapFrame.Create(image));
        using var stream = new MemoryStream();
        encoder.Save(stream);
        return stream.ToArray();
    }

    public static async Task<CaptureResponse> CaptureAsync(CaptureRequest request)
    {
        IReadOnlyList<ScreenCapture> captures;
        try
        {
            captures = ScreenCapture.CaptureAll();
        }
        catch (Exception ex)
        {
            return new CaptureResponse { Ok = false, Message = "截图失败: " + ex.Message };
        }

        if (captures.Count == 0)
        {
            return new CaptureResponse { Ok = false, Message = "未找到可用显示器" };
        }

        var completion = new TaskCompletionSource<CaptureResponse>();
        var windows = captures.Select(capture => new OverlayWindow(capture, request)).ToList();

        foreach (var window in windows)
        {
            window.Completed += response =>
            {
                completion.TrySetResult(response);
            };
            window.Show();
        }

        var result = await completion.Task;
        foreach (var window in windows)
        {
            window.CloseFromCoordinator();
        }
        foreach (var capture in captures)
        {
            capture.Dispose();
        }
        return result;
    }
}
