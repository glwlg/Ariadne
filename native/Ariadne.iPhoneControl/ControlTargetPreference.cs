namespace Ariadne.iPhoneControl;

internal sealed class ControlTargetPreference(string path)
{
    private string? _deviceId;

    public void Load() => _deviceId = File.Exists(path) ? File.ReadAllText(path).Trim() : null;

    public string? Choose(IReadOnlyList<string> connectedIds)
    {
        if (!string.IsNullOrEmpty(_deviceId))
            return connectedIds.FirstOrDefault(id => id == _deviceId);
        return connectedIds.Count == 1 ? connectedIds[0] : null;
    }

    public void Remember(string deviceId)
    {
        if (_deviceId == deviceId) return;
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        File.WriteAllText(path + ".tmp", deviceId);
        File.Move(path + ".tmp", path, overwrite: true);
        _deviceId = deviceId;
    }
}
