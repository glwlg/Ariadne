using System.Runtime.InteropServices;

namespace Ariadne.iPhoneControl;

public enum ScreenPointerKind { Query, Move, Down, Up, Wheel, Cancel }

[StructLayout(LayoutKind.Sequential)]
public readonly record struct ScreenPointerPacket(int Version, ScreenPointerKind Kind, int X, int Y, int Delta)
{
    public const uint DataTag = 0x41524950;
    public bool IsValid => Version == 1 && Enum.IsDefined(Kind) &&
        X is >= 0 and <= 32767 && Y is >= 0 and <= 32767 &&
        (Kind == ScreenPointerKind.Wheel ? Delta is -1 or 1 : Delta == 0);
}
