namespace Ariadne.CaptureHost;

internal sealed class CaptureRequest
{
    public string Command { get; set; } = "capture";
    public bool AutoPin { get; set; }
    public bool DirectClipboardCopy { get; set; }
    public string CallbackPipeName { get; set; } = "";
}

internal sealed class CaptureResponse
{
    public bool Ok { get; set; }
    public bool Canceled { get; set; }
    public string Message { get; set; } = "";
    public string Action { get; set; } = "";
    public string PngBase64 { get; set; } = "";
    public int X { get; set; }
    public int Y { get; set; }
    public int Width { get; set; }
    public int Height { get; set; }
    public string SavedPath { get; set; } = "";
    public bool ClipboardWritten { get; set; }
    public bool Pinned { get; set; }
    public bool PinPositioned { get; set; }
    public int PinX { get; set; }
    public int PinY { get; set; }
    public string NativePinId { get; set; } = "";
    public long RenderMs { get; set; }
    public long ClipboardMs { get; set; }
    public long TotalMs { get; set; }
    public List<AnnotationOperation> Operations { get; set; } = [];
}

internal sealed class PinActionRequest
{
    public string Action { get; set; } = "";
    public string NativePinId { get; set; } = "";
    public string ImagePath { get; set; } = "";
}

internal sealed class PinActionResponse
{
    public bool Ok { get; set; }
    public string Message { get; set; } = "";
    public string Text { get; set; } = "";
    public string PngBase64 { get; set; } = "";
    public int Width { get; set; }
    public int Height { get; set; }
    public bool ClipboardWritten { get; set; }
}

internal sealed class AnnotationPoint
{
    public int X { get; set; }
    public int Y { get; set; }
}

internal sealed class AnnotationOperation
{
    public string Kind { get; set; } = "";
    public int X { get; set; }
    public int Y { get; set; }
    public int Width { get; set; }
    public int Height { get; set; }
    public int EndX { get; set; }
    public int EndY { get; set; }
    public string Color { get; set; } = "";
    public int StrokeWidth { get; set; }
    public int PixelSize { get; set; }
    public List<AnnotationPoint>? Points { get; set; }
    public string Text { get; set; } = "";
    public int FontSize { get; set; }
    public int Number { get; set; }
}
