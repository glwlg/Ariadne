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

            Console.WriteLine("Color format preference tests passed.");
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
}
