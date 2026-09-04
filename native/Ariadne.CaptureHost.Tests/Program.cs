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

    private static void AssertEqual(ColorFormat expected, ColorFormat actual, string behavior)
    {
        if (expected != actual)
        {
            throw new InvalidOperationException($"{behavior}: expected {expected}, got {actual}");
        }
    }

    private static void AssertEqual(ColorPickerShortcutAction expected, ColorPickerShortcutAction actual, string behavior)
    {
        if (expected != actual)
        {
            throw new InvalidOperationException($"{behavior}: expected {expected}, got {actual}");
        }
    }
}
