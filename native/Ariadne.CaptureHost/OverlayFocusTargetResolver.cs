namespace Ariadne.CaptureHost;

internal static class OverlayFocusTargetResolver
{
    public static int SelectTargetIndex(
        IReadOnlyList<(int X, int Y, int Width, int Height)> monitors,
        int cursorX,
        int cursorY)
    {
        if (monitors.Count == 0)
        {
            return -1;
        }

        for (var index = 0; index < monitors.Count; index++)
        {
            var monitor = monitors[index];
            if (cursorX >= monitor.X && cursorX < monitor.X + Math.Max(1, monitor.Width)
                && cursorY >= monitor.Y && cursorY < monitor.Y + Math.Max(1, monitor.Height))
            {
                return index;
            }
        }

        var nearest = 0;
        var nearestDistance = long.MaxValue;
        for (var index = 0; index < monitors.Count; index++)
        {
            var monitor = monitors[index];
            var centerX = monitor.X + monitor.Width / 2;
            var centerY = monitor.Y + monitor.Height / 2;
            var dx = (long)cursorX - centerX;
            var dy = (long)cursorY - centerY;
            var distance = dx * dx + dy * dy;
            if (distance < nearestDistance)
            {
                nearestDistance = distance;
                nearest = index;
            }
        }
        return nearest;
    }

    public static bool ShouldRetryInitialFocus(
        bool completed,
        bool hasActiveTextEditor,
        bool isWindowForeground,
        bool isWindowActive,
        bool isOverlayKeyboardFocused,
        int attempt,
        int maxAttempts)
    {
        if (completed || hasActiveTextEditor || attempt >= maxAttempts)
        {
            return false;
        }
        return !isWindowForeground || !isWindowActive || !isOverlayKeyboardFocused;
    }

    public static bool ShouldRefocusOverlayOnPointerMove(
        bool completed,
        bool hasActiveTextEditor,
        bool isWindowActive,
        bool isOverlayKeyboardFocusWithin)
    {
        if (completed || hasActiveTextEditor)
        {
            return false;
        }
        return !isWindowActive || !isOverlayKeyboardFocusWithin;
    }
}
