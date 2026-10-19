namespace Ariadne.CaptureHost;

internal static class Program
{
    public static void Main()
    {
        var root = Path.Combine(Path.GetTempPath(), "ariadne-color-format-tests-" + Guid.NewGuid().ToString("N"));
        var path = Path.Combine(root, "nested", "capture-color-format.txt");
        try
        {
            AssertEqual(ColorFormat.Rgb, ColorFormatPreferences.Load(path), "missing preference defaults to RGB");

            ColorFormatPreferences.Save(path, ColorFormat.Hex);
            AssertEqual(ColorFormat.Hex, ColorFormatPreferences.Load(path), "saved HEX preference is reloaded");

            ColorFormatPreferences.Save(path, ColorFormat.Rgb);
            AssertEqual(ColorFormat.Rgb, ColorFormatPreferences.Load(path), "saved RGB preference replaces HEX");

            File.WriteAllText(path, "unknown");
            AssertEqual(ColorFormat.Rgb, ColorFormatPreferences.Load(path), "invalid preference defaults to RGB");

            AssertEqual(
                ColorPickerShortcutAction.ToggleFormat,
                ColorPickerShortcutResolver.Resolve(isShiftKey: true, isCKey: false, controlDown: false, isRepeat: false),
                "initial Shift keydown toggles the color format");
            AssertEqual(
                ColorPickerShortcutAction.Consume,
                ColorPickerShortcutResolver.Resolve(isShiftKey: true, isCKey: false, controlDown: false, isRepeat: true),
                "repeated Shift keydown does not toggle again");
            AssertEqual(
                ColorPickerShortcutAction.CopyColor,
                ColorPickerShortcutResolver.Resolve(isShiftKey: false, isCKey: true, controlDown: false, isRepeat: false),
                "C still copies after a Shift event");
            AssertEqual(
                ColorPickerShortcutAction.None,
                ColorPickerShortcutResolver.Resolve(isShiftKey: false, isCKey: true, controlDown: true, isRepeat: false),
                "Ctrl+C is not consumed by the color picker");

            var monitors = new (int X, int Y, int Width, int Height)[]
            {
                (0, 0, 1920, 1080),
                (1920, 0, 2560, 1440)
            };
            AssertEqual(
                1,
                OverlayFocusTargetResolver.SelectTargetIndex(monitors, cursorX: 2200, cursorY: 400),
                "selects the overlay monitor containing the cursor");
            AssertEqual(
                0,
                OverlayFocusTargetResolver.SelectTargetIndex(monitors, cursorX: -50, cursorY: 200),
                "falls back to nearest monitor when cursor is outside bounds");
            AssertEqual(
                -1,
                OverlayFocusTargetResolver.SelectTargetIndex(Array.Empty<(int X, int Y, int Width, int Height)>(), cursorX: 0, cursorY: 0),
                "returns -1 when no monitors exist");

            AssertEqual(
                true,
                OverlayFocusTargetResolver.ShouldRetryInitialFocus(
                    completed: false,
                    hasActiveTextEditor: false,
                    isWindowForeground: false,
                    isWindowActive: true,
                    isOverlayKeyboardFocused: true,
                    attempt: 0,
                    maxAttempts: 8),
                "retries initial focus when another window such as network mini still holds foreground");
            AssertEqual(
                true,
                OverlayFocusTargetResolver.ShouldRetryInitialFocus(
                    completed: false,
                    hasActiveTextEditor: false,
                    isWindowForeground: true,
                    isWindowActive: true,
                    isOverlayKeyboardFocused: false,
                    attempt: 2,
                    maxAttempts: 8),
                "retries initial focus when overlay canvas does not yet own keyboard focus");
            AssertEqual(
                false,
                OverlayFocusTargetResolver.ShouldRetryInitialFocus(
                    completed: false,
                    hasActiveTextEditor: false,
                    isWindowForeground: true,
                    isWindowActive: true,
                    isOverlayKeyboardFocused: true,
                    attempt: 2,
                    maxAttempts: 8),
                "stops retrying once overlay window is foreground, active, and keyboard-focused");
            AssertEqual(
                false,
                OverlayFocusTargetResolver.ShouldRetryInitialFocus(
                    completed: false,
                    hasActiveTextEditor: true,
                    isWindowForeground: false,
                    isWindowActive: false,
                    isOverlayKeyboardFocused: false,
                    attempt: 0,
                    maxAttempts: 8),
                "does not steal focus back from the text annotation editor");
            AssertEqual(
                false,
                OverlayFocusTargetResolver.ShouldRetryInitialFocus(
                    completed: true,
                    hasActiveTextEditor: false,
                    isWindowForeground: false,
                    isWindowActive: false,
                    isOverlayKeyboardFocused: false,
                    attempt: 0,
                    maxAttempts: 8),
                "does not keep focusing after capture is completed");
            AssertEqual(
                false,
                OverlayFocusTargetResolver.ShouldRetryInitialFocus(
                    completed: false,
                    hasActiveTextEditor: false,
                    isWindowForeground: false,
                    isWindowActive: false,
                    isOverlayKeyboardFocused: false,
                    attempt: 8,
                    maxAttempts: 8),
                "stops the initial focus retry after the attempt budget");
            AssertEqual(
                true,
                OverlayFocusTargetResolver.ShouldRefocusOverlayOnPointerMove(
                    completed: false,
                    hasActiveTextEditor: false,
                    isWindowActive: false,
                    isOverlayKeyboardFocusWithin: false),
                "pointer move refocuses inactive overlay without requiring a click");
            AssertEqual(
                false,
                OverlayFocusTargetResolver.ShouldRefocusOverlayOnPointerMove(
                    completed: false,
                    hasActiveTextEditor: true,
                    isWindowActive: true,
                    isOverlayKeyboardFocusWithin: true),
                "pointer move does not steal focus from active text annotation editor");

            Console.WriteLine("Color picker shortcut and preference tests passed.");
        }
        finally
        {
            if (Directory.Exists(root))
            {
                Directory.Delete(root, true);
            }
        }
    }

    private static void AssertEqual<T>(T expected, T actual, string behavior)
    {
        if (!EqualityComparer<T>.Default.Equals(expected, actual))
        {
            throw new InvalidOperationException($"{behavior}: expected {expected}, got {actual}");
        }
    }
}
