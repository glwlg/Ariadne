namespace Ariadne.CaptureHost;

internal static class CaptureCoordinator
{
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
