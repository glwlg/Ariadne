package search

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"ariadne/internal/contracts"
)

const searchPerformanceSampleLimit = 200
const searchPerformanceTargetP95Ms int64 = 100

type Service struct {
	results     []contracts.SearchResult
	providers   []ResultProvider
	state       *StateStore
	performance searchPerformanceRecorder
}

type ResultProvider interface {
	Search(query string) []contracts.SearchResult
}

type ContextResultProvider interface {
	SearchContext(ctx context.Context, query string) []contracts.SearchResult
}

type PerformanceStatus struct {
	SampleCount     int    `json:"sampleCount"`
	TargetP95Ms     int64  `json:"targetP95Ms"`
	LastQuery       string `json:"lastQuery,omitempty"`
	LastElapsedMs   int64  `json:"lastElapsedMs"`
	LastResultCount int    `json:"lastResultCount"`
	AverageMs       int64  `json:"averageMs"`
	P95Ms           int64  `json:"p95Ms"`
	MaxMs           int64  `json:"maxMs"`
	WithinTarget    bool   `json:"withinTarget"`
	LastUpdatedAt   int64  `json:"lastUpdatedAt,omitempty"`
}

type searchSample struct {
	query       string
	elapsedMs   int64
	resultCount int
	createdAt   int64
}

type searchPerformanceRecorder struct {
	mu      sync.Mutex
	samples []searchSample
}

func NewService(providers ...ResultProvider) *Service {
	return NewServiceWithState(NewStateStore(defaultStatePath()), providers...)
}

func NewServiceWithState(state *StateStore, providers ...ResultProvider) *Service {
	return &Service{results: seedResults(), providers: providers, state: state}
}

func (s *Service) Search(ctx context.Context, query string) contracts.SearchResponse {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return newSearchResponse(query, []contracts.SearchResult{}, started)
	}
	if ctx.Err() != nil {
		return newSearchResponse(query, []contracts.SearchResult{}, started)
	}

	results := make([]contracts.SearchResult, 0, len(s.results))

	for _, result := range s.results {
		if ctx.Err() != nil {
			return newSearchResponse(query, results, started)
		}
		if matches(result, normalized) {
			results = appendUnique(results, result)
		}
	}
	for _, provider := range s.providers {
		if ctx.Err() != nil {
			return newSearchResponse(query, results, started)
		}
		providerResults := searchProvider(ctx, provider, query)
		if ctx.Err() != nil {
			return newSearchResponse(query, results, started)
		}
		for _, result := range providerResults {
			if ctx.Err() != nil {
				return newSearchResponse(query, results, started)
			}
			results = appendUnique(results, result)
		}
	}
	results = s.applyUsageState(results)
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	elapsed := time.Since(started).Milliseconds()
	s.recordPerformance(query, elapsed, len(results))
	return contracts.SearchResponse{
		Query:   query,
		Results: results,
		Elapsed: elapsed,
	}
}

func searchProvider(ctx context.Context, provider ResultProvider, query string) []contracts.SearchResult {
	if ctx.Err() != nil {
		return nil
	}
	if contextualProvider, ok := provider.(ContextResultProvider); ok {
		return contextualProvider.SearchContext(ctx, query)
	}
	return provider.Search(query)
}

func newSearchResponse(query string, results []contracts.SearchResult, started time.Time) contracts.SearchResponse {
	if results == nil {
		results = []contracts.SearchResult{}
	}
	return contracts.SearchResponse{
		Query:   query,
		Results: results,
		Elapsed: time.Since(started).Milliseconds(),
	}
}

func (s *Service) PerformanceStatus() PerformanceStatus {
	return s.performance.status()
}

func (s *Service) RecordUse(resultID string) UsageRecord {
	if s.state == nil {
		return UsageRecord{}
	}
	return s.state.RecordUse(resultID)
}

func (s *Service) SetFavorite(resultID string, favorite bool) UsageRecord {
	if s.state == nil {
		return UsageRecord{}
	}
	return s.state.SetFavorite(resultID, favorite)
}

func (s *Service) UsageStatus() UsageStatus {
	if s.state == nil {
		return UsageStatus{}
	}
	return s.state.Status()
}

func (s *Service) ClearUsage() ClearUsageResult {
	if s.state == nil {
		return ClearUsageResult{OK: true, Message: "没有可清理的搜索收藏或最近使用记录"}
	}
	return s.state.Clear()
}

func (s *Service) recordPerformance(query string, elapsedMs int64, resultCount int) {
	s.performance.record(searchSample{
		query:       strings.TrimSpace(query),
		elapsedMs:   elapsedMs,
		resultCount: resultCount,
		createdAt:   time.Now().Unix(),
	})
}

func (r *searchPerformanceRecorder) record(sample searchSample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.samples = append(r.samples, sample)
	if overflow := len(r.samples) - searchPerformanceSampleLimit; overflow > 0 {
		r.samples = append([]searchSample(nil), r.samples[overflow:]...)
	}
}

func (r *searchPerformanceRecorder) status() PerformanceStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.samples) == 0 {
		return PerformanceStatus{TargetP95Ms: searchPerformanceTargetP95Ms, WithinTarget: true}
	}
	values := make([]int64, 0, len(r.samples))
	var total int64
	var max int64
	for _, sample := range r.samples {
		values = append(values, sample.elapsedMs)
		total += sample.elapsedMs
		if sample.elapsedMs > max {
			max = sample.elapsedMs
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	last := r.samples[len(r.samples)-1]
	p95Index := (95*len(values) + 99) / 100
	if p95Index < 1 {
		p95Index = 1
	}
	p95 := values[p95Index-1]
	return PerformanceStatus{
		SampleCount:     len(r.samples),
		TargetP95Ms:     searchPerformanceTargetP95Ms,
		LastQuery:       last.query,
		LastElapsedMs:   last.elapsedMs,
		LastResultCount: last.resultCount,
		AverageMs:       total / int64(len(r.samples)),
		P95Ms:           p95,
		MaxMs:           max,
		WithinTarget:    p95 <= searchPerformanceTargetP95Ms,
		LastUpdatedAt:   last.createdAt,
	}
}

func appendUnique(results []contracts.SearchResult, next contracts.SearchResult) []contracts.SearchResult {
	for _, result := range results {
		if result.ID == next.ID {
			return results
		}
	}
	return append(results, next)
}

func matches(result contracts.SearchResult, query string) bool {
	parts := []string{
		string(result.Type),
		result.Title,
		result.Subtitle,
		result.Detail,
		result.Preview.Title,
		result.Preview.Subtitle,
		result.Preview.Text,
	}
	parts = append(parts, result.Tags...)

	for _, item := range result.Preview.Meta {
		parts = append(parts, item.Label, item.Value)
	}
	for _, item := range result.Preview.Evidence {
		parts = append(parts, item.Label, item.Value)
	}

	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), query)
}

func (s *Service) applyUsageState(results []contracts.SearchResult) []contracts.SearchResult {
	if s.state == nil {
		return results
	}
	now := time.Now()
	for i := range results {
		record := s.state.Get(results[i].ID)
		results[i].Score += usageBoost(record, now)
		results[i].Actions = appendFavoriteAction(results[i].Actions, results[i].ID, record.Favorite)
		if record.Favorite {
			results[i].Tags = appendTag(results[i].Tags, "收藏")
		}
		if record.UseCount > 0 {
			results[i].Tags = appendTag(results[i].Tags, "最近使用")
		}
	}
	return results
}

func appendFavoriteAction(actions []contracts.PreviewAction, resultID string, favorite bool) []contracts.PreviewAction {
	for _, action := range actions {
		if action.ID == "favorite" || action.ID == "unfavorite" {
			return actions
		}
	}
	label := "收藏"
	id := "favorite"
	success := "已收藏"
	if favorite {
		label = "取消收藏"
		id = "unfavorite"
		success = "已取消收藏"
	}
	return append(actions, contracts.PreviewAction{
		ID:    id,
		Label: label,
		Icon:  "pin",
		Kind:  contracts.ActionPin,
		Payload: map[string]interface{}{
			"targetId": resultID,
			"favorite": !favorite,
		},
		Feedback: &contracts.ActionFeedback{SuccessLabel: success, DurationMS: 1400},
	})
}

func appendTag(tags []string, next string) []string {
	for _, tag := range tags {
		if tag == next {
			return tags
		}
	}
	return append(tags, next)
}

func seedResults() []contracts.SearchResult {
	return []contracts.SearchResult{
		{
			ID:       "file-readme",
			Type:     contracts.ResultFile,
			Title:    "README.md",
			Subtitle: "P:\\workspace\\glwlg\\app\\x-tools",
			Detail:   "项目说明、插件列表、截图贴图和跨平台演进策略。",
			Icon:     "file",
			Tags:     []string{"Markdown", "文件"},
			Preview: contracts.PreviewDescriptor{
				Kind:     contracts.PreviewText,
				Title:    "README.md",
				Subtitle: "本地文件",
				Text:     "x-tools 是一款专为 Windows 打造的本地搜索与效率工具。Ariadne 将继承搜索、截图、插件和工作记忆能力。",
				Meta: []contracts.LabelValue{
					{Label: "路径", Value: "P:\\workspace\\glwlg\\app\\x-tools\\README.md"},
					{Label: "动作来源", Value: "文件结果默认动作"},
				},
			},
			Actions: []contracts.PreviewAction{
				{ID: "open", Label: "打开文件", Icon: "open", Kind: contracts.ActionOpen, Shortcut: "Enter", Payload: map[string]interface{}{"path": "P:\\workspace\\glwlg\\app\\x-tools\\README.md"}},
				{ID: "open_parent", Label: "打开所在文件夹", Icon: "folder", Kind: contracts.ActionOpenParent, Payload: map[string]interface{}{"path": "P:\\workspace\\glwlg\\app\\x-tools\\README.md"}},
				contracts.CopyAction("copy_path", "复制路径", "P:\\workspace\\glwlg\\app\\x-tools\\README.md", ""),
				{ID: "remember", Label: "加入记忆", Icon: "remember", Kind: contracts.ActionRemember},
			},
		},
		{
			ID:       "plugin-uuid",
			Type:     contracts.ResultPluginTrigger,
			Title:    "UUID 生成器",
			Subtitle: "插件 · uuid / guid",
			Detail:   "生成随机 UUID，结果只提供复制动作，不显示文件动作。",
			Icon:     "plugin",
			Tags:     []string{"插件", "复制结果"},
			Payload: map[string]interface{}{
				"pluginId": "uuid",
				"keyword":  "uuid",
				"commandSchema": map[string]interface{}{
					"usage":    "uuid [count]",
					"examples": []string{"uuid", "uuid 10", "guid 3"},
					"params": []map[string]interface{}{
						{"name": "count", "label": "数量", "placeholder": "默认 5，最大 50", "required": false},
					},
				},
			},
			Preview: contracts.PreviewDescriptor{
				Kind:     contracts.PreviewText,
				Title:    "UUID 生成器",
				Subtitle: "uuid [count]",
				Text:     "输入 uuid 或 guid 后生成 UUID。插件结果通过 preview actions 明确声明复制动作。",
				Meta: []contracts.LabelValue{
					{Label: "示例", Value: "uuid 5"},
					{Label: "协议", Value: "plugin_trigger -> plugin_result"},
				},
			},
			Actions: []contracts.PreviewAction{
				contracts.RunAction("prepare_command", "补全命令", "uuid", "Enter"),
				contracts.CopyAction("copy_command", "复制用法", "uuid [count]", ""),
			},
		},
		{
			ID:       "clipboard-json",
			Type:     contracts.ResultClipboard,
			Title:    "复制过的 JSON 片段",
			Subtitle: "剪贴板 · 12 分钟前",
			Detail:   `{"service":"gateway","status":"degraded","region":"hk"}`,
			Icon:     "clipboard",
			Tags:     []string{"剪贴板", "JSON"},
			Preview: contracts.PreviewDescriptor{
				Kind:     contracts.PreviewText,
				Title:    "复制过的 JSON 片段",
				Subtitle: "剪贴板历史",
				Text:     "{\n  \"service\": \"gateway\",\n  \"status\": \"degraded\",\n  \"region\": \"hk\"\n}",
				Meta: []contracts.LabelValue{
					{Label: "内容类型", Value: "JSON"},
					{Label: "可纳入", Value: "工作记忆、日报、问题复盘"},
				},
			},
			Actions: []contracts.PreviewAction{
				contracts.CopyAction("copy_value", "复制内容", `{"service":"gateway","status":"degraded","region":"hk"}`, ""),
				contracts.PluginAction("format_json", "JSON 格式化", "json"),
				contracts.RememberAction("remember", "加入记忆", "clipboard-json"),
			},
		},
		{
			ID:       "workflow-daily",
			Type:     contracts.ResultWorkflow,
			Title:    "生成今日工作日报",
			Subtitle: "工作流 · mem daily",
			Detail:   "汇总今天的截图、剪贴板、手动笔记和工作记忆。",
			Icon:     "workflow",
			Tags:     []string{"工作流", "日报"},
			Preview: contracts.PreviewDescriptor{
				Kind:     contracts.PreviewWorkflow,
				Title:    "生成今日工作日报",
				Subtitle: "基于本地工作记忆",
				Text:     "这个工作流会检索今天的工作记忆，生成日报草稿，并保留引用来源。",
				Meta: []contracts.LabelValue{
					{Label: "输入", Value: "今天的工作记忆"},
					{Label: "输出", Value: "日报草稿 + 证据引用"},
				},
			},
			Actions: []contracts.PreviewAction{
				contracts.RunAction("run_workflow", "运行工作流", "mem daily", "Enter"),
				contracts.PluginAction("open_editor", "编辑步骤", "workflow:daily"),
			},
		},
		{
			ID:       "settings-center",
			Type:     contracts.ResultSettings,
			Title:    "设置中心",
			Subtitle: "配置 / 主题 / 快捷键",
			Detail:   "打开设置中心，管理浅色/深色模式、快捷键、旧版配置迁移、启动项和工作记忆策略。",
			Icon:     "settings",
			Score:    120,
			Tags:     []string{"设置", "配置", "偏好", "theme", "hotkey", "settings", "config"},
			Preview: contracts.PreviewDescriptor{
				Kind:     contracts.PreviewSettings,
				Title:    "设置中心",
				Subtitle: "配置入口",
				Text:     "从启动器打开设置中心；默认浅色，深色仅作为显式模式保留。",
				Meta: []contracts.LabelValue{
					{Label: "入口", Value: "Alt+Q -> 设置"},
					{Label: "主题", Value: "light / dark / system"},
				},
			},
			Actions: []contracts.PreviewAction{
				{
					ID:       "open_tool",
					Label:    "打开设置",
					Icon:     "settings",
					Kind:     contracts.ActionPlugin,
					Shortcut: "Enter",
					Payload: map[string]interface{}{
						"command": "open_settings",
					},
					Feedback: &contracts.ActionFeedback{SuccessLabel: "设置已打开", DurationMS: 1200},
				},
			},
		},
	}
}
