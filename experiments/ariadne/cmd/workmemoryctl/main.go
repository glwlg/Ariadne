package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ariadne/internal/aiclient"
	"ariadne/internal/settings"
	"ariadne/internal/workmemory"
)

type output struct {
	ConfigPath string                    `json:"configPath"`
	Action     string                    `json:"action"`
	Status     workmemory.SemanticStatus `json:"status"`
	Refresh    any                       `json:"refresh,omitempty"`
	Search     any                       `json:"search,omitempty"`
	Note       *noteSummary              `json:"note,omitempty"`
}

type noteSummary struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	Favorite  bool     `json:"favorite"`
	Sensitive bool     `json:"sensitive"`
	CreatedAt int64    `json:"createdAt"`
}

func main() {
	action := flag.String("action", "status", "status, refresh, or search")
	query := flag.String("query", "", "semantic search query when action=search")
	title := flag.String("title", "", "manual note title when action=add-note")
	text := flag.String("text", "", "manual note text when action=add-note")
	tags := flag.String("tags", "", "comma-separated manual note tags when action=add-note")
	favorite := flag.Bool("favorite", false, "mark manual note favorite when action=add-note")
	sensitive := flag.Bool("sensitive", false, "mark manual note sensitive when action=add-note")
	vectorStore := flag.String("vector-store", "", "override vector store type, e.g. embedded or milvus")
	vectorURI := flag.String("vector-uri", "", "override vector store URI")
	collection := flag.String("collection", "", "override vector collection")
	flag.Parse()

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
	if strings.TrimSpace(*vectorStore) != "" {
		policy.VectorStoreType = strings.TrimSpace(*vectorStore)
	}
	if strings.TrimSpace(*vectorURI) != "" {
		policy.VectorStoreURI = strings.TrimSpace(*vectorURI)
	}
	if strings.TrimSpace(*collection) != "" {
		policy.VectorCollection = strings.TrimSpace(*collection)
	}

	result := output{
		ConfigPath: defaultConfigPath(),
		Action:     strings.TrimSpace(strings.ToLower(*action)),
		Status:     service.ApplyEmbeddingPolicy(policy),
	}

	switch result.Action {
	case "status":
		result.Status = service.SemanticStatus()
	case "refresh":
		refresh := service.RefreshEmbeddingIndex()
		result.Refresh = refresh
		result.Status = refresh.Status
		if !refresh.OK {
			writeJSON(result)
			os.Exit(1)
		}
	case "search":
		search := service.SemanticSearchExternal(*query)
		result.Search = search
		result.Status = search.Status
		if !search.OK {
			writeJSON(result)
			os.Exit(1)
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
			result.Status = service.SemanticStatus()
			writeJSON(result)
			os.Exit(1)
		}
		result.Note = &noteSummary{
			ID:        entry.ID,
			Title:     entry.Title,
			Tags:      entry.Tags,
			Favorite:  entry.Favorite,
			Sensitive: entry.Sensitive,
			CreatedAt: entry.CreatedAt,
		}
		result.Status = service.SemanticStatus()
	default:
		fmt.Fprintf(os.Stderr, "unsupported action %q\n", *action)
		os.Exit(2)
	}

	writeJSON(result)
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
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

func defaultConfigPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "config.json")
}
