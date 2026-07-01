package searchbench

import (
	"ariadne/internal/filesearch"
	"ariadne/internal/search"
)

type Options struct {
	Iterations     int      `json:"iterations"`
	Warmup         int      `json:"warmup"`
	TargetP95Ms    int64    `json:"targetP95Ms"`
	SlowestLimit   int      `json:"slowestLimit"`
	Queries        []string `json:"queries"`
	UseTempAppData bool     `json:"useTempAppData"`
}

type Report struct {
	ProductName        string                   `json:"productName"`
	CreatedAt          int64                    `json:"createdAt"`
	Options            Options                  `json:"options"`
	QueryCount         int                      `json:"queryCount"`
	TotalSamples       int                      `json:"totalSamples"`
	TempAppDataRoot    string                   `json:"tempAppDataRoot,omitempty"`
	Summary            MetricSummary            `json:"summary"`
	QuerySummaries     []QuerySummary           `json:"querySummaries"`
	SlowestSamples     []Sample                 `json:"slowestSamples"`
	ActionValidation   ActionValidationSummary  `json:"actionValidation"`
	ProviderStatus     ProviderStatus           `json:"providerStatus"`
	RollingPerformance search.PerformanceStatus `json:"rollingPerformance"`
	Verdict            Verdict                  `json:"verdict"`
	Samples            []Sample                 `json:"samples"`
	VerificationNotes  []string                 `json:"verificationNotes"`
}

type MetricSummary struct {
	Count   int     `json:"count"`
	Min     int64   `json:"min"`
	Max     int64   `json:"max"`
	Average float64 `json:"average"`
	P95     int64   `json:"p95"`
}

type QuerySummary struct {
	Query                  string  `json:"query"`
	Count                  int     `json:"count"`
	MinMs                  int64   `json:"minMs"`
	MaxMs                  int64   `json:"maxMs"`
	AverageMs              float64 `json:"averageMs"`
	P95Ms                  int64   `json:"p95Ms"`
	AverageResultCount     float64 `json:"averageResultCount"`
	MaxResultCount         int     `json:"maxResultCount"`
	ZeroResultCount        int     `json:"zeroResultCount"`
	FileIndexFileSamples   int     `json:"fileIndexFileSamples"`
	FileIndexFileResults   int     `json:"fileIndexFileResults"`
	ActionValidationErrors int     `json:"actionValidationErrors"`
}

type Sample struct {
	Index                 int    `json:"index"`
	Iteration             int    `json:"iteration"`
	Query                 string `json:"query"`
	ElapsedMs             int64  `json:"elapsedMs"`
	ResultCount           int    `json:"resultCount"`
	TopResultID           string `json:"topResultId,omitempty"`
	TopResultTitle        string `json:"topResultTitle,omitempty"`
	TopResultType         string `json:"topResultType,omitempty"`
	FileResultCount       int    `json:"fileResultCount"`
	FileIndexFileResults  int    `json:"fileIndexFileResults"`
	ActionValidationError string `json:"actionValidationError,omitempty"`
}

type ActionValidationSummary struct {
	OK             bool   `json:"ok"`
	CheckedSamples int    `json:"checkedSamples"`
	CheckedResults int    `json:"checkedResults"`
	InvalidSamples int    `json:"invalidSamples"`
	LastError      string `json:"lastError,omitempty"`
}

type ProviderStatus struct {
	FileIndexStatusAvailable bool                       `json:"fileIndexStatusAvailable"`
	FileIndex                filesearch.FileIndexStatus `json:"fileIndex"`
	FileIndexFileHits        int                        `json:"fileIndexFileHits"`
	FileIndexHitQueries      []string                   `json:"fileIndexHitQueries,omitempty"`
}

type Verdict struct {
	WithinTarget bool     `json:"withinTarget"`
	Warnings     []string `json:"warnings,omitempty"`
}
