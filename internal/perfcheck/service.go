package perfcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func Run(options Options) Report {
	options = normalizeOptions(options)
	report := Report{
		ProductName:        "Ariadne",
		CreatedAt:          time.Now().Unix(),
		Options:            options,
		Budgets:            DefaultBudgets(),
		Package:            packageComparison(options),
		HotkeyRegistration: probeHotkeyRegistration(options),
		VerificationNotes:  []string{},
	}
	for iteration := 1; iteration <= options.Iterations; iteration++ {
		report.Samples = append(report.Samples, probeStartup(options, iteration))
	}
	report.HotkeySamples = probeHotkey(options)
	startupValues := make([]int64, 0, len(report.Samples))
	hotkeyValues := make([]int64, 0, len(report.HotkeySamples))
	memoryValues := make([]int64, 0, len(report.Samples))
	for _, sample := range report.Samples {
		if sample.Error == "" && sample.StartupMs > 0 {
			startupValues = append(startupValues, sample.StartupMs)
		}
		if sample.Error == "" && sample.WorkingSetBytes > 0 {
			memoryValues = append(memoryValues, sample.WorkingSetBytes)
		}
	}
	for _, sample := range report.HotkeySamples {
		if sample.Error == "" && sample.HotkeyMs > 0 {
			hotkeyValues = append(hotkeyValues, sample.HotkeyMs)
		}
	}
	report.Startup = summarize(startupValues)
	report.Hotkey = summarize(hotkeyValues)
	report.Memory = summarize(memoryValues)
	report.BudgetVerdict = budgetVerdict(report)
	report.VerificationNotes = verificationNotes(report)
	return report
}

func normalizeOptions(options Options) Options {
	if options.ExePath == "" {
		options.ExePath = filepath.Join("bin", "ariadne.exe")
	}
	if options.ReleaseZipPath == "" {
		options.ReleaseZipPath = filepath.Join("dist", "release", "ariadne-dev-windows-x64.zip")
	}
	if options.LegacyInstallerPath == "" {
		options.LegacyInstallerPath = filepath.Join("..", "..", "dist", "x-tools-setup.exe")
	}
	if options.LegacyDistPath == "" {
		options.LegacyDistPath = filepath.Join("..", "..", "dist", "x-tools")
	}
	if options.Iterations <= 0 {
		options.Iterations = 3
	}
	if options.HotkeyIterations <= 0 {
		options.HotkeyIterations = options.Iterations
	}
	if options.TimeoutMs <= 0 {
		options.TimeoutMs = 8000
	}
	return options
}

func packageComparison(options Options) PackageComparison {
	exeBytes := fileBytes(options.ExePath)
	releaseZipBytes := fileBytes(options.ReleaseZipPath)
	legacyInstallerBytes := fileBytes(options.LegacyInstallerPath)
	legacyDistBytes, legacyDistFiles := directoryBytes(options.LegacyDistPath)
	result := PackageComparison{
		ExeBytes:             exeBytes,
		ReleaseZipBytes:      releaseZipBytes,
		LegacyInstallerBytes: legacyInstallerBytes,
		LegacyDistBytes:      legacyDistBytes,
		LegacyDistFiles:      legacyDistFiles,
	}
	if releaseZipBytes > 0 && legacyInstallerBytes > 0 {
		result.PackageComparisonAvailable = true
		result.ReleaseZipReductionPct = reductionPct(legacyInstallerBytes, releaseZipBytes)
	}
	if releaseZipBytes > 0 && legacyDistBytes > 0 {
		result.ReleaseVsLegacyDistPct = reductionPct(legacyDistBytes, releaseZipBytes)
	}
	return result
}

func fileBytes(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

func directoryBytes(path string) (int64, int64) {
	if path == "" {
		return 0, 0
	}
	var bytes int64
	var files int64
	_ = filepath.WalkDir(path, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		bytes += info.Size()
		files++
		return nil
	})
	return bytes, files
}

func reductionPct(before int64, after int64) float64 {
	if before <= 0 || after < 0 {
		return 0
	}
	return (1 - float64(after)/float64(before)) * 100
}

func summarize(values []int64) MetricSummary {
	if len(values) == 0 {
		return MetricSummary{}
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total int64
	for _, value := range sorted {
		total += value
	}
	p95Index := (95*len(sorted) + 99) / 100
	if p95Index < 1 {
		p95Index = 1
	}
	if p95Index > len(sorted) {
		p95Index = len(sorted)
	}
	return MetricSummary{
		Count:   len(sorted),
		Min:     sorted[0],
		Max:     sorted[len(sorted)-1],
		Average: float64(total) / float64(len(sorted)),
		P95:     sorted[p95Index-1],
	}
}

func budgetVerdict(report Report) BudgetVerdict {
	verdict := BudgetVerdict{}
	if report.Startup.P95 > 0 {
		verdict.ColdStartWithinTarget = report.Startup.P95 <= report.Budgets.ColdStartTargetMs
		verdict.ColdStartWithinIdeal = report.Startup.P95 <= report.Budgets.ColdStartIdealMs
	} else {
		verdict.Warnings = append(verdict.Warnings, "未获得有效冷启动样本")
	}
	if report.Hotkey.P95 > 0 {
		verdict.HotkeyWithinTarget = report.Hotkey.P95 <= report.Budgets.HotkeyTargetMs
	} else if report.HotkeyRegistration.DuringBlocked {
		verdict.Warnings = append(verdict.Warnings, "Ariadne 已注册 Alt+Q，但未获得有效唤起耗时样本")
	} else {
		verdict.Warnings = append(verdict.Warnings, "未获得有效 Alt+Q 唤起样本")
	}
	if report.Package.ReleaseZipBytes > 0 && report.Package.LegacyInstallerBytes > 0 {
		verdict.PackageSmallerThanLegacy = report.Package.ReleaseZipBytes < report.Package.LegacyInstallerBytes
	} else {
		verdict.Warnings = append(verdict.Warnings, "缺少 legacy installer 或 Ariadne release zip，无法对比包体积")
	}
	for _, sample := range report.Samples {
		if sample.Error != "" {
			verdict.Warnings = append(verdict.Warnings, fmt.Sprintf("样本 %d 失败: %s", sample.Iteration, sample.Error))
		}
	}
	for _, sample := range report.HotkeySamples {
		if sample.Error != "" {
			verdict.Warnings = append(verdict.Warnings, fmt.Sprintf("Alt+Q 样本 %d 失败: %s", sample.Iteration, sample.Error))
		}
	}
	return verdict
}

func verificationNotes(report Report) []string {
	notes := []string{}
	if report.Startup.Count > 0 {
		notes = append(notes, fmt.Sprintf("cold_start_p95_ms=%d target_ms=%d", report.Startup.P95, report.Budgets.ColdStartTargetMs))
	}
	if report.Hotkey.Count > 0 {
		notes = append(notes, fmt.Sprintf("hotkey_p95_ms=%d target_ms=%d", report.Hotkey.P95, report.Budgets.HotkeyTargetMs))
	}
	if report.HotkeyRegistration.BeforeAvailable || report.HotkeyRegistration.DuringBlocked || report.HotkeyRegistration.DuringErrorCode != 0 {
		notes = append(notes, fmt.Sprintf("hotkey_registration_before_available=%t during_blocked=%t during_error_code=%d", report.HotkeyRegistration.BeforeAvailable, report.HotkeyRegistration.DuringBlocked, report.HotkeyRegistration.DuringErrorCode))
	}
	if report.Memory.Count > 0 {
		notes = append(notes, fmt.Sprintf("working_set_avg_bytes=%.0f", report.Memory.Average))
	}
	if report.Package.ReleaseZipBytes > 0 {
		notes = append(notes, fmt.Sprintf("release_zip_bytes=%d", report.Package.ReleaseZipBytes))
	}
	if report.Package.LegacyInstallerBytes > 0 {
		notes = append(notes, fmt.Sprintf("legacy_installer_bytes=%d reduction_pct=%.2f", report.Package.LegacyInstallerBytes, report.Package.ReleaseZipReductionPct))
	}
	return notes
}
