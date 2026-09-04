namespace Ariadne.CaptureHost;

internal enum ColorPickerShortcutAction
{
    None,
    Consume,
    ToggleFormat,
    CopyColor
}

internal static class ColorPickerShortcutResolver
{
    public static ColorPickerShortcutAction Resolve(bool isShiftKey, bool isCKey, bool controlDown, bool isRepeat)
    {
        if (isShiftKey)
        {
            return isRepeat ? ColorPickerShortcutAction.Consume : ColorPickerShortcutAction.ToggleFormat;
        }

        return isCKey && !controlDown
            ? ColorPickerShortcutAction.CopyColor
            : ColorPickerShortcutAction.None;
    }
}
