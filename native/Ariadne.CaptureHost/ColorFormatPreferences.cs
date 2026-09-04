using System.IO;
using System.Text;

namespace Ariadne.CaptureHost;

internal enum ColorFormat
{
    Rgb,
    Hex
}

internal static class ColorFormatPreferences
{
    private const string RgbValue = "rgb";
    private const string HexValue = "hex";
    private static readonly string PreferencePath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "Ariadne",
        "capture-color-format.txt");
    private static ColorFormat _current = Load(PreferencePath);

    public static ColorFormat Current => _current;

    public static bool TrySetCurrent(ColorFormat format)
    {
        _current = format;
        try
        {
            Save(PreferencePath, format);
            return true;
        }
        catch
        {
            return false;
        }
    }

    internal static ColorFormat Load(string path)
    {
        try
        {
            return File.ReadAllText(path).Trim().ToLowerInvariant() == HexValue
                ? ColorFormat.Hex
                : ColorFormat.Rgb;
        }
        catch
        {
            return ColorFormat.Rgb;
        }
    }

    internal static void Save(string path, ColorFormat format)
    {
        var directory = Path.GetDirectoryName(path);
        if (!string.IsNullOrWhiteSpace(directory))
        {
            Directory.CreateDirectory(directory);
        }

        var tempPath = path + "." + Guid.NewGuid().ToString("N") + ".tmp";
        try
        {
            File.WriteAllText(tempPath, format == ColorFormat.Hex ? HexValue : RgbValue, new UTF8Encoding(false));
            File.Move(tempPath, path, true);
        }
        finally
        {
            if (File.Exists(tempPath))
            {
                File.Delete(tempPath);
            }
        }
    }
}
