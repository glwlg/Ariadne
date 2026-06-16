package perfcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarize(t *testing.T) {
	summary := summarize([]int64{120, 80, 100, 140})
	if summary.Count != 4 {
		t.Fatalf("Count = %d, want 4", summary.Count)
	}
	if summary.Min != 80 || summary.Max != 140 {
		t.Fatalf("range = %d..%d, want 80..140", summary.Min, summary.Max)
	}
	if summary.Average != 110 {
		t.Fatalf("Average = %f, want 110", summary.Average)
	}
	if summary.P95 != 140 {
		t.Fatalf("P95 = %d, want 140", summary.P95)
	}
}

func TestPackageComparison(t *testing.T) {
	root := t.TempDir()
	exe := writeSizedFile(t, filepath.Join(root, "ariadne.exe"), 30)
	zip := writeSizedFile(t, filepath.Join(root, "ariadne.zip"), 20)
	legacyInstaller := writeSizedFile(t, filepath.Join(root, "x-tools-setup.exe"), 100)
	legacyDist := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacyDist, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSizedFile(t, filepath.Join(legacyDist, "one.bin"), 40)
	writeSizedFile(t, filepath.Join(legacyDist, "two.bin"), 60)

	result := packageComparison(Options{
		ExePath:             exe,
		ReleaseZipPath:      zip,
		LegacyInstallerPath: legacyInstaller,
		LegacyDistPath:      legacyDist,
	})

	if result.ExeBytes != 30 || result.ReleaseZipBytes != 20 {
		t.Fatalf("package sizes = exe %d zip %d, want 30 20", result.ExeBytes, result.ReleaseZipBytes)
	}
	if result.LegacyInstallerBytes != 100 || result.LegacyDistBytes != 100 || result.LegacyDistFiles != 2 {
		t.Fatalf("legacy = installer %d dist %d files %d, want 100 100 2", result.LegacyInstallerBytes, result.LegacyDistBytes, result.LegacyDistFiles)
	}
	if !result.PackageComparisonAvailable {
		t.Fatal("PackageComparisonAvailable = false, want true")
	}
	if result.ReleaseZipReductionPct != 80 {
		t.Fatalf("ReleaseZipReductionPct = %f, want 80", result.ReleaseZipReductionPct)
	}
	if result.ReleaseVsLegacyDistPct != 80 {
		t.Fatalf("ReleaseVsLegacyDistPct = %f, want 80", result.ReleaseVsLegacyDistPct)
	}
}

func TestBudgetVerdictWarnsForFailedSamples(t *testing.T) {
	report := Report{
		Budgets: DefaultBudgets(),
		Package: PackageComparison{
			ReleaseZipBytes:      20,
			LegacyInstallerBytes: 100,
		},
		Startup: MetricSummary{
			Count: 1,
			P95:   700,
		},
		Hotkey: MetricSummary{
			Count: 1,
			P95:   90,
		},
		Samples: []StartupSample{
			{Iteration: 1, StartupMs: 700},
			{Iteration: 2, Error: "boom"},
		},
	}

	verdict := budgetVerdict(report)
	if !verdict.ColdStartWithinTarget {
		t.Fatal("ColdStartWithinTarget = false, want true")
	}
	if verdict.ColdStartWithinIdeal {
		t.Fatal("ColdStartWithinIdeal = true, want false")
	}
	if !verdict.HotkeyWithinTarget {
		t.Fatal("HotkeyWithinTarget = false, want true")
	}
	if !verdict.PackageSmallerThanLegacy {
		t.Fatal("PackageSmallerThanLegacy = false, want true")
	}
	if len(verdict.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one sample warning", verdict.Warnings)
	}
}

func writeSizedFile(t *testing.T, path string, size int) string {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
