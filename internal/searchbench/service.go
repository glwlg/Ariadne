package searchbench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ariadne/internal/apps"
	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/contracts"
	"ariadne/internal/filesearch"
	"ariadne/internal/imageindex"
	"ariadne/internal/launchers"
	"ariadne/internal/ocr"
	"ariadne/internal/plugins"
	"ariadne/internal/search"
	"ariadne/internal/workflows"
	"ariadne/internal/workmemory"
)

const defaultSearchTargetP95Ms int64 = 100

type Runner interface {
	Search(query string) contracts.SearchResponse
	PerformanceStatus() search.PerformanceStatus
}

type ProviderReporter interface {
	FileSearchStatus() filesearch.EverythingStatus
}

type DefaultStack struct {
	SearchService      *search.Service
	FileSearchService  *filesearch.Service
	ClipboardService   *clipboardhistory.Service
	WorkMemoryService  *workmemory.Service
	CaptureService     *capturehistory.Service
	ImageIndexService  *imageindex.Service
	WorkflowService    *workflows.Service
	PluginService      *plugins.Service
	LauncherService    *launchers.Service
	ApplicationService *apps.Service
}

func Run(options Options) Report {
	options = normalizeOptions(options)
	restore, tempRoot, envWarning := applyBenchmarkEnvironment(options.UseTempAppData)
	defer restore()

	stack := NewDefaultStack()
	defer stack.Close()

	report := RunWithRunner(stack, options)
	report.TempAppDataRoot = tempRoot
	if envWarning != "" {
		report.Verdict.Warnings = append(report.Verdict.Warnings, envWarning)
		report.VerificationNotes = append(report.VerificationNotes, "temp_appdata_warning="+envWarning)
	}
	return report
}

func RunWithRunner(runner Runner, options Options) Report {
	options = normalizeOptions(options)
	report := Report{
		ProductName:  "Ariadne",
		CreatedAt:    time.Now().Unix(),
		Options:      options,
		QueryCount:   len(options.Queries),
		TotalSamples: options.Iterations * len(options.Queries),
	}
	if runner == nil {
		report.Verdict = Verdict{Warnings: []string{"搜索 benchmark runner 为空"}}
		report.VerificationNotes = verificationNotes(report)
		return report
	}

	for i := 0; i < options.Warmup; i++ {
		for _, query := range options.Queries {
			runner.Search(query)
		}
	}

	for iteration := 1; iteration <= options.Iterations; iteration++ {
		for _, query := range options.Queries {
			response := runner.Search(query)
			sample := sampleFromResponse(len(report.Samples)+1, iteration, query, response)
			report.Samples = append(report.Samples, sample)
			report.ActionValidation.CheckedSamples++
			report.ActionValidation.CheckedResults += len(response.Results)
			if sample.ActionValidationError != "" {
				report.ActionValidation.InvalidSamples++
				report.ActionValidation.LastError = sample.ActionValidationError
			}
		}
	}

	report.ActionValidation.OK = report.ActionValidation.InvalidSamples == 0
	report.Summary = summarizeSamples(report.Samples)
	report.QuerySummaries = summarizeQueries(report.Samples, options.Queries)
	report.SlowestSamples = slowestSamples(report.Samples, options.SlowestLimit)
	report.RollingPerformance = runner.PerformanceStatus()
	report.ProviderStatus = providerStatus(runner, report.Samples)
	report.Verdict = verdict(report)
	report.VerificationNotes = verificationNotes(report)
	return report
}

func NewDefaultStack() *DefaultStack {
	captureService := capturehistory.NewService()
	clipboardService := clipboardhistory.NewService(captureService)
	fileSearchService := filesearch.NewService()
	appService := apps.NewService()
	launcherService := launchers.NewService()
	pluginService := plugins.NewService()
	workflowService := workflows.NewService(pluginService)
	workMemoryService := workmemory.NewService(captureService)
	ocrService := ocr.NewService(captureService, clipboardService, workMemoryService)
	imageIndexService := imageindex.NewService(captureService, clipboardService, ocrService)
	searchService := search.NewService(
		fileSearchService,
		appService,
		launcherService,
		clipboardService,
		captureService,
		imageIndexService,
		workflowService,
		pluginService,
		workMemoryService,
	)
	return &DefaultStack{
		SearchService:      searchService,
		FileSearchService:  fileSearchService,
		ClipboardService:   clipboardService,
		WorkMemoryService:  workMemoryService,
		CaptureService:     captureService,
		ImageIndexService:  imageIndexService,
		WorkflowService:    workflowService,
		PluginService:      pluginService,
		LauncherService:    launcherService,
		ApplicationService: appService,
	}
}

func (s *DefaultStack) Search(query string) contracts.SearchResponse {
	if s == nil || s.SearchService == nil {
		return contracts.SearchResponse{Query: query}
	}
	return s.SearchService.Search(context.Background(), query)
}

func (s *DefaultStack) PerformanceStatus() search.PerformanceStatus {
	if s == nil || s.SearchService == nil {
		return search.PerformanceStatus{TargetP95Ms: defaultSearchTargetP95Ms, WithinTarget: true}
	}
	return s.SearchService.PerformanceStatus()
}

func (s *DefaultStack) FileSearchStatus() filesearch.EverythingStatus {
	if s == nil || s.FileSearchService == nil {
		return filesearch.EverythingStatus{}
	}
	return s.FileSearchService.Status()
}

func (s *DefaultStack) Close() {
	if s == nil {
		return
	}
	if s.ClipboardService != nil {
		s.ClipboardService.StopWatcher()
	}
	if s.WorkMemoryService != nil {
		s.WorkMemoryService.Stop()
	}
}

func DefaultQueries() []string {
	return []string{
		"settings",
		"设置",
		"net",
		"jsondiff",
		"uuid 2",
		"hash ariadne",
		"base64 hello",
		"calc 12*(8+3)",
		"hosts",
		"mem daily",
		"workflow",
		"cap",
		"clipboard",
		"gateway",
		"README.md",
		"Everything64.dll",
	}
}

func normalizeOptions(options Options) Options {
	if options.Iterations <= 0 {
		options.Iterations = 20
	}
	if options.Warmup < 0 {
		options.Warmup = 0
	}
	if options.TargetP95Ms <= 0 {
		options.TargetP95Ms = defaultSearchTargetP95Ms
	}
	if options.SlowestLimit <= 0 {
		options.SlowestLimit = 10
	}
	options.Queries = normalizeQueries(options.Queries)
	if len(options.Queries) == 0 {
		options.Queries = DefaultQueries()
	}
	return options
}

func normalizeQueries(queries []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, query)
	}
	return result
}

func sampleFromResponse(index int, iteration int, query string, response contracts.SearchResponse) Sample {
	elapsedMs := response.Elapsed
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	sample := Sample{
		Index:       index,
		Iteration:   iteration,
		Query:       query,
		ElapsedMs:   elapsedMs,
		ResultCount: len(response.Results),
	}
	if len(response.Results) > 0 {
		top := response.Results[0]
		sample.TopResultID = top.ID
		sample.TopResultTitle = top.Title
		sample.TopResultType = string(top.Type)
	}
	for _, result := range response.Results {
		if result.Type == contracts.ResultFile {
			sample.FileResultCount++
		}
		if isEverythingFile(result) {
			sample.EverythingFileResults++
		}
	}
	if err := contracts.ValidateActionSurfaces(response.Results); err != nil {
		sample.ActionValidationError = err.Error()
	}
	return sample
}

func isEverythingFile(result contracts.SearchResult) bool {
	if result.Type != contracts.ResultFile {
		return false
	}
	for _, tag := range result.Tags {
		if strings.EqualFold(strings.TrimSpace(tag), "Everything") {
			return true
		}
	}
	if source, ok := result.Payload["source"].(string); ok && strings.Contains(strings.ToLower(source), "everything") {
		return true
	}
	return false
}

func summarizeSamples(samples []Sample) MetricSummary {
	values := make([]int64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.ElapsedMs)
	}
	return summarize(values)
}

func summarizeQueries(samples []Sample, queryOrder []string) []QuerySummary {
	byQuery := map[string][]Sample{}
	for _, sample := range samples {
		byQuery[sample.Query] = append(byQuery[sample.Query], sample)
	}
	summaries := make([]QuerySummary, 0, len(byQuery))
	seen := map[string]bool{}
	for _, query := range queryOrder {
		if samplesForQuery, ok := byQuery[query]; ok {
			summaries = append(summaries, summarizeQuery(query, samplesForQuery))
			seen[query] = true
		}
	}
	extraQueries := make([]string, 0, len(byQuery))
	for query := range byQuery {
		if !seen[query] {
			extraQueries = append(extraQueries, query)
		}
	}
	sort.Strings(extraQueries)
	for _, query := range extraQueries {
		summaries = append(summaries, summarizeQuery(query, byQuery[query]))
	}
	return summaries
}

func summarizeQuery(query string, samples []Sample) QuerySummary {
	values := make([]int64, 0, len(samples))
	summary := QuerySummary{Query: query, Count: len(samples)}
	var totalResults int
	for _, sample := range samples {
		values = append(values, sample.ElapsedMs)
		totalResults += sample.ResultCount
		if sample.ResultCount > summary.MaxResultCount {
			summary.MaxResultCount = sample.ResultCount
		}
		if sample.ResultCount == 0 {
			summary.ZeroResultCount++
		}
		if sample.EverythingFileResults > 0 {
			summary.EverythingFileSamples++
			summary.EverythingFileResults += sample.EverythingFileResults
		}
		if sample.ActionValidationError != "" {
			summary.ActionValidationErrors++
		}
	}
	metric := summarize(values)
	summary.MinMs = metric.Min
	summary.MaxMs = metric.Max
	summary.AverageMs = metric.Average
	summary.P95Ms = metric.P95
	if len(samples) > 0 {
		summary.AverageResultCount = float64(totalResults) / float64(len(samples))
	}
	return summary
}

func slowestSamples(samples []Sample, limit int) []Sample {
	if limit <= 0 || len(samples) == 0 {
		return nil
	}
	sorted := append([]Sample(nil), samples...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].ElapsedMs == sorted[j].ElapsedMs {
			return sorted[i].Index < sorted[j].Index
		}
		return sorted[i].ElapsedMs > sorted[j].ElapsedMs
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
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

func providerStatus(runner Runner, samples []Sample) ProviderStatus {
	status := ProviderStatus{}
	if reporter, ok := runner.(ProviderReporter); ok {
		status.EverythingStatusAvailable = true
		status.Everything = reporter.FileSearchStatus()
	}
	hitQueries := []string{}
	seen := map[string]bool{}
	for _, sample := range samples {
		if sample.EverythingFileResults <= 0 {
			continue
		}
		status.EverythingFileHits += sample.EverythingFileResults
		key := strings.ToLower(sample.Query)
		if !seen[key] {
			seen[key] = true
			hitQueries = append(hitQueries, sample.Query)
		}
	}
	status.EverythingHitQueries = hitQueries
	return status
}

func verdict(report Report) Verdict {
	result := Verdict{WithinTarget: report.Summary.Count > 0 && report.Summary.P95 <= report.Options.TargetP95Ms}
	if report.Summary.Count == 0 {
		result.Warnings = append(result.Warnings, "未获得搜索样本")
	} else if report.Summary.P95 > report.Options.TargetP95Ms {
		result.Warnings = append(result.Warnings, fmt.Sprintf("搜索 p95 %dms 超过目标 %dms", report.Summary.P95, report.Options.TargetP95Ms))
	}
	if !report.ActionValidation.OK {
		result.Warnings = append(result.Warnings, fmt.Sprintf("搜索结果动作协议校验失败 %d 个样本，最近错误: %s", report.ActionValidation.InvalidSamples, report.ActionValidation.LastError))
	}
	if report.ProviderStatus.EverythingStatusAvailable {
		if report.ProviderStatus.Everything.DLLFound && report.ProviderStatus.Everything.LastError != "" {
			result.Warnings = append(result.Warnings, "Everything 查询最近错误: "+report.ProviderStatus.Everything.LastError)
		}
		if !report.ProviderStatus.Everything.DLLFound {
			result.Warnings = append(result.Warnings, "未找到 Everything SDK DLL，文件结果真实命中无法验收")
		} else if report.ProviderStatus.EverythingFileHits == 0 {
			result.Warnings = append(result.Warnings, "未获得 Everything 文件结果命中样本")
		}
	}
	return result
}

func verificationNotes(report Report) []string {
	notes := []string{
		fmt.Sprintf("search_samples=%d queries=%d p95_ms=%d target_ms=%d within_target=%t", report.Summary.Count, report.QueryCount, report.Summary.P95, report.Options.TargetP95Ms, report.Verdict.WithinTarget),
		fmt.Sprintf("search_action_validation_ok=%t checked_results=%d invalid_samples=%d", report.ActionValidation.OK, report.ActionValidation.CheckedResults, report.ActionValidation.InvalidSamples),
		fmt.Sprintf("everything_status_available=%t dll_found=%t ready=%t last_query=%q last_results=%d file_hits=%d", report.ProviderStatus.EverythingStatusAvailable, report.ProviderStatus.Everything.DLLFound, report.ProviderStatus.Everything.Ready, report.ProviderStatus.Everything.LastQuery, report.ProviderStatus.Everything.LastResultCount, report.ProviderStatus.EverythingFileHits),
	}
	if report.RollingPerformance.SampleCount > 0 {
		notes = append(notes, fmt.Sprintf("rolling_search_samples=%d p95_ms=%d avg_ms=%d max_ms=%d", report.RollingPerformance.SampleCount, report.RollingPerformance.P95Ms, report.RollingPerformance.AverageMs, report.RollingPerformance.MaxMs))
	}
	return notes
}

func applyBenchmarkEnvironment(useTemp bool) (func(), string, string) {
	if !useTemp {
		return func() {}, "", ""
	}
	root, err := os.MkdirTemp("", "ariadne-searchbench-*")
	if err != nil {
		return func() {}, "", err.Error()
	}
	appData := filepath.Join(root, "AppData", "Roaming")
	localAppData := filepath.Join(root, "AppData", "Local")
	if err := os.MkdirAll(appData, 0o755); err != nil {
		return func() { _ = os.RemoveAll(root) }, root, err.Error()
	}
	if err := os.MkdirAll(localAppData, 0o755); err != nil {
		return func() { _ = os.RemoveAll(root) }, root, err.Error()
	}

	appDataValue, appDataSet := os.LookupEnv("APPDATA")
	localAppDataValue, localAppDataSet := os.LookupEnv("LOCALAPPDATA")
	_ = os.Setenv("APPDATA", appData)
	_ = os.Setenv("LOCALAPPDATA", localAppData)
	return func() {
		restoreEnv("APPDATA", appDataValue, appDataSet)
		restoreEnv("LOCALAPPDATA", localAppDataValue, localAppDataSet)
		_ = os.RemoveAll(root)
	}, root, ""
}

func restoreEnv(key string, value string, wasSet bool) {
	if wasSet {
		_ = os.Setenv(key, value)
		return
	}
	_ = os.Unsetenv(key)
}
