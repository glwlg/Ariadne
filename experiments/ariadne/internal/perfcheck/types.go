package perfcheck

type Options struct {
	ExePath             string
	ReleaseZipPath      string
	LegacyInstallerPath string
	LegacyDistPath      string
	Iterations          int
	HotkeyIterations    int
	TimeoutMs           int64
	UseTempAppData      bool
}

type Budgets struct {
	ColdStartTargetMs int64 `json:"coldStartTargetMs"`
	ColdStartIdealMs  int64 `json:"coldStartIdealMs"`
	HotkeyTargetMs    int64 `json:"hotkeyTargetMs"`
}

type Report struct {
	ProductName        string                  `json:"productName"`
	CreatedAt          int64                   `json:"createdAt"`
	Options            Options                 `json:"options"`
	Budgets            Budgets                 `json:"budgets"`
	Package            PackageComparison       `json:"package"`
	HotkeyRegistration HotkeyRegistrationProbe `json:"hotkeyRegistration"`
	Startup            MetricSummary           `json:"startup"`
	Hotkey             MetricSummary           `json:"hotkey"`
	Memory             MetricSummary           `json:"memory"`
	Samples            []StartupSample         `json:"samples"`
	HotkeySamples      []HotkeySample          `json:"hotkeySamples"`
	BudgetVerdict      BudgetVerdict           `json:"budgetVerdict"`
	VerificationNotes  []string                `json:"verificationNotes"`
}

type PackageComparison struct {
	ExeBytes                   int64   `json:"exeBytes"`
	ReleaseZipBytes            int64   `json:"releaseZipBytes"`
	LegacyInstallerBytes       int64   `json:"legacyInstallerBytes,omitempty"`
	LegacyDistBytes            int64   `json:"legacyDistBytes,omitempty"`
	LegacyDistFiles            int64   `json:"legacyDistFiles,omitempty"`
	ReleaseZipReductionPct     float64 `json:"releaseZipReductionPct,omitempty"`
	ReleaseVsLegacyDistPct     float64 `json:"releaseVsLegacyDistPct,omitempty"`
	PackageComparisonAvailable bool    `json:"packageComparisonAvailable"`
}

type HotkeyRegistrationProbe struct {
	BeforeAvailable bool   `json:"beforeAvailable"`
	BeforeErrorCode int    `json:"beforeErrorCode,omitempty"`
	BeforeError     string `json:"beforeError,omitempty"`
	DuringBlocked   bool   `json:"duringBlocked"`
	DuringErrorCode int    `json:"duringErrorCode,omitempty"`
	DuringError     string `json:"duringError,omitempty"`
	Note            string `json:"note,omitempty"`
}

type MetricSummary struct {
	Count   int     `json:"count"`
	Min     int64   `json:"min"`
	Max     int64   `json:"max"`
	Average float64 `json:"average"`
	P95     int64   `json:"p95"`
}

type StartupSample struct {
	Iteration       int          `json:"iteration"`
	ProcessID       int          `json:"processId,omitempty"`
	StartupMs       int64        `json:"startupMs,omitempty"`
	WorkingSetBytes int64        `json:"workingSetBytes,omitempty"`
	Window          WindowSample `json:"window,omitempty"`
	Error           string       `json:"error,omitempty"`
}

type HotkeySample struct {
	Iteration int          `json:"iteration"`
	ProcessID int          `json:"processId,omitempty"`
	HotkeyMs  int64        `json:"hotkeyMs,omitempty"`
	Window    WindowSample `json:"window,omitempty"`
	Error     string       `json:"error,omitempty"`
}

type WindowSample struct {
	Handle        uint64 `json:"handle,omitempty"`
	Title         string `json:"title,omitempty"`
	Visible       bool   `json:"visible"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	StyleHex      string `json:"styleHex,omitempty"`
	ExStyleHex    string `json:"exStyleHex,omitempty"`
	HasCaption    bool   `json:"hasCaption"`
	HasThickFrame bool   `json:"hasThickFrame"`
	IsTopmost     bool   `json:"isTopmost"`
	IsForeground  bool   `json:"isForeground"`
}

type BudgetVerdict struct {
	ColdStartWithinTarget    bool     `json:"coldStartWithinTarget"`
	ColdStartWithinIdeal     bool     `json:"coldStartWithinIdeal"`
	HotkeyWithinTarget       bool     `json:"hotkeyWithinTarget"`
	PackageSmallerThanLegacy bool     `json:"packageSmallerThanLegacy"`
	Warnings                 []string `json:"warnings,omitempty"`
}

func DefaultBudgets() Budgets {
	return Budgets{
		ColdStartTargetMs: 800,
		ColdStartIdealMs:  500,
		HotkeyTargetMs:    120,
	}
}
