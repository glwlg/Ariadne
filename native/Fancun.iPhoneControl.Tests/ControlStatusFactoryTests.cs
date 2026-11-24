using Fancun.iPhoneControl;

internal static class ControlStatusFactoryTests
{
    public static void Run()
    {
        var connected = ControlStatusFactory.Create(42, false, false, 1, "iPhone", 2, "", "diagnostics");
        Assert(connected.Phase == "direct" && !connected.Advertising &&
               connected.ConnectedDevices == 1 && connected.TargetName == "iPhone" &&
               connected.LastError.Contains("自动恢复"),
            "Interrupted advertising must not hide a connected phone's direct control readiness.");

        var touchpad = ControlStatusFactory.Create(42, false, false, 1, "iPhone", 1, "", "");
        Assert(touchpad.Phase == "touchpad", "A connected touchpad must retain its own readiness.");

        var choose = ControlStatusFactory.Create(42, false, false, 2, "", 0, "", "");
        Assert(choose.Phase == "select-device", "Connected phones must remain selectable during recovery.");

        var stopped = ControlStatusFactory.Create(42, false, false, 0, "", 0, "连接失败", "");
        Assert(stopped.Phase == "error" && stopped.LastError.Contains("连接失败") &&
               stopped.LastError.Contains("自动恢复"),
            "A disconnected phone must report both the input error and advertising recovery.");

        var partial = ControlStatusFactory.Create(42, true, true, 0, "", 0, "", "");
        Assert(partial.Phase == "waiting" && partial.Advertising && partial.LastError.Contains("数据不完整"),
            "An incomplete advertisement must remain active while warning about discovery.");

        var normal = ControlStatusFactory.Create(42, true, false, 1, "iPhone", 2, "", "");
        Assert(normal.Phase == "direct" && normal.LastError == "",
            "Successful recovery must clear the advertising warning.");
        Console.WriteLine("PASS separate advertising, phone readiness, recovery and incomplete-data status");
    }

    private static void Assert(bool condition, string message)
    {
        if (!condition) throw new Exception(message);
    }
}
