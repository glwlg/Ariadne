package workmemorycli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"ariadne/internal/aiclient"
	"ariadne/internal/contracts"
	"ariadne/internal/settings"
	"ariadne/internal/workmemory"
)

type Output struct {
	OK          bool                      `json:"ok"`
	Action      string                    `json:"action"`
	Message     string                    `json:"message,omitempty"`
	Status      workmemory.SemanticStatus `json:"status"`
	Memory      workmemory.Status         `json:"memory,omitempty"`
	Refresh     any                       `json:"refresh,omitempty"`
	Results     []MemorySummary           `json:"results,omitempty"`
	Entry       *MemoryDetail             `json:"entry,omitempty"`
	Note        *NoteSummary              `json:"note,omitempty"`
	StoragePath string                    `json:"storagePath,omitempty"`
}

type MemorySummary struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary,omitempty"`
	Source       string   `json:"source"`
	AppName      string   `json:"appName,omitempty"`
	WindowTitle  string   `json:"windowTitle,omitempty"`
	ContentType  string   `json:"contentType,omitempty"`
	CreatedAt    int64    `json:"createdAt"`
	Score        float64  `json:"score,omitempty"`
	HasImage     bool     `json:"hasImage"`
	Tags         []string `json:"tags,omitempty"`
	OCRStatus    string   `json:"ocrStatus,omitempty"`
	Quality      string   `json:"quality,omitempty"`
	Preview      string   `json:"preview,omitempty"`
	Match        string   `json:"match,omitempty"`
	EvidenceHint string   `json:"evidenceHint,omitempty"`
	Guidance     []string `json:"guidance,omitempty"`
}

type MemoryDetail struct {
	MemorySummary
	Text      string                    `json:"text,omitempty"`
	OCRText   string                    `json:"ocrText,omitempty"`
	ImagePath string                    `json:"imagePath,omitempty"`
	Frames    []workmemory.CaptureFrame `json:"frames,omitempty"`
}

type NoteSummary struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	Favorite  bool     `json:"favorite"`
	Sensitive bool     `json:"sensitive"`
	CreatedAt int64    `json:"createdAt"`
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	defaultAction := "status"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		defaultAction = strings.TrimSpace(strings.ToLower(args[0]))
		args = args[1:]
	}
	fs := flag.NewFlagSet("workmemory", flag.ContinueOnError)
	fs.SetOutput(stderr)
	action := fs.String("action", defaultAction, "status, refresh, search, recent, get, or add-note")
	query := fs.String("query", "", "query text for action=search")
	id := fs.String("id", "", "memory entry id for action=get")
	title := fs.String("title", "", "manual note title when action=add-note")
	text := fs.String("text", "", "manual note text when action=add-note")
	tags := fs.String("tags", "", "comma-separated manual note tags when action=add-note")
	favorite := fs.Bool("favorite", false, "mark manual note favorite when action=add-note")
	sensitive := fs.Bool("sensitive", false, "mark manual note sensitive when action=add-note")
	limit := fs.Int("limit", 8, "maximum number of entries")
	since := fs.Int64("since", 0, "unix timestamp lower bound")
	sinceHours := fs.Int("since-hours", 0, "relative lower bound in hours")
	source := fs.String("source", "", "filter by source")
	app := fs.String("app", "", "filter by app/process name")
	vectorStore := fs.String("vector-store", "", "override vector store type, e.g. embedded or milvus")
	vectorURI := fs.String("vector-uri", "", "override vector store URI")
	collection := fs.String("collection", "", "override vector collection")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	service := newWorkMemoryService(*vectorStore, *vectorURI, *collection)
	defer service.Stop()
	cutoff := *since
	if *sinceHours > 0 {
		cutoff = time.Now().Add(-time.Duration(*sinceHours) * time.Hour).Unix()
	}
	filter := memoryFilter{Since: cutoff, Source: *source, App: *app}
	result := Output{
		OK:          true,
		Action:      strings.TrimSpace(strings.ToLower(*action)),
		Status:      service.SemanticStatus(),
		Memory:      service.Status(),
		StoragePath: service.Status().StoragePath,
	}

	switch result.Action {
	case "status":
		result.Message = "work memory cli ready"
	case "refresh":
		refresh := service.RefreshEmbeddingIndex()
		result.Refresh = refresh
		result.Status = refresh.Status
		result.OK = refresh.OK
		result.Message = refresh.Message
	case "search":
		result.Results, result.Message = searchMemories(service, *query, clampLimit(*limit), filter)
	case "recent", "timeline":
		result.Results = recentMemories(service, clampLimit(*limit), filter)
		result.Message = fmt.Sprintf("returned %d recent memories", len(result.Results))
	case "get":
		entry := service.Entry(*id)
		if entry.ID == "" || !entryAllowed(entry) {
			result.OK = false
			result.Message = "memory entry not found"
		} else {
			detail := detailFromEntry(entry, 6000)
			result.Entry = &detail
			result.Message = "memory entry loaded"
		}
	case "add-note":
		entry := service.AddNote(workmemory.NoteRequest{
			Title:     *title,
			Text:      *text,
			Tags:      splitCSV(*tags),
			Favorite:  *favorite,
			Sensitive: *sensitive,
		})
		if entry.ID == "" {
			result.OK = false
			result.Message = "manual note was not added"
		} else {
			result.Note = &NoteSummary{
				ID:        entry.ID,
				Title:     entry.Title,
				Tags:      entry.Tags,
				Favorite:  entry.Favorite,
				Sensitive: entry.Sensitive,
				CreatedAt: entry.CreatedAt,
			}
			result.Message = "manual note added"
		}
	default:
		_, _ = fmt.Fprintf(stderr, "unsupported action %q\n", result.Action)
		return 2
	}

	result.Status = service.SemanticStatus()
	result.Memory = service.Status()
	writeJSON(stdout, result)
	if !result.OK {
		return 1
	}
	return 0
}

func newWorkMemoryService(vectorStore string, vectorURI string, collection string) *workmemory.Service {
	settingsService := settings.NewService()
	appSettings := settingsService.GetSettings()
	service := workmemory.NewService()
	workmemory.RegisterEmbeddingClient(service, aiclient.NewOpenAICompatibleEmbedder())
	service.ApplySettings(
		appSettings.WorkMemory.Enabled,
		appSettings.WorkMemory.PrivacyMode,
		false,
		appSettings.WorkMemory.AutoCaptureIntervalSeconds,
	)
	policy := workmemory.EmbeddingPolicy{
		Enabled:          appSettings.AI.EmbeddingEnabled,
		Provider:         firstNonEmpty(appSettings.AI.EmbeddingProvider, "openai-compatible"),
		BaseURL:          firstNonEmpty(appSettings.AI.EmbeddingBaseURL, os.Getenv("EMBED__BASE_URL")),
		Model:            firstNonEmpty(appSettings.AI.EmbeddingModel, os.Getenv("EMBED__MODEL")),
		VectorStoreType:  appSettings.AI.VectorStoreType,
		VectorStoreURI:   appSettings.AI.VectorStoreURI,
		VectorCollection: appSettings.AI.VectorCollection,
	}
	if strings.TrimSpace(vectorStore) != "" {
		policy.VectorStoreType = strings.TrimSpace(vectorStore)
	}
	if strings.TrimSpace(vectorURI) != "" {
		policy.VectorStoreURI = strings.TrimSpace(vectorURI)
	}
	if strings.TrimSpace(collection) != "" {
		policy.VectorCollection = strings.TrimSpace(collection)
	}
	service.ApplyEmbeddingPolicy(policy)
	return service
}

type memoryFilter struct {
	Since  int64
	Source string
	App    string
}

func (f memoryFilter) Match(entry workmemory.Entry) bool {
	if f.Since > 0 && entry.CreatedAt < f.Since {
		return false
	}
	if f.Source != "" && !strings.EqualFold(entry.Source, f.Source) {
		return false
	}
	if f.App != "" && !strings.Contains(strings.ToLower(entry.AppName), strings.ToLower(f.App)) {
		return false
	}
	return true
}

func searchMemories(service *workmemory.Service, query string, limit int, filter memoryFilter) ([]MemorySummary, string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return recentMemories(service, limit, filter), "empty query, returned recent memories"
	}
	seen := map[string]bool{}
	results := []MemorySummary{}
	semantic := service.SemanticSearchExternal(query)
	if semantic.OK {
		for _, item := range semantic.Results {
			if entryID := searchResultEntryID(item); entryID != "" {
				entry := service.Entry(entryID)
				if entry.ID != "" && entryAllowed(entry) && filter.Match(entry) && !seen[entry.ID] {
					summary := summaryFromEntry(entry, item.Score, 900)
					summary.Match = previewEvidenceValue(item.Preview, "匹配")
					results = append(results, summary)
					seen[entry.ID] = true
				}
			}
			if len(results) >= limit {
				return results, semantic.Message
			}
		}
	}
	for _, item := range service.Search(query) {
		if entryID := searchResultEntryID(item); entryID != "" {
			entry := service.Entry(entryID)
			if entry.ID != "" && entryAllowed(entry) && filter.Match(entry) && !seen[entry.ID] {
				summary := summaryFromEntry(entry, item.Score, 900)
				summary.Match = previewEvidenceValue(item.Preview, "命中")
				if summary.Match == "" {
					summary.Match = item.Detail
				}
				results = append(results, summary)
				seen[entry.ID] = true
			}
		}
		if len(results) >= limit {
			break
		}
	}
	message := fmt.Sprintf("returned %d memories", len(results))
	if semantic.Message != "" && !semantic.OK {
		message += "; semantic search skipped: " + semantic.Message
	}
	return results, message
}

func recentMemories(service *workmemory.Service, limit int, filter memoryFilter) []MemorySummary {
	entries := service.Timeline()
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].CreatedAt > entries[j].CreatedAt
	})
	results := []MemorySummary{}
	for _, entry := range entries {
		if !filter.Match(entry) || !entryAllowed(entry) {
			continue
		}
		results = append(results, summaryFromEntry(entry, 0, 900))
		if len(results) >= limit {
			break
		}
	}
	return results
}

func entryAllowed(entry workmemory.Entry) bool {
	return entry.ID != "" && !entry.Sensitive && !strings.EqualFold(entry.QualityStatus, "pending")
}

func summaryFromEntry(entry workmemory.Entry, score float64, previewLimit int) MemorySummary {
	preview := firstNonEmpty(entry.Text, entry.OCRText, entry.Summary)
	return MemorySummary{
		ID:           entry.ID,
		Title:        firstNonEmpty(entry.Title, entry.Summary, entry.ID),
		Summary:      truncate(firstNonEmpty(entry.Summary, entry.Text, entry.OCRText), 320),
		Source:       entry.Source,
		AppName:      entry.AppName,
		WindowTitle:  entry.WindowTitle,
		ContentType:  entry.ContentType,
		CreatedAt:    entry.CreatedAt,
		Score:        score,
		HasImage:     entry.CaptureID != "" || entry.ImagePath != "" || len(entry.Frames) > 0,
		Tags:         append([]string(nil), entry.Tags...),
		OCRStatus:    entry.OCRStatus,
		Quality:      entry.QualityStatus,
		Preview:      truncate(preview, previewLimit),
		EvidenceHint: evidenceHint(entry),
		Guidance:     interpretationGuidance(entry),
	}
}

func detailFromEntry(entry workmemory.Entry, textLimit int) MemoryDetail {
	return MemoryDetail{
		MemorySummary: summaryFromEntry(entry, 0, 1200),
		Text:          truncate(entry.Text, textLimit),
		OCRText:       truncate(entry.OCRText, textLimit),
		ImagePath:     entry.ImagePath,
		Frames:        append([]workmemory.CaptureFrame(nil), entry.Frames...),
	}
}

func searchResultEntryID(item contracts.SearchResult) string {
	if value, ok := item.Payload["entryId"].(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(item.ID)
}

func previewEvidenceValue(preview contracts.PreviewDescriptor, label string) string {
	for _, item := range preview.Evidence {
		if item.Label == label {
			return item.Value
		}
	}
	return ""
}

func evidenceHint(entry workmemory.Entry) string {
	parts := []string{}
	if entry.CaptureID != "" || entry.ImagePath != "" || len(entry.Frames) > 0 {
		parts = append(parts, "image")
	}
	if strings.TrimSpace(entry.OCRText) != "" {
		parts = append(parts, "ocr")
	}
	if strings.TrimSpace(entry.Text) != "" {
		parts = append(parts, "text")
	}
	return strings.Join(parts, ",")
}

func interpretationGuidance(entry workmemory.Entry) []string {
	app := strings.ToLower(strings.TrimSpace(entry.AppName))
	title := strings.ToLower(strings.TrimSpace(entry.WindowTitle))
	tags := strings.ToLower(strings.Join(entry.Tags, " "))
	if !strings.Contains(app, "weixin") && !strings.Contains(title, "微信") && !strings.Contains(tags, "微信") {
		return nil
	}
	return []string{
		"微信截图可能同时包含当前会话、左侧会话列表、群聊预览、服务号和后台窗口。",
		"联系人类问题必须用 get 查看 OCR/正文明细后再判断；不要把左侧列表或背景窗口里出现的人名当成已聊天对象。",
		"右侧绿色气泡通常是当前用户自己的消息；不要把当前用户名字列为联系人，除非有独立证据说明对方也在发言。",
	}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 8
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func writeJSON(w io.Writer, value any) {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}

func ParseUnixOrHours(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "h") {
		hours, _ := strconv.Atoi(strings.TrimSuffix(value, "h"))
		if hours > 0 {
			return time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
		}
	}
	unix, _ := strconv.ParseInt(value, 10, 64)
	return unix
}
