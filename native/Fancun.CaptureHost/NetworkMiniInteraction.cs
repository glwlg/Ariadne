namespace Fancun.CaptureHost;

internal sealed class NetworkMiniClickTracker
{
    private bool _hasPrevious;
    private uint _previousTime;
    private int _previousX;
    private int _previousY;

    public bool Register(uint time, int x, int y, uint interval, int width, int height)
    {
        var doubleClick = _hasPrevious && unchecked(time - _previousTime) <= interval
            && Math.Abs((long)x - _previousX) * 2 <= width
            && Math.Abs((long)y - _previousY) * 2 <= height;
        _hasPrevious = !doubleClick;
        _previousTime = time;
        _previousX = x;
        _previousY = y;
        return doubleClick;
    }
}
