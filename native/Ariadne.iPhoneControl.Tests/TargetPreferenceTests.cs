using Ariadne.iPhoneControl;

internal static class TargetPreferenceTests
{
    public static void Run()
    {
        var directory = Path.Combine(Path.GetTempPath(), "AriadneControlTarget-" + Guid.NewGuid());
        Directory.CreateDirectory(directory);
        try
        {
            var path = Path.Combine(directory, "target.txt");
            var preference = new ControlTargetPreference(path);
            preference.Load();
            Assert(preference.Choose([]) is null, "No connected phone must leave control inactive.");
            Assert(preference.Choose(["phone-a", "phone-b"]) is null,
                "Multiple phones require an explicit first selection.");
            Assert(preference.Choose(["phone-a"]) == "phone-a", "A sole phone must be selected automatically.");
            preference.Remember("phone-a");
            var reopened = new ControlTargetPreference(path);
            reopened.Load();
            Assert(reopened.Choose(["phone-b", "phone-a"]) == "phone-a",
                "The remembered phone must be restored across process restarts.");
            Assert(reopened.Choose(["phone-b"]) is null,
                "Disconnecting the remembered phone must not redirect input to another phone.");
            Assert(reopened.Choose([]) is null && reopened.Choose(["phone-a"]) == "phone-a",
                "Disconnecting and reconnecting must retain the selected phone.");
            reopened.Remember("phone-b");
            var changed = new ControlTargetPreference(path);
            changed.Load();
            Assert(changed.Choose(["phone-a", "phone-b"]) == "phone-b",
                "An explicit new selection must replace the remembered target.");
            Assert(!File.Exists(path + ".tmp"), "A successful save must leave no temporary file.");
            File.Delete(path);
            Directory.CreateDirectory(path);
            var failed = false;
            try { reopened.Remember("phone-c"); }
            catch (Exception ex) when (ex is IOException or UnauthorizedAccessException) { failed = true; }
            Assert(failed && reopened.Choose(["phone-b", "phone-c"]) == "phone-b",
                "A failed save must report failure and preserve the previously committed selection.");
        }
        finally { Directory.Delete(directory, recursive: true); }
        Console.WriteLine("PASS automatic device selection, remembered target, reconnect, ambiguity and save failure");
    }

    private static void Assert(bool condition, string message)
    {
        if (!condition) throw new Exception(message);
    }
}
