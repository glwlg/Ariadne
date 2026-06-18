package workmemory

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"ariadne/internal/appdb"
	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/contracts"
)

const (
	defaultAutoCaptureInterval = 30 * time.Second
	defaultMaxEntries          = 1000
	similarImageHashMaxBits    = 6
	qualityStatusPending       = "pending"
	qualityStatusChecked       = "checked"
)

var windowSwitchPollInterval = time.Second
var qualityReviewInterval = time.Hour
var ftsRebuildDebounceDelay = 750 * time.Millisecond

var sensitiveCredentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:password|passwd|pwd|api[_-]?key|apikey|token|access[_-]?token|refresh[_-]?token|id[_-]?token|secret|client[_-]?secret|private[_-]?key)\b["']?\s*[:=]\s*["']?[a-z0-9._~+/=@#$%^&*!?-]{4,}`),
	regexp.MustCompile(`(?i)\bauthorization\b\s*:\s*(?:bearer|basic)\s+[a-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]{16,}`),
	regexp.MustCompile(`(?i)\b(?:cookie|set-cookie)\s*:\s*[a-z0-9._~-]{2,}=[a-z0-9._~+/=%-]{3,}`),
	regexp.MustCompile(`(?i)-----begin [a-z0-9 ]*private key-----`),
	regexp.MustCompile(`(?i)\bssh-(?:rsa|ed25519)\s+[a-z0-9+/=]{32,}`),
	regexp.MustCompile(`(?i)\beyj[a-z0-9_-]{10,}\.eyj[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\b`),
	regexp.MustCompile(`(?i)\b(?:sk|rk)-[a-z0-9_-]{20,}\b`),
	regexp.MustCompile(`(?i)\bgh[pousr]_[a-z0-9_]{20,}\b`),
	regexp.MustCompile(`(?i)(?:数据库密码|密码|密钥|令牌|验证码)\s*[:：=]\s*["']?[a-z0-9._~+/=@#$%^&*!?-]{4,}`),
}

type Status struct {
	Enabled                    bool                `json:"enabled"`
	TimeMachineEnabled         bool                `json:"timeMachineEnabled"`
	WorkerRunning              bool                `json:"workerRunning"`
	PrivacyMode                bool                `json:"privacyMode"`
	PauseReason                string              `json:"pauseReason,omitempty"`
	AutoOCREnabled             bool                `json:"autoOcrEnabled"`
	CaptureScope               string              `json:"captureScope,omitempty"`
	MultiMonitor               string              `json:"multiMonitor,omitempty"`
	WindowSwitchCaptureEnabled bool                `json:"windowSwitchCaptureEnabled"`
	WindowSwitchCooldownSecs   int                 `json:"windowSwitchCooldownSeconds,omitempty"`
	AppCaptureProfiles         []AppCaptureProfile `json:"appCaptureProfiles,omitempty"`
	LastWindowSwitchAt         int64               `json:"lastWindowSwitchAt,omitempty"`
	LastWindowSwitchCaptureAt  int64               `json:"lastWindowSwitchCaptureAt,omitempty"`
	PauseOnIdle                bool                `json:"pauseOnIdle"`
	IdlePauseSeconds           int                 `json:"idlePauseSeconds,omitempty"`
	PauseOnLock                bool                `json:"pauseOnLock"`
	IdleSeconds                int                 `json:"idleSeconds,omitempty"`
	LastActivityAt             int64               `json:"lastActivityAt,omitempty"`
	SessionLocked              bool                `json:"sessionLocked"`
	EntryCount                 int                 `json:"entryCount"`
	AutoCaptureIntervalSeconds int                 `json:"autoCaptureIntervalSeconds"`
	LastCaptureAt              int64               `json:"lastCaptureAt,omitempty"`
	LastCaptureID              string              `json:"lastCaptureId,omitempty"`
	LastCaptureError           string              `json:"lastCaptureError,omitempty"`
	LastSkippedAt              int64               `json:"lastSkippedAt,omitempty"`
	LastSkippedReason          string              `json:"lastSkippedReason,omitempty"`
	LastAutoOCRAt              int64               `json:"lastAutoOcrAt,omitempty"`
	LastAutoOCRID              string              `json:"lastAutoOcrId,omitempty"`
	LastAutoOCRError           string              `json:"lastAutoOcrError,omitempty"`
	CaptureCount               int                 `json:"captureCount"`
	StoragePath                string              `json:"storagePath,omitempty"`
}

type DraftSchedulePolicy struct {
	Enabled                 bool `json:"enabled"`
	IntervalMinutes         int  `json:"intervalMinutes"`
	DailyDraftEnabled       bool `json:"dailyDraftEnabled"`
	RetrospectiveEnabled    bool `json:"retrospectiveEnabled"`
	ExperienceReportEnabled bool `json:"experienceReportEnabled"`
	ExperiencePeriodDays    int  `json:"experiencePeriodDays"`
}

type ScheduledDraftStatus struct {
	Enabled                 bool             `json:"enabled"`
	Running                 bool             `json:"running"`
	IntervalMinutes         int              `json:"intervalMinutes"`
	DailyDraftEnabled       bool             `json:"dailyDraftEnabled"`
	RetrospectiveEnabled    bool             `json:"retrospectiveEnabled"`
	ExperienceReportEnabled bool             `json:"experienceReportEnabled"`
	LastCheckedAt           int64            `json:"lastCheckedAt,omitempty"`
	LastRunAt               int64            `json:"lastRunAt,omitempty"`
	LastEntryCount          int              `json:"lastEntryCount"`
	LastEntryCreatedAt      int64            `json:"lastEntryCreatedAt,omitempty"`
	LastError               string           `json:"lastError,omitempty"`
	LastAutonomousRunAt     int64            `json:"lastAutonomousRunAt,omitempty"`
	AutonomousGenerated     int              `json:"autonomousGenerated"`
	AutonomousMessage       string           `json:"autonomousMessage,omitempty"`
	DailyDraft              Draft            `json:"dailyDraft,omitempty"`
	RetrospectiveDraft      Draft            `json:"retrospectiveDraft,omitempty"`
	ExperienceReport        ExperienceReport `json:"experienceReport,omitempty"`
}

type AutonomousArtifact struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	Body            string   `json:"body"`
	Evidence        []string `json:"evidence"`
	SourceInsightID string   `json:"sourceInsightId,omitempty"`
	DedupKey        string   `json:"dedupKey,omitempty"`
	Status          string   `json:"status"`
	DeleteReason    string   `json:"deleteReason,omitempty"`
	Confidence      float64  `json:"confidence,omitempty"`
	AgentExecutable bool     `json:"agentExecutable,omitempty"`
	CreatedAt       int64    `json:"createdAt"`
	UpdatedAt       int64    `json:"updatedAt,omitempty"`
	DeletedAt       int64    `json:"deletedAt,omitempty"`
}

type AutonomousArtifactRejectRequest struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type AutonomousArtifactRejectResult struct {
	OK       bool                 `json:"ok"`
	Message  string               `json:"message"`
	Artifact AutonomousArtifact   `json:"artifact,omitempty"`
	Status   ScheduledDraftStatus `json:"status"`
}

type AutonomousRunResult struct {
	OK        bool                 `json:"ok"`
	Message   string               `json:"message"`
	Generated int                  `json:"generated"`
	Skipped   int                  `json:"skipped"`
	Artifacts []AutonomousArtifact `json:"artifacts"`
	Status    ScheduledDraftStatus `json:"status"`
	CreatedAt int64                `json:"createdAt"`
}

type AutonomousRejection struct {
	Key        string `json:"key"`
	ArtifactID string `json:"artifactId,omitempty"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
	RejectedAt int64  `json:"rejectedAt"`
}

type SemanticStatus struct {
	Enabled                bool   `json:"enabled"`
	Provider               string `json:"provider"`
	Mode                   string `json:"mode"`
	External               bool   `json:"external"`
	FTSEnabled             bool   `json:"ftsEnabled"`
	FTSPath                string `json:"ftsPath,omitempty"`
	IndexedEntries         int    `json:"indexedEntries"`
	LastIndexedAt          int64  `json:"lastIndexedAt,omitempty"`
	LastIndexError         string `json:"lastIndexError,omitempty"`
	ExternalEmbeddingReady bool   `json:"externalEmbeddingReady"`
	ExternalProvider       string `json:"externalProvider,omitempty"`
	EmbeddingModel         string `json:"embeddingModel,omitempty"`
	EmbeddingIndexed       int    `json:"embeddingIndexed"`
	LastEmbeddingAt        int64  `json:"lastEmbeddingAt,omitempty"`
	LastEmbeddingError     string `json:"lastEmbeddingError,omitempty"`
	VectorStoreType        string `json:"vectorStoreType,omitempty"`
	VectorStoreURI         string `json:"vectorStoreUri,omitempty"`
	VectorCollection       string `json:"vectorCollection,omitempty"`
	Note                   string `json:"note,omitempty"`
}

type EmbeddingPolicy struct {
	Enabled          bool   `json:"enabled"`
	Provider         string `json:"provider"`
	BaseURL          string `json:"baseUrl"`
	Model            string `json:"model"`
	VectorStoreType  string `json:"vectorStoreType"`
	VectorStoreURI   string `json:"vectorStoreUri"`
	VectorCollection string `json:"vectorCollection"`
}

type EmbeddingJob struct {
	Provider string
	BaseURL  string
	Model    string
	Inputs   []string
}

type EmbeddingClient interface {
	EmbedTexts(context.Context, EmbeddingJob) ([][]float64, error)
}

type EmbeddingRefreshResult struct {
	OK             bool           `json:"ok"`
	Message        string         `json:"message"`
	Status         SemanticStatus `json:"status"`
	Indexed        int            `json:"indexed"`
	Skipped        int            `json:"skipped"`
	Failed         int            `json:"failed"`
	Provider       string         `json:"provider,omitempty"`
	Model          string         `json:"model,omitempty"`
	RefreshedAt    int64          `json:"refreshedAt,omitempty"`
	RequiresReview bool           `json:"requiresReview"`
}

type SemanticSearchResult struct {
	OK       bool                     `json:"ok"`
	Message  string                   `json:"message"`
	Query    string                   `json:"query"`
	Results  []contracts.SearchResult `json:"results"`
	Status   SemanticStatus           `json:"status"`
	Provider string                   `json:"provider,omitempty"`
	Model    string                   `json:"model,omitempty"`
}

type FlowAskRequest struct {
	Question string `json:"question"`
	Limit    int    `json:"limit,omitempty"`
	Since    int64  `json:"since,omitempty"`
}

type FlowAskEvidence struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Source      string   `json:"source"`
	AppName     string   `json:"appName,omitempty"`
	WindowTitle string   `json:"windowTitle,omitempty"`
	CreatedAt   int64    `json:"createdAt"`
	Score       float64  `json:"score,omitempty"`
	HasImage    bool     `json:"hasImage"`
	Sensitive   bool     `json:"sensitive"`
	Tags        []string `json:"tags"`
}

type FlowAskResponse struct {
	OK                 bool              `json:"ok"`
	Question           string            `json:"question"`
	Title              string            `json:"title"`
	Answer             string            `json:"answer"`
	Intent             string            `json:"intent"`
	Mode               string            `json:"mode"`
	Evidence           []FlowAskEvidence `json:"evidence"`
	SuggestedQuestions []string          `json:"suggestedQuestions,omitempty"`
	UsedAI             bool              `json:"usedAi"`
	Message            string            `json:"message,omitempty"`
	CreatedAt          int64             `json:"createdAt"`
}

type FlowAgentPolicy struct {
	Enabled      bool   `json:"enabled"`
	Runner       string `json:"runner"`
	Provider     string `json:"provider,omitempty"`
	BaseURL      string `json:"baseUrl,omitempty"`
	Model        string `json:"model,omitempty"`
	NativeSkills bool   `json:"nativeSkills,omitempty"`
	WorkDir      string `json:"workDir,omitempty"`
}

type FlowAgentEvidence struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Text        string   `json:"text,omitempty"`
	OCRText     string   `json:"ocrText,omitempty"`
	Source      string   `json:"source"`
	AppName     string   `json:"appName,omitempty"`
	WindowTitle string   `json:"windowTitle,omitempty"`
	CreatedAt   int64    `json:"createdAt"`
	HasImage    bool     `json:"hasImage"`
	Tags        []string `json:"tags,omitempty"`
}

type FlowAgentJob struct {
	Question     string              `json:"question"`
	Intent       string              `json:"intent"`
	LocalAnswer  string              `json:"localAnswer"`
	Evidence     []FlowAgentEvidence `json:"evidence"`
	ToolCommand  string              `json:"toolCommand,omitempty"`
	Runner       string              `json:"runner"`
	Provider     string              `json:"provider,omitempty"`
	BaseURL      string              `json:"baseUrl,omitempty"`
	Model        string              `json:"model,omitempty"`
	NativeSkills bool                `json:"nativeSkills,omitempty"`
	WorkDir      string              `json:"workDir,omitempty"`
	Now          time.Time           `json:"now"`
}

type FlowAgentResult struct {
	Answer  string `json:"answer"`
	Mode    string `json:"mode,omitempty"`
	Message string `json:"message,omitempty"`
}

type FlowAgentRunner interface {
	AnswerFlow(context.Context, FlowAgentJob) (FlowAgentResult, error)
}

type Entry struct {
	ID               string         `json:"id"`
	Source           string         `json:"source"`
	ContentType      string         `json:"contentType"`
	Title            string         `json:"title"`
	Summary          string         `json:"summary"`
	Text             string         `json:"text"`
	OCRText          string         `json:"ocrText,omitempty"`
	OCRStatus        string         `json:"ocrStatus,omitempty"`
	QualityOCRText   string         `json:"qualityOcrText,omitempty"`
	QualityOCRStatus string         `json:"qualityOcrStatus,omitempty"`
	WindowTitle      string         `json:"windowTitle,omitempty"`
	AppName          string         `json:"appName,omitempty"`
	CaptureID        string         `json:"captureId,omitempty"`
	ImagePath        string         `json:"imagePath,omitempty"`
	ImageSignature   string         `json:"imageSignature,omitempty"`
	ImageFingerprint string         `json:"imageFingerprint,omitempty"`
	Frames           []CaptureFrame `json:"frames,omitempty"`
	FrameCount       int            `json:"frameCount,omitempty"`
	QualityStatus    string         `json:"qualityStatus,omitempty"`
	QualityCheckedAt int64          `json:"qualityCheckedAt,omitempty"`
	QualityReason    string         `json:"qualityReason,omitempty"`
	Width            int            `json:"width,omitempty"`
	Height           int            `json:"height,omitempty"`
	Bytes            int64          `json:"bytes,omitempty"`
	Tags             []string       `json:"tags"`
	Favorite         bool           `json:"favorite"`
	Sensitive        bool           `json:"sensitive"`
	MergedCount      int            `json:"mergedCount,omitempty"`
	LastMergedAt     int64          `json:"lastMergedAt,omitempty"`
	CreatedAt        int64          `json:"createdAt"`
}

type CaptureFrame struct {
	CaptureID        string `json:"captureId,omitempty"`
	ImagePath        string `json:"imagePath,omitempty"`
	ImageSignature   string `json:"imageSignature,omitempty"`
	ImageFingerprint string `json:"imageFingerprint,omitempty"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	Bytes            int64  `json:"bytes,omitempty"`
	WindowTitle      string `json:"windowTitle,omitempty"`
	AppName          string `json:"appName,omitempty"`
	CreatedAt        int64  `json:"createdAt"`
}

type Draft struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Evidence  []string `json:"evidence"`
	CreatedAt int64    `json:"createdAt"`
}

type DraftPolishPolicy struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
}

type OCRSummaryPolicy struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
}

type OCRSummaryJob struct {
	Entry    Entry     `json:"entry"`
	OCRText  string    `json:"ocrText"`
	Provider string    `json:"provider"`
	BaseURL  string    `json:"baseUrl"`
	Model    string    `json:"model"`
	Now      time.Time `json:"now"`
}

type OCRSummaryResult struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
}

type OCRSummarizer interface {
	SummarizeOCR(context.Context, OCRSummaryJob) (OCRSummaryResult, error)
}

type DraftPolishRequest struct {
	Draft     Draft  `json:"draft"`
	Kind      string `json:"kind"`
	Confirmed bool   `json:"confirmed"`
}

type DraftPolishResult struct {
	OK                   bool     `json:"ok"`
	Message              string   `json:"message"`
	Draft                Draft    `json:"draft"`
	PolishedDraft        Draft    `json:"polishedDraft,omitempty"`
	RequiresConfirmation bool     `json:"requiresConfirmation"`
	External             bool     `json:"external"`
	Provider             string   `json:"provider,omitempty"`
	Model                string   `json:"model,omitempty"`
	RiskReasons          []string `json:"riskReasons,omitempty"`
}

type DraftPolishJob struct {
	Draft    Draft
	Kind     string
	Provider string
	BaseURL  string
	Model    string
}

type DraftPolisher interface {
	PolishDraft(context.Context, DraftPolishJob) (Draft, error)
}

type ExperienceDiscoveryPolicy struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
}

type AgentTaskPackage struct {
	ID             string   `json:"id"`
	Goal           string   `json:"goal"`
	Context        string   `json:"context"`
	Evidence       []string `json:"evidence"`
	Boundaries     []string `json:"boundaries"`
	Acceptance     []string `json:"acceptance"`
	RequiresReview bool     `json:"requiresReview"`
	CreatedAt      int64    `json:"createdAt"`
}

type WorkflowDraftStep struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Command         string `json:"command"`
	RequiresConfirm bool   `json:"requiresConfirm"`
}

type WorkflowDraft struct {
	ID             string              `json:"id"`
	Title          string              `json:"title"`
	Trigger        string              `json:"trigger"`
	Input          string              `json:"input"`
	Steps          []WorkflowDraftStep `json:"steps"`
	Output         string              `json:"output"`
	RiskLevel      string              `json:"riskLevel"`
	Evidence       []string            `json:"evidence"`
	RequiresReview bool                `json:"requiresReview"`
	CreatedAt      int64               `json:"createdAt"`
}

type ChecklistDraft struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Context        string   `json:"context"`
	Items          []string `json:"items"`
	Evidence       []string `json:"evidence"`
	RequiresReview bool     `json:"requiresReview"`
	CreatedAt      int64    `json:"createdAt"`
}

type ExperienceInsight struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	Reason            string   `json:"reason"`
	Recommendation    string   `json:"recommendation"`
	Evidence          []string `json:"evidence"`
	Confidence        float64  `json:"confidence"`
	Severity          string   `json:"severity"`
	RequiresReview    bool     `json:"requiresReview"`
	CreatedAt         int64    `json:"createdAt"`
	DecisionStatus    string   `json:"decisionStatus,omitempty"`
	DecisionNote      string   `json:"decisionNote,omitempty"`
	DecisionUpdatedAt int64    `json:"decisionUpdatedAt,omitempty"`
	TaskPackageID     string   `json:"taskPackageId,omitempty"`
}

type ExperienceReport struct {
	ID            string              `json:"id"`
	Title         string              `json:"title"`
	Summary       string              `json:"summary"`
	PeriodDays    int                 `json:"periodDays"`
	EntryCount    int                 `json:"entryCount"`
	EvidenceCount int                 `json:"evidenceCount"`
	Insights      []ExperienceInsight `json:"insights"`
	GeneratedAt   int64               `json:"generatedAt"`
}

type ExperienceDiscoveryRequest struct {
	PeriodDays int  `json:"periodDays"`
	External   bool `json:"external"`
	Confirmed  bool `json:"confirmed"`
}

type ExperienceDiscoveryResult struct {
	OK                   bool             `json:"ok"`
	Message              string           `json:"message"`
	Report               ExperienceReport `json:"report"`
	RequiresConfirmation bool             `json:"requiresConfirmation"`
	External             bool             `json:"external"`
	Provider             string           `json:"provider,omitempty"`
	Model                string           `json:"model,omitempty"`
	RiskReasons          []string         `json:"riskReasons,omitempty"`
}

type ExperienceDiscoveryEvidence struct {
	ID        string   `json:"id"`
	Source    string   `json:"source"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Text      string   `json:"text"`
	AppName   string   `json:"appName,omitempty"`
	Tags      []string `json:"tags"`
	CreatedAt int64    `json:"createdAt"`
}

type ExperienceDiscoveryJob struct {
	Evidence   []ExperienceDiscoveryEvidence
	PeriodDays int
	Provider   string
	BaseURL    string
	Model      string
	Now        time.Time
}

type ExperienceDiscoverer interface {
	DiscoverExperiences(context.Context, ExperienceDiscoveryJob) (ExperienceReport, error)
}

type ExperienceDecision struct {
	InsightID     string `json:"insightId"`
	Status        string `json:"status"`
	Note          string `json:"note,omitempty"`
	TaskPackageID string `json:"taskPackageId,omitempty"`
	UpdatedAt     int64  `json:"updatedAt"`
}

type ExperienceDecisionResult struct {
	OK       bool               `json:"ok"`
	Message  string             `json:"message"`
	Decision ExperienceDecision `json:"decision,omitempty"`
}

type NoteRequest struct {
	Title     string   `json:"title"`
	Text      string   `json:"text"`
	Tags      []string `json:"tags"`
	Favorite  bool     `json:"favorite"`
	Sensitive bool     `json:"sensitive"`
}

type ExportFilter struct {
	StartAt  int64    `json:"startAt,omitempty"`
	EndAt    int64    `json:"endAt,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	EntryIDs []string `json:"entryIds,omitempty"`
}

type ExportRequest struct {
	IncludeSensitive bool     `json:"includeSensitive"`
	StartAt          int64    `json:"startAt,omitempty"`
	EndAt            int64    `json:"endAt,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	EntryIDs         []string `json:"entryIds,omitempty"`
}

type ExportResult struct {
	OK                    bool         `json:"ok"`
	Message               string       `json:"message"`
	Path                  string       `json:"path,omitempty"`
	EntryCount            int          `json:"entryCount"`
	SkippedSensitiveCount int          `json:"skippedSensitiveCount"`
	SkippedExcludedCount  int          `json:"skippedExcludedCount"`
	FilteredOutCount      int          `json:"filteredOutCount"`
	IncludesSensitive     bool         `json:"includesSensitive"`
	Filter                ExportFilter `json:"filter,omitempty"`
	Bytes                 int64        `json:"bytes,omitempty"`
	CreatedAt             int64        `json:"createdAt,omitempty"`
}

type ImportMaterialRequest struct {
	Paths     []string `json:"paths"`
	Tags      []string `json:"tags,omitempty"`
	Favorite  bool     `json:"favorite,omitempty"`
	Sensitive bool     `json:"sensitive,omitempty"`
}

type ImportMaterialItemResult struct {
	Path        string `json:"path"`
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	EntryID     string `json:"entryId,omitempty"`
	Source      string `json:"source,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
}

type ImportMaterialResult struct {
	OK        bool                       `json:"ok"`
	Message   string                     `json:"message"`
	Imported  int                        `json:"imported"`
	Skipped   int                        `json:"skipped"`
	Failed    int                        `json:"failed"`
	Entries   []Entry                    `json:"entries"`
	Items     []ImportMaterialItemResult `json:"items"`
	CreatedAt int64                      `json:"createdAt"`
}

type RetentionResult struct {
	OK             bool  `json:"ok"`
	RetentionDays  int   `json:"retentionDays"`
	KeepFavorites  bool  `json:"keepFavorites"`
	CutoffAt       int64 `json:"cutoffAt,omitempty"`
	Removed        int   `json:"removed"`
	Kept           int   `json:"kept"`
	KeptFavorites  int   `json:"keptFavorites"`
	RemainingCount int   `json:"remainingCount"`
	AppliedAt      int64 `json:"appliedAt"`
}

type QualityReviewResult struct {
	OK               bool   `json:"ok"`
	Message          string `json:"message"`
	Checked          int    `json:"checked"`
	CollapsedEntries int    `json:"collapsedEntries"`
	RemovedFrames    int    `json:"removedFrames"`
	QualityOCR       int    `json:"qualityOcr"`
	OCRPromoted      int    `json:"ocrPromoted"`
	SkippedActive    int    `json:"skippedActive"`
	PendingRemaining int    `json:"pendingRemaining"`
	ReviewedAt       int64  `json:"reviewedAt"`
}

type HealthAppStat struct {
	AppName    string `json:"appName"`
	Count      int    `json:"count"`
	Pending    int    `json:"pending"`
	Checked    int    `json:"checked"`
	OCRDone    int    `json:"ocrDone"`
	QualityOCR int    `json:"qualityOcr"`
	Sensitive  int    `json:"sensitive"`
	LastSeenAt int64  `json:"lastSeenAt,omitempty"`
}

type HealthRecentEvent struct {
	ID        string `json:"id,omitempty"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Detail    string `json:"detail,omitempty"`
	AppName   string `json:"appName,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

type HealthSummary struct {
	OK                 bool                `json:"ok"`
	Message            string              `json:"message"`
	Total              int                 `json:"total"`
	Today              int                 `json:"today"`
	Pending            int                 `json:"pending"`
	Checked            int                 `json:"checked"`
	Sensitive          int                 `json:"sensitive"`
	Images             int                 `json:"images"`
	MultiFrame         int                 `json:"multiFrame"`
	CollapsedEntries   int                 `json:"collapsedEntries"`
	RemovedFrames      int                 `json:"removedFrames"`
	OCRDone            int                 `json:"ocrDone"`
	OCRPending         int                 `json:"ocrPending"`
	OCRFailed          int                 `json:"ocrFailed"`
	QualityOCRDone     int                 `json:"qualityOcrDone"`
	QualityOCRPending  int                 `json:"qualityOcrPending"`
	QualityOCRFailed   int                 `json:"qualityOcrFailed"`
	SkippedSensitive   int                 `json:"skippedSensitive"`
	SkippedPending     int                 `json:"skippedPending"`
	LastCaptureAt      int64               `json:"lastCaptureAt,omitempty"`
	LastQualityCheckAt int64               `json:"lastQualityCheckAt,omitempty"`
	LastAutoOCRAt      int64               `json:"lastAutoOcrAt,omitempty"`
	LastSkippedReason  string              `json:"lastSkippedReason,omitempty"`
	LastAutoOCRError   string              `json:"lastAutoOcrError,omitempty"`
	AppStats           []HealthAppStat     `json:"appStats,omitempty"`
	RecentEvents       []HealthRecentEvent `json:"recentEvents,omitempty"`
	GeneratedAt        int64               `json:"generatedAt"`
}

type ScreenCapturer interface {
	CaptureScreen(source string) capturehistory.Status
}

type OptionScreenCapturer interface {
	CaptureScreenWithOptions(source string, options capturehistory.CaptureOptions) capturehistory.Status
}

type CapturePolicy struct {
	ExcludeApps              []string            `json:"excludeApps,omitempty"`
	ExcludeWindowKeywords    []string            `json:"excludeWindowKeywords,omitempty"`
	ExcludePaths             []string            `json:"excludePaths,omitempty"`
	ExcludeURLs              []string            `json:"excludeUrls,omitempty"`
	ExcludeContentPatterns   []string            `json:"excludeContentPatterns,omitempty"`
	SensitiveRulesEnabled    bool                `json:"sensitiveRulesEnabled"`
	SensitiveRulesConfigured bool                `json:"sensitiveRulesConfigured,omitempty"`
	AppCaptureProfiles       []AppCaptureProfile `json:"appCaptureProfiles,omitempty"`
	AutoOCR                  bool                `json:"autoOcr"`
	CaptureScope             string              `json:"captureScope,omitempty"`
	MultiMonitor             string              `json:"multiMonitor,omitempty"`
	CaptureOnWindowChange    bool                `json:"captureOnWindowChange"`
	WindowChangeCooldown     int                 `json:"windowChangeCooldownSeconds,omitempty"`
	PauseOnIdle              bool                `json:"pauseOnIdle"`
	IdlePauseSeconds         int                 `json:"idlePauseSeconds,omitempty"`
	PauseOnLock              bool                `json:"pauseOnLock"`
}

type AppCaptureProfile struct {
	ID                       string `json:"id"`
	DisplayName              string `json:"displayName"`
	ProcessName              string `json:"processName"`
	Icon                     string `json:"icon,omitempty"`
	Enabled                  bool   `json:"enabled"`
	WindowSwitchDelaySeconds int    `json:"windowSwitchDelaySeconds"`
	ActiveIntervalSeconds    int    `json:"activeIntervalSeconds"`
}

type windowContext struct {
	title string
	app   string
}

type activitySnapshot struct {
	Available      bool
	IdleSeconds    int
	LastActivityAt int64
	SessionLocked  bool
	Error          string
}

type activityProvider interface {
	Snapshot(now time.Time) activitySnapshot
}

type activityProviderFunc func(now time.Time) activitySnapshot

func (fn activityProviderFunc) Snapshot(now time.Time) activitySnapshot {
	return fn(now)
}

type AutoOCRProcessor func(Entry) Entry

type embeddingRecord struct {
	EntryID   string
	Vector    []float64 `json:",omitempty"`
	IndexedAt int64
}

type embeddingStateFile struct {
	Version          int               `json:"version"`
	Provider         string            `json:"provider"`
	Model            string            `json:"model"`
	VectorStoreType  string            `json:"vectorStoreType"`
	VectorStoreURI   string            `json:"vectorStoreUri,omitempty"`
	VectorCollection string            `json:"vectorCollection"`
	LastIndexedAt    int64             `json:"lastIndexedAt"`
	Records          []embeddingRecord `json:"records"`
}

type Service struct {
	mu                            sync.RWMutex
	path                          string
	status                        Status
	entries                       []Entry
	decisions                     map[string]ExperienceDecision
	autonomousArtifacts           []AutonomousArtifact
	autonomousRejections          map[string]AutonomousRejection
	lastAutonomousRunAt           int64
	capturer                      ScreenCapturer
	maxEntries                    int
	interval                      time.Duration
	stopWorker                    chan struct{}
	draftSchedule                 DraftSchedulePolicy
	draftScheduleInterval         time.Duration
	stopDraftScheduler            chan struct{}
	scheduledDrafts               ScheduledDraftStatus
	now                           func() time.Time
	context                       func() windowContext
	activity                      activityProvider
	autoOCR                       AutoOCRProcessor
	ocrSummarizer                 OCRSummarizer
	ocrSummaryPolicy              OCRSummaryPolicy
	draftPolisher                 DraftPolisher
	draftPolishPolicy             DraftPolishPolicy
	flowAgentRunner               FlowAgentRunner
	flowAgentPolicy               FlowAgentPolicy
	experienceDiscoverer          ExperienceDiscoverer
	experiencePolicy              ExperienceDiscoveryPolicy
	embedder                      EmbeddingClient
	embeddingPolicy               EmbeddingPolicy
	embeddingIndex                map[string]embeddingRecord
	embeddingIndexed              int
	embeddingError                string
	lastEmbeddingAt               int64
	fts                           *ftsIndex
	ftsError                      string
	ftsDirty                      bool
	ftsRebuildScheduled           bool
	ftsRebuildDisabled            bool
	policy                        CapturePolicy
	lastWindowSignature           string
	lastWindowContext             windowContext
	lastWindowCaptureAt           int64
	pendingWindowSignature        string
	pendingWindowDueAt            int64
	lastAppCaptureAt              map[string]int64
	currentWindowSessionSignature string
	currentWindowSessionEntryID   string
	currentWindowSessionStartedAt int64
	qualityReviewRunning          bool
	saveError                     string
}

func NewService(capturers ...ScreenCapturer) *Service {
	var capturer ScreenCapturer
	if len(capturers) > 0 {
		capturer = capturers[0]
	}
	return NewServiceWithPath(defaultMemoryPath(), capturer)
}

func NewServiceWithPath(path string, capturer ScreenCapturer) *Service {
	service := &Service{
		path:                 path,
		capturer:             capturer,
		decisions:            map[string]ExperienceDecision{},
		autonomousRejections: map[string]AutonomousRejection{},
		embeddingIndex:       map[string]embeddingRecord{},
		lastAppCaptureAt:     map[string]int64{},
		maxEntries:           defaultMaxEntries,
		interval:             defaultAutoCaptureInterval,
		draftSchedule: DraftSchedulePolicy{
			IntervalMinutes:         240,
			DailyDraftEnabled:       true,
			RetrospectiveEnabled:    true,
			ExperienceReportEnabled: true,
			ExperiencePeriodDays:    7,
		},
		draftScheduleInterval: 240 * time.Minute,
		now:                   time.Now,
		context:               defaultWindowContextProvider(),
		activity:              defaultActivityProvider(),
		policy: CapturePolicy{
			AutoOCR:               false,
			SensitiveRulesEnabled: true,
			CaptureScope:          "active_window",
			MultiMonitor:          "combined",
			CaptureOnWindowChange: true,
			WindowChangeCooldown:  3,
			PauseOnIdle:           true,
			IdlePauseSeconds:      600,
			PauseOnLock:           true,
		},
		status: Status{
			Enabled:                    true,
			TimeMachineEnabled:         false,
			PrivacyMode:                false,
			AutoOCREnabled:             false,
			AutoCaptureIntervalSeconds: int(defaultAutoCaptureInterval.Seconds()),
			CaptureScope:               "active_window",
			MultiMonitor:               "combined",
			WindowSwitchCaptureEnabled: true,
			WindowSwitchCooldownSecs:   3,
			PauseOnIdle:                true,
			IdlePauseSeconds:           600,
			PauseOnLock:                true,
			StoragePath:                firstNonEmpty(appdb.DatabasePathForPath(path), path),
		},
	}
	service.scheduledDrafts = service.scheduledDraftStatusLocked()
	if !service.load() && strings.TrimSpace(path) == "" {
		service.entries = seedEntries()
	}
	service.refreshEntryCountLocked()
	if len(service.entries) > 0 {
		service.initFTSIndex()
	}
	service.loadEmbeddingIndex()
	return service
}

func RegisterAutoOCRProcessor(service *Service, processor AutoOCRProcessor) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.autoOCR = processor
}

func RegisterOCRSummarizer(service *Service, summarizer OCRSummarizer) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.ocrSummarizer = summarizer
}

func RegisterDraftPolisher(service *Service, polisher DraftPolisher) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.draftPolisher = polisher
}

func RegisterFlowAgentRunner(service *Service, runner FlowAgentRunner) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.flowAgentRunner = runner
}

func RegisterExperienceDiscoverer(service *Service, discoverer ExperienceDiscoverer) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.experienceDiscoverer = discoverer
}

func RegisterEmbeddingClient(service *Service, embedder EmbeddingClient) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.embedder = embedder
}

func (s *Service) ApplyDraftPolishPolicy(policy DraftPolishPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.draftPolishPolicy = normalizeDraftPolishPolicy(policy)
}

func (s *Service) ApplyOCRSummaryPolicy(policy OCRSummaryPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ocrSummaryPolicy = normalizeOCRSummaryPolicy(policy)
}

func (s *Service) ApplyFlowAgentPolicy(policy FlowAgentPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flowAgentPolicy = normalizeFlowAgentPolicy(policy)
}

func (s *Service) ApplyExperienceDiscoveryPolicy(policy ExperienceDiscoveryPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.experiencePolicy = normalizeExperienceDiscoveryPolicy(policy)
}

func (s *Service) ApplyEmbeddingPolicy(policy EmbeddingPolicy) SemanticStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embeddingPolicy = normalizeEmbeddingPolicy(policy)
	return s.semanticStatusLocked()
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusSnapshotLocked()
}

func (s *Service) HealthSummary() HealthSummary {
	now := s.now()
	s.mu.RLock()
	entries := cloneEntries(s.entries)
	status := s.statusSnapshotLocked()
	s.mu.RUnlock()
	return buildHealthSummary(entries, status, now)
}

func (s *Service) LegacyEntryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, entry := range s.entries {
		if entry.Source == "legacy_x_tools" || stringListContainsFold(entry.Tags, "legacy_x_tools") {
			count++
		}
	}
	return count
}

func (s *Service) SemanticStatus() SemanticStatus {
	s.ensureFTSReady()
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.semanticStatusLocked()
}

func (s *Service) semanticStatusLocked() SemanticStatus {
	status := SemanticStatus{
		Enabled:        true,
		Provider:       "local_term_vector",
		Mode:           "local",
		External:       false,
		IndexedEntries: len(s.entries),
		Note:           "本地 token/短语向量检索；embedding/Milvus 仍需按设置接入。",
	}
	if s.fts != nil {
		status.Provider = "sqlite_fts5+local_term_vector"
		status.FTSEnabled = true
		status.FTSPath = s.fts.Path()
		status.Note = "SQLite FTS5 本地关键词索引 + token/短语向量降级；embedding/Milvus 仍需按设置接入。"
		if count, err := s.fts.Count(); err == nil {
			status.IndexedEntries = count
		} else if s.ftsError == "" {
			status.LastIndexError = err.Error()
		}
	}
	if s.ftsError != "" {
		status.LastIndexError = s.ftsError
	}
	if s.embeddingPolicy.Enabled {
		status.ExternalEmbeddingReady = s.embedder != nil
		status.ExternalProvider = s.embeddingPolicy.Provider
		status.EmbeddingModel = s.embeddingPolicy.Model
		status.VectorStoreType = s.embeddingPolicy.VectorStoreType
		status.VectorStoreURI = s.embeddingPolicy.VectorStoreURI
		status.VectorCollection = s.embeddingPolicy.VectorCollection
		status.EmbeddingIndexed = s.embeddingIndexed
		if status.EmbeddingIndexed == 0 && len(s.embeddingIndex) > 0 {
			status.EmbeddingIndexed = len(s.embeddingIndex)
		}
		status.LastEmbeddingAt = s.lastEmbeddingAt
		status.LastEmbeddingError = s.embeddingError
		if status.EmbeddingIndexed > 0 {
			status.External = true
			status.Mode = "hybrid"
			if s.embeddingPolicy.VectorStoreType == "milvus" {
				status.Provider += "+milvus_embedding"
				status.Note = "SQLite FTS5 / 本地 token 检索保持回退；外部 embedding 已写入 Milvus collection。"
			} else {
				status.Provider += "+external_embedding"
				status.Note = "SQLite FTS5 / 本地 token 检索保持回退；外部 embedding 已刷新到内置向量缓存。"
			}
		} else if s.embeddingPolicy.VectorStoreType == "milvus" {
			status.Note += " Milvus 目标已记录但尚未刷新索引。"
		} else if status.Note != "" {
			status.Note += " 外部 embedding 已配置但尚未刷新索引。"
		}
	}
	for _, entry := range s.entries {
		if entry.CreatedAt > status.LastIndexedAt {
			status.LastIndexedAt = entry.CreatedAt
		}
	}
	return status
}

func (s *Service) ApplySettings(enabled bool, privacyMode bool, timeMachineEnabled bool, intervalSeconds int) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.Enabled = enabled
	s.status.PrivacyMode = privacyMode
	oldInterval := s.interval
	s.setIntervalLocked(intervalSeconds)
	if !enabled {
		s.status.TimeMachineEnabled = false
		s.status.PauseReason = "工作记忆已停用"
		s.stopWorkerLocked()
		s.stopDraftSchedulerLocked()
		return s.statusSnapshotLocked()
	}
	if privacyMode {
		s.status.TimeMachineEnabled = false
		s.status.PauseReason = "隐私模式已开启"
		s.stopWorkerLocked()
		s.stopDraftSchedulerLocked()
		s.scheduledDrafts.LastError = "隐私模式已开启，定期草稿暂停"
		return s.statusSnapshotLocked()
	}
	s.status.PauseReason = ""
	s.status.TimeMachineEnabled = timeMachineEnabled
	if timeMachineEnabled {
		if oldInterval != s.interval && s.stopWorker != nil {
			s.restartWorkerLocked()
		} else {
			s.startWorkerLocked()
		}
	} else {
		s.stopWorkerLocked()
	}
	return s.statusSnapshotLocked()
}

func (s *Service) ApplyDraftSchedule(policy DraftSchedulePolicy) ScheduledDraftStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldInterval := s.draftScheduleInterval
	s.draftSchedule = normalizeDraftSchedulePolicy(policy)
	s.draftScheduleInterval = time.Duration(s.draftSchedule.IntervalMinutes) * time.Minute
	if !s.status.Enabled || s.status.PrivacyMode || !s.draftSchedule.Enabled {
		s.stopDraftSchedulerLocked()
		s.scheduledDrafts.Enabled = s.draftSchedule.Enabled
		s.scheduledDrafts.Running = false
		s.scheduledDrafts.IntervalMinutes = s.draftSchedule.IntervalMinutes
		s.scheduledDrafts.DailyDraftEnabled = s.draftSchedule.DailyDraftEnabled
		s.scheduledDrafts.RetrospectiveEnabled = s.draftSchedule.RetrospectiveEnabled
		s.scheduledDrafts.ExperienceReportEnabled = s.draftSchedule.ExperienceReportEnabled
		if s.status.PrivacyMode {
			s.scheduledDrafts.LastError = "隐私模式已开启，定期草稿暂停"
		}
		return s.scheduledDraftStatusLocked()
	}
	if oldInterval != s.draftScheduleInterval && s.stopDraftScheduler != nil {
		s.restartDraftSchedulerLocked()
	} else {
		s.startDraftSchedulerLocked()
	}
	return s.scheduledDraftStatusLocked()
}

func (s *Service) ScheduledDraftStatus() ScheduledDraftStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scheduledDraftStatusLocked()
}

func (s *Service) RunScheduledDraftsNow() ScheduledDraftStatus {
	return s.runScheduledDrafts(true)
}

func (s *Service) AutonomousArtifacts() []AutonomousArtifact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return activeAutonomousArtifacts(s.autonomousArtifacts)
}

func (s *Service) RunAutonomousFlowNow() AutonomousRunResult {
	result := s.runAutonomousFlow(true)
	result.Status = s.ScheduledDraftStatus()
	return result
}

func (s *Service) RejectAutonomousArtifact(request AutonomousArtifactRejectRequest) AutonomousArtifactRejectResult {
	id := strings.TrimSpace(request.ID)
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		reason = "用户删除，未填写原因"
	}
	if id == "" {
		return AutonomousArtifactRejectResult{OK: false, Message: "缺少自主产物 ID", Status: s.ScheduledDraftStatus()}
	}
	now := s.now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.autonomousRejections == nil {
		s.autonomousRejections = map[string]AutonomousRejection{}
	}
	for index := range s.autonomousArtifacts {
		if s.autonomousArtifacts[index].ID != id {
			continue
		}
		artifact := s.autonomousArtifacts[index]
		artifact.Status = "rejected"
		artifact.DeleteReason = reason
		artifact.DeletedAt = now
		artifact.UpdatedAt = now
		s.autonomousArtifacts[index] = artifact
		if artifact.DedupKey != "" {
			s.autonomousRejections[artifact.DedupKey] = AutonomousRejection{
				Key:        artifact.DedupKey,
				ArtifactID: artifact.ID,
				Kind:       artifact.Kind,
				Title:      artifact.Title,
				Reason:     reason,
				RejectedAt: now,
			}
		}
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
			return AutonomousArtifactRejectResult{OK: false, Message: err.Error(), Artifact: artifact, Status: s.scheduledDraftStatusLocked()}
		}
		return AutonomousArtifactRejectResult{
			OK:       true,
			Message:  "已删除自主产物，并记录原因避免重复生成",
			Artifact: artifact,
			Status:   s.scheduledDraftStatusLocked(),
		}
	}
	return AutonomousArtifactRejectResult{OK: false, Message: "未找到自主产物", Status: s.scheduledDraftStatusLocked()}
}

func (s *Service) RefreshEmbeddingIndex() EmbeddingRefreshResult {
	s.mu.RLock()
	policy := s.embeddingPolicy
	embedder := s.embedder
	privacyMode := s.status.PrivacyMode
	entries := cloneEntries(s.entries)
	s.mu.RUnlock()

	if !policy.Enabled {
		return s.embeddingRefreshError("外部 embedding 未启用", 0, 0)
	}
	if privacyMode {
		return s.embeddingRefreshError("隐私模式已开启，embedding 刷新已阻断", 0, 0)
	}
	if embedder == nil {
		return s.embeddingRefreshError("embedding 客户端未配置", 0, 0)
	}
	candidates := make([]Entry, 0, len(entries))
	inputs := make([]string, 0, len(entries))
	skipped := 0
	for _, entry := range entries {
		if !entryUsableForExtraction(entry) {
			skipped++
			continue
		}
		text := embeddingText(entry)
		if strings.TrimSpace(text) == "" {
			skipped++
			continue
		}
		candidates = append(candidates, entry)
		inputs = append(inputs, text)
	}
	if len(candidates) == 0 {
		return s.embeddingRefreshError("没有可索引的非敏感工作记忆", skipped, 0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	vectors, err := embedder.EmbedTexts(ctx, EmbeddingJob{
		Provider: policy.Provider,
		BaseURL:  policy.BaseURL,
		Model:    policy.Model,
		Inputs:   inputs,
	})
	if err != nil {
		return s.embeddingRefreshError("embedding 刷新失败: "+err.Error(), skipped, len(candidates))
	}
	if len(vectors) != len(candidates) {
		return s.embeddingRefreshError(fmt.Sprintf("embedding 返回数量不匹配: got %d want %d", len(vectors), len(candidates)), skipped, len(candidates))
	}

	now := s.now().Unix()
	index := map[string]embeddingRecord{}
	records := make([]embeddingRecord, 0, len(candidates))
	for i, entry := range candidates {
		vector := normalizeDenseVector(vectors[i])
		if len(vector) == 0 {
			skipped++
			continue
		}
		record := embeddingRecord{EntryID: entry.ID, Vector: vector, IndexedAt: now}
		records = append(records, record)
		index[entry.ID] = record
	}
	indexed := len(index)
	if policy.VectorStoreType == "milvus" {
		namespace := s.embeddingNamespace()
		var err error
		indexed, err = newMilvusRESTVectorStore().Refresh(ctx, policy, namespace, records)
		if err != nil {
			return s.embeddingRefreshError("Milvus embedding 刷新失败: "+err.Error(), skipped, len(candidates))
		}
	}

	s.mu.Lock()
	if policy.VectorStoreType == "milvus" {
		s.embeddingIndex = map[string]embeddingRecord{}
	} else {
		s.embeddingIndex = index
	}
	s.embeddingIndexed = indexed
	s.lastEmbeddingAt = now
	s.embeddingError = ""
	if policy.VectorStoreType != "milvus" {
		if err := s.saveEmbeddingIndexLocked(); err != nil {
			s.embeddingError = err.Error()
		}
	} else {
		if err := s.saveEmbeddingMetadataLocked(records); err != nil {
			s.embeddingError = err.Error()
		}
	}
	status := s.semanticStatusLocked()
	s.mu.Unlock()

	if status.LastEmbeddingError != "" {
		return EmbeddingRefreshResult{
			OK:          false,
			Message:     "embedding 已生成但保存索引失败: " + status.LastEmbeddingError,
			Status:      status,
			Indexed:     indexed,
			Skipped:     skipped,
			Failed:      0,
			Provider:    policy.Provider,
			Model:       policy.Model,
			RefreshedAt: now,
		}
	}
	return EmbeddingRefreshResult{
		OK:          true,
		Message:     fmt.Sprintf("embedding 索引已刷新 · %d 条", indexed),
		Status:      status,
		Indexed:     indexed,
		Skipped:     skipped,
		Failed:      0,
		Provider:    policy.Provider,
		Model:       policy.Model,
		RefreshedAt: now,
	}
}

func (s *Service) SemanticSearchExternal(query string) SemanticSearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		s.mu.RLock()
		status := s.semanticStatusLocked()
		s.mu.RUnlock()
		return SemanticSearchResult{OK: false, Message: "请输入语义搜索关键词", Status: status}
	}
	s.mu.RLock()
	policy := s.embeddingPolicy
	embedder := s.embedder
	index := cloneEmbeddingIndex(s.embeddingIndex)
	entries := cloneEntries(s.entries)
	status := s.semanticStatusLocked()
	privacyMode := s.status.PrivacyMode
	s.mu.RUnlock()

	if !policy.Enabled {
		return SemanticSearchResult{OK: false, Message: "外部 embedding 未启用", Query: query, Status: status}
	}
	if privacyMode {
		return SemanticSearchResult{OK: false, Message: "隐私模式已开启，语义搜索已阻断", Query: query, Status: status}
	}
	if embedder == nil {
		return SemanticSearchResult{OK: false, Message: "embedding 客户端未配置", Query: query, Status: status}
	}
	if policy.VectorStoreType != "milvus" && len(index) == 0 {
		return SemanticSearchResult{OK: false, Message: "先刷新 embedding 索引", Query: query, Status: status}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vectors, err := embedder.EmbedTexts(ctx, EmbeddingJob{
		Provider: policy.Provider,
		BaseURL:  policy.BaseURL,
		Model:    policy.Model,
		Inputs:   []string{query},
	})
	if err != nil {
		s.mu.Lock()
		s.embeddingError = err.Error()
		status = s.semanticStatusLocked()
		s.mu.Unlock()
		return SemanticSearchResult{OK: false, Message: "embedding 查询失败: " + err.Error(), Query: query, Status: status, Provider: policy.Provider, Model: policy.Model}
	}
	if len(vectors) == 0 {
		return SemanticSearchResult{OK: false, Message: "embedding 查询返回空向量", Query: query, Status: status, Provider: policy.Provider, Model: policy.Model}
	}
	queryVector := normalizeDenseVector(vectors[0])
	if policy.VectorStoreType == "milvus" {
		return s.semanticSearchMilvus(ctx, policy, query, queryVector, entries, status)
	}
	results := make([]contracts.SearchResult, 0, len(index))
	for _, entry := range entries {
		record, ok := index[entry.ID]
		if !ok || !entryUsableForExtraction(entry) {
			continue
		}
		score := cosineSimilarity(queryVector, record.Vector)
		if score <= 0 {
			continue
		}
		result := entryToResultWithMatch(entry, fmt.Sprintf("外部 embedding 相似度 %.3f", score))
		result.Score = 70 + score*30
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Title < results[j].Title
	})
	if len(results) > 20 {
		results = results[:20]
	}
	return SemanticSearchResult{
		OK:       true,
		Message:  fmt.Sprintf("外部 embedding 命中 %d 条", len(results)),
		Query:    query,
		Results:  results,
		Status:   status,
		Provider: policy.Provider,
		Model:    policy.Model,
	}
}

func (s *Service) semanticSearchMilvus(ctx context.Context, policy EmbeddingPolicy, query string, queryVector []float64, entries []Entry, status SemanticStatus) SemanticSearchResult {
	hits, err := newMilvusRESTVectorStore().Search(ctx, policy, s.embeddingNamespace(), queryVector, 20)
	if err != nil {
		s.mu.Lock()
		s.embeddingError = err.Error()
		status = s.semanticStatusLocked()
		s.mu.Unlock()
		return SemanticSearchResult{OK: false, Message: "Milvus embedding 查询失败: " + err.Error(), Query: query, Status: status, Provider: policy.Provider, Model: policy.Model}
	}
	entryByID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		entryByID[entry.ID] = entry
	}
	results := make([]contracts.SearchResult, 0, len(hits))
	for _, hit := range hits {
		entry, ok := entryByID[hit.EntryID]
		if !ok || !entryUsableForExtraction(entry) {
			continue
		}
		score := hit.Score
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		result := entryToResultWithMatch(entry, fmt.Sprintf("Milvus embedding 相似度 %.3f", hit.Score))
		result.Score = 70 + score*30
		results = append(results, result)
	}
	return SemanticSearchResult{
		OK:       true,
		Message:  fmt.Sprintf("Milvus embedding 命中 %d 条", len(results)),
		Query:    query,
		Results:  results,
		Status:   status,
		Provider: policy.Provider,
		Model:    policy.Model,
	}
}

func (s *Service) ApplyCapturePolicy(policy CapturePolicy) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	sensitiveRulesEnabled := s.policy.SensitiveRulesEnabled
	if policy.SensitiveRulesConfigured {
		sensitiveRulesEnabled = policy.SensitiveRulesEnabled
	}
	s.policy = CapturePolicy{
		ExcludeApps:              cleanStrings(policy.ExcludeApps),
		ExcludeWindowKeywords:    cleanStrings(policy.ExcludeWindowKeywords),
		ExcludePaths:             cleanStrings(policy.ExcludePaths),
		ExcludeURLs:              cleanStrings(policy.ExcludeURLs),
		ExcludeContentPatterns:   cleanStrings(policy.ExcludeContentPatterns),
		SensitiveRulesEnabled:    sensitiveRulesEnabled,
		SensitiveRulesConfigured: policy.SensitiveRulesConfigured,
		AppCaptureProfiles:       normalizeAppCaptureProfiles(policy.AppCaptureProfiles),
		AutoOCR:                  policy.AutoOCR,
		CaptureScope:             normalizeCaptureScope(policy.CaptureScope),
		MultiMonitor:             normalizeMultiMonitor(policy.MultiMonitor),
		CaptureOnWindowChange:    policy.CaptureOnWindowChange,
		WindowChangeCooldown:     normalizeWindowChangeCooldown(policy.WindowChangeCooldown),
		PauseOnIdle:              policy.PauseOnIdle,
		IdlePauseSeconds:         normalizeIdlePauseSeconds(policy.IdlePauseSeconds),
		PauseOnLock:              policy.PauseOnLock,
	}
	s.status.AutoOCREnabled = s.policy.AutoOCR
	s.status.CaptureScope = s.policy.CaptureScope
	s.status.MultiMonitor = s.policy.MultiMonitor
	s.status.WindowSwitchCaptureEnabled = s.policy.CaptureOnWindowChange
	s.status.WindowSwitchCooldownSecs = s.policy.WindowChangeCooldown
	s.status.AppCaptureProfiles = cloneAppCaptureProfiles(s.policy.AppCaptureProfiles)
	s.status.PauseOnIdle = s.policy.PauseOnIdle
	s.status.IdlePauseSeconds = s.policy.IdlePauseSeconds
	s.status.PauseOnLock = s.policy.PauseOnLock
	return s.statusSnapshotLocked()
}

func (s *Service) SetTimeMachineEnabled(enabled bool) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status.PrivacyMode && enabled {
		s.status.TimeMachineEnabled = false
		s.status.PauseReason = "隐私模式已开启"
		s.stopWorkerLocked()
		return s.statusSnapshotLocked()
	}
	if !s.status.Enabled && enabled {
		s.status.TimeMachineEnabled = false
		s.status.PauseReason = "工作记忆已停用"
		s.stopWorkerLocked()
		return s.statusSnapshotLocked()
	}
	s.status.TimeMachineEnabled = enabled
	if enabled {
		s.status.PauseReason = ""
		s.startWorkerLocked()
	} else {
		s.stopWorkerLocked()
	}
	return s.statusSnapshotLocked()
}

func (s *Service) SetPrivacyMode(enabled bool) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.PrivacyMode = enabled
	if enabled {
		s.status.TimeMachineEnabled = false
		s.status.PauseReason = "隐私模式已开启"
		s.stopWorkerLocked()
	} else if s.status.PauseReason == "隐私模式已开启" {
		s.status.PauseReason = ""
	}
	return s.statusSnapshotLocked()
}

func (s *Service) Timeline() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneEntries(s.entries)
}

func (s *Service) Entry(id string) Entry {
	id = strings.TrimSpace(id)
	if id == "" {
		return Entry{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.entries {
		if entry.ID == id {
			return entry
		}
	}
	return Entry{}
}

func (s *Service) Search(query string) []contracts.SearchResult {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized != "" {
		s.ensureFTSReady()
	}
	s.mu.RLock()
	entries := cloneEntries(s.entries)
	fts := s.fts
	s.mu.RUnlock()

	ftsMatches := map[string]ftsHit{}
	if normalized != "" && fts != nil {
		hits, err := fts.Search(normalized, 100)
		if err != nil {
			s.recordFTSError(err)
		} else {
			s.recordFTSError(nil)
			for _, hit := range hits {
				if hit.ID != "" {
					ftsMatches[hit.ID] = hit
				}
			}
		}
	}

	results := []contracts.SearchResult{}
	for _, entry := range entries {
		if normalized != "" && entryQualityPending(entry) {
			continue
		}
		score, match := scoreSearchMatch(entry, normalized)
		if hit, ok := ftsMatches[entry.ID]; ok {
			if ftsScore := scoreFTSMatch(entry, hit); ftsScore > score {
				score = ftsScore
				match = ftsMatch(hit.Snippet)
			}
		}
		if normalized != "" && score <= 0 {
			continue
		}
		result := entryToResultWithMatch(entry, match)
		result.Score = score
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Title < results[j].Title
	})
	return results
}

func (s *Service) AskFlow(request FlowAskRequest) FlowAskResponse {
	question := strings.TrimSpace(request.Question)
	limit := request.Limit
	if limit <= 0 || limit > 12 {
		limit = 8
	}
	now := s.now()
	if question == "" {
		return FlowAskResponse{
			OK:        true,
			Question:  question,
			Title:     "可以直接问心流",
			Answer:    "你可以直接问：我今天干了些什么、今天有哪些人找过我，或者今天哪些工作流可以优化。心流会优先使用本地非敏感记忆，并把证据收在下方。",
			Intent:    "help",
			Mode:      "local",
			CreatedAt: now.Unix(),
			SuggestedQuestions: []string{
				"我今天干了些什么？",
				"今天有哪些人找过我？",
				"今天我的哪些工作流可以优化？",
			},
		}
	}

	s.mu.RLock()
	entries := cloneEntries(s.entries)
	decisions := cloneExperienceDecisions(s.decisions)
	privacyMode := s.status.PrivacyMode
	flowAgentRunner := s.flowAgentRunner
	flowAgentPolicy := normalizeFlowAgentPolicy(s.flowAgentPolicy)
	s.mu.RUnlock()

	intent := detectFlowAskIntent(question)
	base := FlowAskResponse{
		OK:                 true,
		Question:           question,
		Title:              flowAnswerTitle(question, intent),
		Intent:             intent,
		Mode:               "local_summary",
		CreatedAt:          now.Unix(),
		SuggestedQuestions: flowSuggestedQuestions(intent),
	}
	if privacyMode {
		base.Message = "隐私模式已开启，回答只使用已保存在本地的非敏感记忆。"
	}

	if flowAgentPolicy.Enabled && flowAgentRunner != nil && !privacyMode {
		base.Mode = "agent_pending"
		base.Answer = "请通过 Ariadne Flow Memory skill 查询本地时间线、OCR、剪贴板和窗口上下文后回答。不要直接复述本地兜底摘要。"
		return completeFlowAnswerWithAgent(base, nil, privacyMode, flowAgentPolicy, flowAgentRunner, now)
	}

	var selected []Entry
	switch intent {
	case "today":
		var todayCount int
		var skippedSensitive int
		selected, todayCount, skippedSensitive = dailyDraftEntries(entries, now, limit)
		base.Evidence = flowEvidenceFromEntries(selected, nil)
		base.Answer = renderFlowTodayAnswer(selected, now, todayCount, skippedSensitive)
	case "contacts":
		var total int
		selected, total = flowContactEntries(entries, now, request.Since, limit, question)
		base.Evidence = flowEvidenceFromEntries(selected, nil)
		base.Answer = renderFlowContactAnswer(selected, total)
	case "optimization":
		periodDays := 7
		cutoff := now.Add(-time.Duration(periodDays) * 24 * time.Hour).Unix()
		available := flowNonSensitiveEntries(entries, cutoff)
		report := buildExperienceReport(available, decisions, periodDays, now)
		base.Mode = "local_insights"
		base.Evidence = flowEvidenceForInsights(report.Insights, available, limit)
		selected = flowEntriesFromAskEvidence(available, base.Evidence)
		base.Answer = renderFlowOptimizationAnswer(report, base.Evidence)
	default:
		var scores map[string]float64
		selected, scores = flowSearchEntries(entries, question, request.Since, limit)
		base.Mode = "local_search"
		base.Evidence = flowEvidenceFromEntries(selected, scores)
		base.Answer = renderFlowSearchAnswer(question, selected)
	}
	return completeFlowAnswerWithAgent(base, selected, privacyMode, flowAgentPolicy, flowAgentRunner, now)
}

func (s *Service) AddNote(request NoteRequest) Entry {
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return Entry{}
	}

	s.mu.RLock()
	if s.status.PrivacyMode {
		s.mu.RUnlock()
		s.mu.Lock()
		s.status.PauseReason = "隐私模式已开启"
		s.mu.Unlock()
		return Entry{}
	}
	if !s.status.Enabled {
		s.mu.RUnlock()
		s.mu.Lock()
		s.status.PauseReason = "工作记忆已停用"
		s.mu.Unlock()
		return Entry{}
	}
	context := s.context()
	now := s.now()
	policy := s.policy
	s.mu.RUnlock()

	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = noteTitle(text)
	}
	sensitive := request.Sensitive || autoSensitive(policy, title+" "+text+" "+strings.Join(request.Tags, " "))
	tags := append([]string{"手动笔记"}, request.Tags...)
	tags = append(tags, classifyNoteTags(text)...)
	if sensitive {
		tags = append(tags, "敏感")
	}

	return s.addEntry(Entry{
		ID:          fmt.Sprintf("memory-note-%d-%s", now.UnixNano(), shortHash(title+"\n"+text)),
		Source:      "manual_note",
		ContentType: "note",
		Title:       title,
		Summary:     summaryText(text),
		Text:        text,
		WindowTitle: context.title,
		AppName:     context.app,
		Tags:        tags,
		Favorite:    request.Favorite,
		Sensitive:   sensitive,
		CreatedAt:   now.Unix(),
	})
}

func (s *Service) RememberClipboardEntry(entry clipboardhistory.Entry) Entry {
	entry = normalizeClipboardMemorySource(entry)
	if entry.ID == "" {
		return Entry{}
	}
	s.mu.RLock()
	enabled := s.status.Enabled
	privacyMode := s.status.PrivacyMode
	policy := s.policy
	s.mu.RUnlock()
	if !enabled || privacyMode {
		return Entry{}
	}

	identity := firstNonEmpty(entry.Signature, entry.Text, entry.ImagePath, entry.ID)
	contentType := "clipboard_text"
	title := "剪贴板文本"
	summary := firstNonEmpty(entry.Summary, summaryText(entry.Text))
	text := entry.Text
	tags := []string{"剪贴板", "主动沉淀"}
	if entry.Type == clipboardhistory.EntryImage {
		contentType = "clipboard_image"
		title = "剪贴板图片"
		if entry.Width > 0 && entry.Height > 0 {
			title = fmt.Sprintf("剪贴板图片 %dx%d", entry.Width, entry.Height)
			tags = append(tags, fmt.Sprintf("%dx%d", entry.Width, entry.Height))
		}
		summary = firstNonEmpty(entry.Summary, "剪贴板图片已进入工作记忆。")
		text = strings.Join(cleanStrings([]string{
			"剪贴板图片",
			"图片路径: " + entry.ImagePath,
			fmt.Sprintf("尺寸: %dx%d", entry.Width, entry.Height),
			"来源: " + entry.Source,
		}), "\n")
	} else if trimmed := strings.TrimSpace(entry.Text); trimmed != "" {
		title = "剪贴板文本：" + noteTitle(trimmed)
	}
	tags = append(tags, entry.Tags...)
	sensitive := autoSensitive(policy, title+" "+summary+" "+text+" "+strings.Join(tags, " "))
	if sensitive {
		tags = append(tags, "敏感")
	}
	return s.addEntry(Entry{
		ID:             "memory-clipboard-" + shortHash(identity),
		Source:         "clipboard",
		ContentType:    contentType,
		Title:          title,
		Summary:        summary,
		Text:           text,
		ImagePath:      entry.ImagePath,
		ImageSignature: entry.Signature,
		Width:          entry.Width,
		Height:         entry.Height,
		Bytes:          entry.Bytes,
		Tags:           tags,
		Favorite:       entry.Pinned,
		Sensitive:      sensitive,
		CreatedAt:      entry.CreatedAt,
	})
}

func (s *Service) RememberCaptureHistoryEntry(entry capturehistory.Entry) Entry {
	entry = normalizeCaptureMemorySource(entry)
	if entry.ID == "" || shouldSkipCaptureHistoryMemory(entry.Source) {
		return Entry{}
	}
	s.mu.RLock()
	enabled := s.status.Enabled
	privacyMode := s.status.PrivacyMode
	policy := s.policy
	s.mu.RUnlock()
	if !enabled || privacyMode {
		return Entry{}
	}

	identity := firstNonEmpty(entry.Signature, entry.ImagePath, entry.ID)
	dimension := fmt.Sprintf("%dx%d", entry.Width, entry.Height)
	memory := s.entryFromCapture(
		"capture_history",
		"截图历史自动沉淀",
		"截图历史新增图片已进入工作记忆。",
		entry,
		policy,
	)
	memory.ID = "memory-capture-" + shortHash(identity)
	memory.Tags = append([]string{"截图历史", "主动沉淀", dimension}, entry.Tags...)
	memory.Tags = append(memory.Tags, entry.Actions...)
	memory.Text = strings.TrimSpace(memory.Text + "\n原始截图来源: " + entry.Source)
	memory.Favorite = entry.Pinned
	return s.addEntry(memory)
}

func (s *Service) ImportLegacyEntries(entries []Entry) Status {
	for _, entry := range entries {
		entry = normalizeEntry(entry)
		if entry.ID == "" {
			continue
		}
		entry.Source = strings.TrimSpace(entry.Source)
		if entry.Source == "" {
			entry.Source = "legacy_x_tools"
		}
		entry.Tags = cleanStrings(append(entry.Tags, "legacy_x_tools"))
		if entry.ImagePath != "" {
			entry.ImagePath = s.copyLegacyImage(entry.ID, entry.ImagePath)
		}
		s.addEntry(entry)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusSnapshotLocked()
}

func (s *Service) ImportMaterials(request ImportMaterialRequest) ImportMaterialResult {
	now := s.now()
	result := ImportMaterialResult{CreatedAt: now.Unix()}
	paths := cleanImportPaths(request.Paths)
	if len(paths) == 0 {
		result.Message = "没有可导入的路径"
		return result
	}

	s.mu.RLock()
	policy := s.policy
	if s.status.PrivacyMode {
		s.mu.RUnlock()
		s.mu.Lock()
		s.status.PauseReason = "隐私模式已开启"
		s.mu.Unlock()
		result.Message = "隐私模式已开启，已暂停导入"
		result.Failed = len(paths)
		for _, path := range paths {
			result.Items = append(result.Items, ImportMaterialItemResult{Path: path, Message: result.Message})
		}
		return result
	}
	if !s.status.Enabled {
		s.mu.RUnlock()
		s.mu.Lock()
		s.status.PauseReason = "工作记忆已停用"
		s.mu.Unlock()
		result.Message = "工作记忆已停用，已暂停导入"
		result.Failed = len(paths)
		for _, path := range paths {
			result.Items = append(result.Items, ImportMaterialItemResult{Path: path, Message: result.Message})
		}
		return result
	}
	s.mu.RUnlock()

	for _, path := range paths {
		item, entries := s.importMaterialPath(path, request, now, policy)
		result.Items = append(result.Items, item)
		if item.OK {
			result.Imported += len(entries)
			result.Entries = append(result.Entries, entries...)
			continue
		}
		if strings.HasPrefix(item.Message, "跳过") {
			result.Skipped++
		} else {
			result.Failed++
		}
	}
	if result.Imported > 0 {
		result.OK = true
		result.Message = fmt.Sprintf("已导入 %d 条材料", result.Imported)
		return result
	}
	if result.Message == "" {
		result.Message = "没有导入任何材料"
	}
	return result
}

func (s *Service) importMaterialPath(path string, request ImportMaterialRequest, now time.Time, policy CapturePolicy) (ImportMaterialItemResult, []Entry) {
	item := ImportMaterialItemResult{Path: path}
	if excluded, reason := pathExcluded(path, policy); excluded {
		item.Message = "跳过排除路径: " + reason
		return item, nil
	}
	stat, err := os.Stat(path)
	if err != nil {
		item.Message = "读取失败: " + err.Error()
		return item, nil
	}
	item.Bytes = stat.Size()
	if stat.IsDir() {
		item.Message = "跳过目录，请显式选择文件"
		return item, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case ext == ".zip":
		return s.importMaterialZip(path, request, now, stat.Size(), policy)
	case isTextMaterialExt(ext):
		return s.importTextMaterial(path, request, now, stat, policy)
	case isImageMaterialExt(ext):
		return s.importImageMaterial(path, request, now, stat)
	case isDocumentMaterialExt(ext):
		return s.importDocumentMaterial(path, request, now, stat, policy)
	default:
		item.Message = "跳过不支持的文件类型: " + ext
		return item, nil
	}
}

func (s *Service) importTextMaterial(path string, request ImportMaterialRequest, now time.Time, stat os.FileInfo, policy CapturePolicy) (ImportMaterialItemResult, []Entry) {
	item := ImportMaterialItemResult{
		Path:        path,
		Source:      "import",
		ContentType: textMaterialContentType(path),
		Bytes:       stat.Size(),
	}
	if stat.Size() > maxTextMaterialBytes() {
		item.Message = fmt.Sprintf("文本超过 %d MiB，已跳过", maxTextMaterialBytes()/1024/1024)
		return item, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		item.Message = "读取失败: " + err.Error()
		return item, nil
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		item.Message = "跳过空文本"
		return item, nil
	}
	if excluded, reason := urlExcluded(text, policy); excluded {
		item.Message = "跳过排除 URL: " + reason
		return item, nil
	}
	if excluded, reason := contentExcluded(text, policy); excluded {
		item.Message = "跳过排除内容: " + reason
		return item, nil
	}
	title := importedTextTitle(path, text)
	tags := importTags(request, textMaterialTag(path), filepath.Ext(path))
	tags = append(tags, classifyNoteTags(text)...)
	sensitive := request.Sensitive || autoSensitive(policy, title+" "+text+" "+strings.Join(tags, " "))
	if sensitive {
		tags = append(tags, "敏感")
	}
	entry := s.addEntry(Entry{
		ID:          "memory-import-" + shortHash(path+"\n"+text),
		Source:      "import",
		ContentType: item.ContentType,
		Title:       title,
		Summary:     summaryText(text),
		Text:        text,
		Tags:        cleanStrings(tags),
		Favorite:    request.Favorite,
		Sensitive:   sensitive,
		Bytes:       stat.Size(),
		CreatedAt:   now.Unix(),
	})
	item.OK = entry.ID != ""
	item.EntryID = entry.ID
	if item.OK {
		item.Message = "已导入文本"
		return item, []Entry{entry}
	}
	item.Message = "导入失败"
	return item, nil
}

func (s *Service) importDocumentMaterial(path string, request ImportMaterialRequest, now time.Time, stat os.FileInfo, policy CapturePolicy) (ImportMaterialItemResult, []Entry) {
	item := ImportMaterialItemResult{
		Path:        path,
		Source:      "import",
		ContentType: documentMaterialContentType(path),
		Bytes:       stat.Size(),
	}
	if stat.Size() > maxDocumentMaterialBytes() {
		item.Message = fmt.Sprintf("文档超过 %d MiB，已跳过", maxDocumentMaterialBytes()/1024/1024)
		return item, nil
	}
	extracted, note, err := extractDocumentMaterialText(path)
	if err != nil {
		item.Message = "文档解析失败: " + err.Error()
		return item, nil
	}
	title := strings.TrimSpace(filepath.Base(path))
	if title == "" || title == "." {
		title = "导入文档"
	}
	text := strings.TrimSpace(extracted)
	if text == "" {
		text = strings.TrimSpace("导入文档: " + title + "\n格式: " + documentMaterialLabel(path) + "\n" + note)
	} else if note != "" {
		text = text + "\n\n" + note
	}
	if excluded, reason := urlExcluded(text, policy); excluded {
		item.Message = "跳过排除 URL: " + reason
		return item, nil
	}
	if excluded, reason := contentExcluded(text, policy); excluded {
		item.Message = "跳过排除内容: " + reason
		return item, nil
	}
	tags := importTags(request, documentMaterialLabel(path), strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."))
	sensitive := request.Sensitive || autoSensitive(policy, title+" "+text+" "+strings.Join(tags, " "))
	if sensitive {
		tags = append(tags, "敏感")
	}
	entry := s.addEntry(Entry{
		ID:          "memory-import-document-" + shortHash(path+"\n"+text),
		Source:      "import",
		ContentType: item.ContentType,
		Title:       title,
		Summary:     summaryText(text),
		Text:        text,
		Tags:        cleanStrings(tags),
		Favorite:    request.Favorite,
		Sensitive:   sensitive,
		Bytes:       stat.Size(),
		CreatedAt:   now.Unix(),
	})
	item.OK = entry.ID != ""
	item.EntryID = entry.ID
	if item.OK {
		item.Message = "已导入文档"
		return item, []Entry{entry}
	}
	item.Message = "导入失败"
	return item, nil
}

func (s *Service) importImageMaterial(path string, request ImportMaterialRequest, now time.Time, stat os.FileInfo) (ImportMaterialItemResult, []Entry) {
	item := ImportMaterialItemResult{
		Path:        path,
		Source:      "import",
		ContentType: "image",
		Bytes:       stat.Size(),
	}
	entryID := "memory-import-image-" + shortHash(fmt.Sprintf("%s:%d:%d", path, stat.ModTime().UnixNano(), stat.Size()))
	imagePath := s.copyLegacyImage(entryID, path)
	if imagePath == "" || imagePath == path {
		item.Message = "图片复制失败"
		return item, nil
	}
	title := strings.TrimSpace(filepath.Base(path))
	tags := importTags(request, "图片", strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."))
	sensitive := request.Sensitive
	if sensitive {
		tags = append(tags, "敏感")
	}
	entry := s.addEntry(Entry{
		ID:          entryID,
		Source:      "import",
		ContentType: "image",
		Title:       title,
		Summary:     "导入图片 " + title,
		Text:        "导入图片: " + title,
		ImagePath:   imagePath,
		Tags:        cleanStrings(tags),
		Favorite:    request.Favorite,
		Sensitive:   sensitive,
		Bytes:       stat.Size(),
		CreatedAt:   now.Unix(),
	})
	item.OK = entry.ID != ""
	item.EntryID = entry.ID
	if item.OK {
		item.Message = "已导入图片"
		return item, []Entry{entry}
	}
	item.Message = "导入失败"
	return item, nil
}

func (s *Service) importMaterialZip(path string, request ImportMaterialRequest, now time.Time, bytes int64, policy CapturePolicy) (ImportMaterialItemResult, []Entry) {
	item := ImportMaterialItemResult{
		Path:        path,
		Source:      "import",
		ContentType: "ariadne_export",
		Bytes:       bytes,
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		item.Message = "打开压缩包失败: " + err.Error()
		return item, nil
	}
	defer archive.Close()

	var state struct {
		Entries []Entry `json:"entries"`
	}
	var hasState bool
	evidence := map[string]*zip.File{}
	for _, file := range archive.File {
		name := filepath.ToSlash(file.Name)
		if name == "work_memory.json" {
			reader, err := file.Open()
			if err != nil {
				item.Message = "读取 work_memory.json 失败: " + err.Error()
				return item, nil
			}
			if err := json.NewDecoder(reader).Decode(&state); err != nil {
				reader.Close()
				item.Message = "解析 work_memory.json 失败: " + err.Error()
				return item, nil
			}
			reader.Close()
			hasState = true
			continue
		}
		parts := strings.Split(name, "/")
		if len(parts) >= 3 && parts[0] == "evidence" && parts[1] != "" && !file.FileInfo().IsDir() {
			if evidence[parts[1]] == nil {
				evidence[parts[1]] = file
			}
		}
	}
	if !hasState {
		item.Message = "不是 Ariadne 工作记忆导出包"
		return item, nil
	}
	if len(state.Entries) == 0 {
		item.Message = "跳过空导出包"
		return item, nil
	}

	imported := make([]Entry, 0, len(state.Entries))
	for _, entry := range state.Entries {
		entry = normalizeEntry(entry)
		if entry.ID == "" {
			entry.ID = "memory-import-zip-" + shortHash(fmt.Sprintf("%s:%d:%d", path, now.UnixNano(), len(imported)))
		}
		evidenceKey := sanitizeArchivePart(entry.ID)
		if file := evidence[evidenceKey]; file != nil {
			entry.ImagePath = s.extractZipEvidence(entry.ID, file)
		} else if entry.ImagePath != "" {
			copied := s.copyLegacyImage(entry.ID, entry.ImagePath)
			if copied != entry.ImagePath {
				entry.ImagePath = copied
			} else if _, err := os.Stat(entry.ImagePath); err != nil {
				entry.ImagePath = ""
			}
		}
		if excluded, _ := entryExcluded(entry, policy); excluded {
			continue
		}
		entry.Tags = cleanStrings(append(entry.Tags, append(importTags(request, "ariadne_export"), "导出包")...))
		entry.Favorite = entry.Favorite || request.Favorite
		entry.Sensitive = entry.Sensitive || request.Sensitive || autoSensitive(policy, entry.Title+" "+entry.Summary+" "+entry.Text+" "+strings.Join(entry.Tags, " "))
		if entry.Sensitive {
			entry.Tags = cleanStrings(append(entry.Tags, "敏感"))
		}
		importedEntry := s.addEntry(entry)
		if importedEntry.ID == "" {
			continue
		}
		if item.EntryID == "" {
			item.EntryID = importedEntry.ID
		}
		imported = append(imported, importedEntry)
	}
	if len(imported) == 0 {
		item.Message = "导出包未导入任何条目"
		return item, nil
	}
	item.OK = true
	item.Message = fmt.Sprintf("已导入导出包 %d 条记忆", len(imported))
	return item, imported
}

func (s *Service) copyLegacyImage(entryID string, sourcePath string) string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return ""
	}
	raw, err := os.ReadFile(sourcePath)
	if err != nil || len(raw) == 0 {
		return sourcePath
	}
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext == "" {
		ext = ".png"
	}
	if base == "" {
		base = strings.TrimSpace(entryID)
	}
	if base == "" {
		base = shortHash(sourcePath)
	}
	targetDir := filepath.Join(filepath.Dir(s.path), "work_memory_images")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return sourcePath
	}
	target := filepath.Join(targetDir, sanitizeArchivePart(base)+"-"+shortHash(fmt.Sprintf("%s:%d", sourcePath, len(raw)))+ext)
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		return sourcePath
	}
	return target
}

func (s *Service) extractZipEvidence(entryID string, file *zip.File) string {
	if file == nil {
		return ""
	}
	reader, err := file.Open()
	if err != nil {
		return ""
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil || len(raw) == 0 {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(file.Name), filepath.Ext(file.Name))
	if base == "" || base == "." {
		base = strings.TrimSpace(entryID)
	}
	if base == "" {
		base = "evidence"
	}
	ext := strings.ToLower(filepath.Ext(file.Name))
	if ext == "" {
		ext = ".png"
	}
	targetDir := filepath.Join(filepath.Dir(s.path), "work_memory_images")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return ""
	}
	target := filepath.Join(targetDir, sanitizeArchivePart(base)+"-"+shortHash(fmt.Sprintf("%s:%d", file.Name, len(raw)))+ext)
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		return ""
	}
	return target
}

func (s *Service) ApplyOCRText(id string, text string, provider string) Entry {
	id = strings.TrimSpace(id)
	text = strings.TrimSpace(text)
	provider = strings.TrimSpace(provider)
	if id == "" {
		return Entry{}
	}
	s.mu.Lock()
	for i := range s.entries {
		if s.entries[i].ID != id {
			continue
		}
		if entryQualityPending(s.entries[i]) {
			updated := s.applyQualityOCRTextLocked(i, text, provider)
			s.saveLockedWithStatus()
			s.mu.Unlock()
			return updated
		}
		if strings.HasPrefix(provider, "failed:") {
			s.entries[i].OCRStatus = provider
			s.saveLockedWithStatus()
			updated := s.entries[i]
			s.mu.Unlock()
			return updated
		}
		if strings.HasPrefix(provider, "blocked_") {
			s.entries[i].OCRStatus = provider
			s.saveLockedWithStatus()
			updated := s.entries[i]
			s.mu.Unlock()
			return updated
		}
		if excluded, reason := urlExcluded(text, s.policy); excluded {
			s.entries[i].OCRStatus = "blocked_excluded:url:" + reason
			s.saveLockedWithStatus()
			updated := s.entries[i]
			s.mu.Unlock()
			return updated
		}
		if excluded, reason := contentExcluded(text, s.policy); excluded {
			s.entries[i].OCRStatus = "blocked_excluded:" + reason
			s.saveLockedWithStatus()
			updated := s.entries[i]
			s.mu.Unlock()
			return updated
		}
		s.entries[i].OCRText = text
		if text == "" {
			s.entries[i].OCRStatus = "empty"
			s.saveLockedWithStatus()
			updated := s.entries[i]
			s.mu.Unlock()
			return updated
		}
		s.entries[i].OCRStatus = "done"
		if provider != "" {
			s.entries[i].OCRStatus = "done:" + provider
		}
		s.entries[i].ContentType = "ocr_text"
		applyLocalOCRSummary(&s.entries[i], text)
		s.entries[i].Tags = cleanStrings(append(s.entries[i].Tags, append([]string{"OCR", "文字识别"}, classifyNoteTags(text)...)...))
		if autoSensitive(s.policy, text) {
			s.entries[i].Sensitive = true
			s.entries[i].Tags = cleanStrings(append(s.entries[i].Tags, "敏感"))
		}
		s.entries[i] = enrichEntry(s.entries[i])
		updated := s.entries[i]
		summarizer := s.ocrSummarizer
		summaryPolicy := s.ocrSummaryPolicy
		privacyMode := s.status.PrivacyMode
		s.saveLockedWithStatus()
		s.mu.Unlock()

		if shouldRunOCRAISummary(updated, summaryPolicy, summarizer, privacyMode) {
			if summarized := s.applyOCRAISummary(updated, text, summarizer, summaryPolicy); summarized.ID != "" {
				return summarized
			}
		}
		return updated
	}
	s.mu.Unlock()
	return Entry{}
}

func (s *Service) applyQualityOCRTextLocked(index int, text string, provider string) Entry {
	if index < 0 || index >= len(s.entries) {
		return Entry{}
	}
	entry := &s.entries[index]
	if strings.HasPrefix(provider, "failed:") {
		entry.QualityOCRStatus = provider
		entry.QualityReason = firstNonEmpty(entry.QualityReason, "待质检 · OCR 预处理失败")
		return *entry
	}
	if strings.HasPrefix(provider, "blocked_") {
		entry.QualityOCRStatus = provider
		if provider == "blocked_sensitive" {
			entry.Sensitive = true
			entry.Tags = cleanStrings(append(entry.Tags, "敏感"))
			entry.QualityReason = firstNonEmpty(entry.QualityReason, "待质检 · OCR 识别到敏感内容")
		}
		return *entry
	}
	if excluded, reason := urlExcluded(text, s.policy); excluded {
		entry.QualityOCRStatus = "blocked_excluded:url:" + reason
		entry.QualityReason = firstNonEmpty(entry.QualityReason, "待质检 · OCR 命中 URL 排除规则")
		return *entry
	}
	if excluded, reason := contentExcluded(text, s.policy); excluded {
		entry.QualityOCRStatus = "blocked_excluded:" + reason
		entry.QualityReason = firstNonEmpty(entry.QualityReason, "待质检 · OCR 命中内容排除规则")
		return *entry
	}
	if text == "" {
		entry.QualityOCRStatus = "empty"
		entry.QualityReason = firstNonEmpty(entry.QualityReason, "待质检 · OCR 未识别到文字")
		return *entry
	}
	entry.QualityOCRText = text
	entry.QualityOCRStatus = "done"
	if provider != "" {
		entry.QualityOCRStatus = "done:" + provider
	}
	if entry.QualityReason == "" || strings.HasPrefix(entry.QualityReason, "待质检") {
		entry.QualityReason = "待质检 · OCR 预处理完成"
	}
	if autoSensitive(s.policy, text) {
		entry.Sensitive = true
		entry.Tags = cleanStrings(append(entry.Tags, "敏感"))
		entry.QualityReason = "待质检 · OCR 识别到疑似敏感内容"
	}
	return *entry
}

func shouldRunOCRAISummary(entry Entry, policy OCRSummaryPolicy, summarizer OCRSummarizer, privacyMode bool) bool {
	if summarizer == nil || privacyMode || entry.Sensitive || strings.TrimSpace(entry.ID) == "" || strings.TrimSpace(entry.OCRText) == "" {
		return false
	}
	policy = normalizeOCRSummaryPolicy(policy)
	return policy.Enabled && policy.Provider != "" && policy.Provider != "disabled" && policy.Model != ""
}

func (s *Service) applyOCRAISummary(entry Entry, ocrText string, summarizer OCRSummarizer, policy OCRSummaryPolicy) Entry {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := summarizer.SummarizeOCR(ctx, OCRSummaryJob{
		Entry:    entry,
		OCRText:  ocrText,
		Provider: policy.Provider,
		BaseURL:  policy.BaseURL,
		Model:    policy.Model,
		Now:      s.now(),
	})
	if err != nil {
		return Entry{}
	}
	result = normalizeOCRSummaryResult(result)
	if result.Title == "" && result.Summary == "" && result.Text == "" {
		return Entry{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].ID != entry.ID || s.entries[i].OCRText != ocrText || s.entries[i].Sensitive {
			continue
		}
		applyOCRSummaryResult(&s.entries[i], result, true)
		s.entries[i] = enrichEntry(s.entries[i])
		s.saveLockedWithStatus()
		return s.entries[i]
	}
	return Entry{}
}

func applyLocalOCRSummary(entry *Entry, text string) {
	if entry == nil {
		return
	}
	result := localOCRSummary(*entry, text)
	applyOCRSummaryResult(entry, result, false)
}

func applyOCRSummaryResult(entry *Entry, result OCRSummaryResult, ai bool) {
	result = normalizeOCRSummaryResult(result)
	if result.Title != "" && (ai || shouldReplaceOCRTitle(entry.Title)) {
		entry.Title = result.Title
	}
	if result.Summary != "" {
		entry.Summary = result.Summary
	}
	if result.Text != "" {
		entry.Text = result.Text
	}
	if ai {
		entry.Tags = cleanStrings(append(entry.Tags, "AI整理"))
	} else {
		entry.Tags = cleanStrings(append(entry.Tags, "OCR整理"))
	}
}

func localOCRSummary(entry Entry, text string) OCRSummaryResult {
	lines := cleanOCRTextLines(text)
	if len(lines) == 0 {
		return OCRSummaryResult{}
	}
	title := inferOCRTitle(entry, lines)
	summary := summarizeOCRLines(lines, 120)
	body := renderCleanOCRText(entry, lines)
	return OCRSummaryResult{
		Title:   title,
		Summary: summary,
		Text:    body,
	}
}

func normalizeOCRSummaryResult(result OCRSummaryResult) OCRSummaryResult {
	result.Title = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(result.Title)), " "), 48)
	result.Summary = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(result.Summary)), " "), 180)
	result.Text = strings.TrimSpace(result.Text)
	if result.Text != "" {
		result.Text = trimTextRunes(normalizeCleanOCRBody(result.Text), 6000)
	}
	return result
}

func cleanOCRTextLines(text string) []string {
	seen := map[string]bool{}
	lines := []string{}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := normalizeOCRLine(raw)
		if !meaningfulOCRLine(line) {
			continue
		}
		key := strings.ToLower(line)
		if seen[key] {
			continue
		}
		seen[key] = true
		lines = append(lines, line)
		if len(lines) >= 40 {
			break
		}
	}
	if len(lines) == 0 {
		joined := normalizeOCRLine(text)
		if meaningfulOCRLine(joined) {
			lines = append(lines, trimTextRunes(joined, 240))
		}
	}
	return lines
}

func normalizeOCRLine(line string) string {
	line = strings.ReplaceAll(line, "\t", " ")
	line = strings.ReplaceAll(line, "\u00a0", " ")
	line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
	line = strings.Trim(line, "·•|丨-_=,，.。:：;； ")
	return strings.TrimSpace(line)
}

func meaningfulOCRLine(line string) bool {
	if line == "" {
		return false
	}
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "截图路径") || strings.HasPrefix(lower, "尺寸") || strings.HasPrefix(lower, "来源") || strings.HasPrefix(lower, "采集范围") || strings.HasPrefix(lower, "多屏策略") {
		return false
	}
	letters := 0
	digits := 0
	for _, r := range line {
		if unicode.IsLetter(r) {
			letters++
		}
		if unicode.IsDigit(r) {
			digits++
		}
	}
	runeCount := len([]rune(line))
	if letters == 0 && digits < 3 {
		return false
	}
	if runeCount <= 1 {
		return false
	}
	return true
}

func inferOCRTitle(entry Entry, lines []string) string {
	for _, line := range lines {
		if isWeakOCRTitle(line) {
			continue
		}
		runes := []rune(line)
		if len(runes) >= 6 && len(runes) <= 42 {
			return string(runes)
		}
	}
	for _, line := range lines {
		if !isWeakOCRTitle(line) {
			return trimTextRunes(line, 42)
		}
	}
	if window := strings.TrimSpace(entry.WindowTitle); window != "" && !isWeakOCRTitle(window) {
		return trimTextRunes(window, 42)
	}
	return "截图文字整理"
}

func summarizeOCRLines(lines []string, max int) string {
	if len(lines) == 0 {
		return ""
	}
	selected := lines
	if len(selected) > 3 {
		selected = selected[:3]
	}
	return trimTextRunes(strings.Join(selected, "；"), max)
}

func renderCleanOCRText(entry Entry, lines []string) string {
	var builder strings.Builder
	builder.WriteString("## 画面文字整理\n")
	for _, line := range lines {
		builder.WriteString("- ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	context := []string{}
	if app := strings.TrimSpace(entry.AppName); app != "" {
		context = append(context, "应用："+app)
	}
	if window := strings.TrimSpace(entry.WindowTitle); window != "" && !isWeakOCRTitle(window) {
		context = append(context, "窗口："+window)
	}
	if len(context) > 0 {
		builder.WriteString("\n## 上下文\n")
		for _, item := range context {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func normalizeCleanOCRBody(text string) string {
	lines := []string{}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(strings.TrimSpace(raw), " \t")
		if line == "" {
			if len(lines) > 0 && lines[len(lines)-1] != "" {
				lines = append(lines, "")
			}
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func shouldReplaceOCRTitle(title string) bool {
	return isWeakOCRTitle(title) || strings.Contains(title, "屏幕时间机器") || strings.Contains(title, "手动补记") || strings.Contains(title, "截图证据") || strings.Contains(title, "截图尚未识别")
}

func isWeakOCRTitle(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return true
	}
	switch normalized {
	case "work", "unknown", "ariadne", "截图", "截图历史", "当前屏幕", "工作记忆", "自动记录", "自动沉淀":
		return true
	}
	return strings.Contains(normalized, "截图覆盖层")
}

func (s *Service) CaptureCurrentScreen() Entry {
	return s.captureScreen("manual_capture", "手动补记当前屏幕", "用户主动把当前屏幕纳入工作记忆。")
}

func (s *Service) CaptureTimeMachineNow() Entry {
	return s.captureScreen("time_machine", "屏幕时间机器自动记录", "后台时间机器按策略记录当前屏幕。")
}

func (s *Service) CaptureTimeMachineWindowSwitch() Entry {
	return s.captureScreen("time_machine", "屏幕时间机器窗口切换记录", "前台窗口切换触发屏幕时间机器记录。")
}

func (s *Service) GenerateDailyDraft() Draft {
	s.mu.RLock()
	entries := cloneEntries(s.entries)
	s.mu.RUnlock()

	now := s.now()
	selected, todayCount, skippedSensitive := dailyDraftEntries(entries, now, 12)
	evidence := entryIDs(selected, 0)
	return Draft{
		ID:        "daily-" + now.Format("20060102"),
		Title:     "今日工作日报草稿",
		Body:      renderDailyDraftBody(selected, now, todayCount, skippedSensitive),
		Evidence:  evidence,
		CreatedAt: now.Unix(),
	}
}

func (s *Service) PolishDraft(request DraftPolishRequest) DraftPolishResult {
	draft := normalizeDraftForPolish(request.Draft)
	if draft.ID == "" || draft.Body == "" {
		return DraftPolishResult{OK: false, Message: "没有可润色的草稿", Draft: draft}
	}
	kind := normalizeDraftKind(request.Kind)

	s.mu.RLock()
	policy := s.draftPolishPolicy
	polisher := s.draftPolisher
	privacyMode := s.status.PrivacyMode
	s.mu.RUnlock()

	provider := firstNonEmpty(policy.Provider, "disabled")
	model := strings.TrimSpace(policy.Model)
	risks := polishRiskReasons(draft, kind, provider, model)
	if privacyMode {
		return DraftPolishResult{
			OK:          false,
			Message:     "隐私模式已开启，AI 润色已阻断",
			Draft:       draft,
			External:    false,
			Provider:    provider,
			Model:       model,
			RiskReasons: risks,
		}
	}
	if !policy.Enabled || provider == "disabled" || provider == "" {
		return DraftPolishResult{
			OK:          false,
			Message:     "AI 草稿润色未启用",
			Draft:       draft,
			External:    false,
			Provider:    provider,
			Model:       model,
			RiskReasons: risks,
		}
	}
	if !request.Confirmed {
		return DraftPolishResult{
			OK:                   false,
			Message:              "AI 润色需要确认外发",
			Draft:                draft,
			RequiresConfirmation: true,
			External:             true,
			Provider:             provider,
			Model:                model,
			RiskReasons:          risks,
		}
	}
	if polisher == nil {
		return DraftPolishResult{
			OK:          false,
			Message:     "AI 润色客户端未配置",
			Draft:       draft,
			External:    true,
			Provider:    provider,
			Model:       model,
			RiskReasons: risks,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	polished, err := polisher.PolishDraft(ctx, DraftPolishJob{
		Draft:    draft,
		Kind:     kind,
		Provider: provider,
		BaseURL:  policy.BaseURL,
		Model:    model,
	})
	if err != nil {
		return DraftPolishResult{
			OK:          false,
			Message:     "AI 润色失败: " + err.Error(),
			Draft:       draft,
			External:    true,
			Provider:    provider,
			Model:       model,
			RiskReasons: risks,
		}
	}
	polished = normalizePolishedDraft(draft, polished, kind, s.now())
	return DraftPolishResult{
		OK:            true,
		Message:       "AI 润色草稿已生成",
		Draft:         draft,
		PolishedDraft: polished,
		External:      true,
		Provider:      provider,
		Model:         model,
		RiskReasons:   risks,
	}
}

func (s *Service) GenerateRetrospectiveDraft(ids []string) Draft {
	s.mu.RLock()
	entries := cloneEntries(s.entries)
	s.mu.RUnlock()

	now := s.now()
	selected, requestedCount, skippedSensitive := retrospectiveDraftEntries(entries, ids, 12)
	title := "问题复盘草稿"
	if len(selected) > 0 {
		title = "问题复盘草稿：" + shortDraftTitle(firstNonEmpty(selected[0].Title, selected[0].Summary, selected[0].ID), 28)
	}
	return Draft{
		ID:        "retrospective-" + now.Format("20060102150405"),
		Title:     title,
		Body:      renderRetrospectiveDraftBody(selected, now, requestedCount, skippedSensitive),
		Evidence:  entryIDs(selected, 0),
		CreatedAt: now.Unix(),
	}
}

func (s *Service) GenerateKnowledgeDraft(requestedIDs []string) Draft {
	entries := s.entriesByID(requestedIDs)
	return Draft{
		ID:        "knowledge-" + s.now().Format("20060102150405"),
		Title:     "知识条目草稿",
		Body:      "从选中的工作记忆整理问题背景、处理步骤、注意事项和敏感内容提示。",
		Evidence:  entryIDs(entries, 0),
		CreatedAt: s.now().Unix(),
	}
}

func (s *Service) GenerateAgentTaskPackage(goal string, evidence []string) AgentTaskPackage {
	entries := s.entriesByID(evidence)
	return AgentTaskPackage{
		ID:       "agent-task-" + s.now().Format("20060102150405"),
		Goal:     goal,
		Context:  "由 Ariadne 工作记忆中心生成，可交给 Codex 桌面版等外部代理前必须由用户确认。",
		Evidence: entryIDs(entries, 0),
		Boundaries: []string{
			"不得绕过用户授权修改文件或运行高风险命令",
			"不得把敏感记忆默认发送到外部 AI 或 embedding 服务",
			"必须保留验收证据",
		},
		Acceptance: []string{
			"任务包包含目标、上下文、证据、边界和验收标准",
			"执行前用户已确认项目范围和权限",
		},
		RequiresReview: true,
		CreatedAt:      s.now().Unix(),
	}
}

func (s *Service) GenerateWorkflowDraft(title string, evidence []string) WorkflowDraft {
	evidence = cleanStrings(evidence)
	entries := s.entriesByID(evidence)
	profile := workflowDraftProfile(entries, title)
	now := s.now()
	return WorkflowDraft{
		ID:             "workflow-draft-" + now.Format("20060102150405"),
		Title:          firstNonEmpty(strings.TrimSpace(title), profile.title),
		Trigger:        profile.trigger,
		Input:          profile.input,
		Steps:          profile.steps,
		Output:         profile.output,
		RiskLevel:      profile.riskLevel,
		Evidence:       entryIDs(entries, 0),
		RequiresReview: true,
		CreatedAt:      now.Unix(),
	}
}

func (s *Service) GenerateChecklistDraft(title string, evidence []string) ChecklistDraft {
	evidence = cleanStrings(evidence)
	entries := s.entriesByID(evidence)
	profile := checklistDraftProfile(entries, title)
	now := s.now()
	return ChecklistDraft{
		ID:             "checklist-draft-" + now.Format("20060102150405"),
		Title:          firstNonEmpty(strings.TrimSpace(title), profile.title),
		Context:        profile.context,
		Items:          profile.items,
		Evidence:       entryIDs(entries, 0),
		RequiresReview: true,
		CreatedAt:      now.Unix(),
	}
}

func (s *Service) entriesByID(ids []string) []Entry {
	if len(ids) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = true
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := []Entry{}
	for _, entry := range s.entries {
		if allowed[entry.ID] && entryUsableForExtraction(entry) {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (s *Service) DiscoverExperiences(periodDays int) ExperienceReport {
	if periodDays <= 0 {
		periodDays = 7
	}
	now := s.now()
	cutoff := now.Add(-time.Duration(periodDays) * 24 * time.Hour).Unix()

	s.mu.RLock()
	entries := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if !entryUsableForExtraction(entry) {
			continue
		}
		if entry.CreatedAt != 0 && entry.CreatedAt < cutoff {
			continue
		}
		entries = append(entries, entry)
	}
	decisions := cloneExperienceDecisions(s.decisions)
	s.mu.RUnlock()

	return buildExperienceReport(entries, decisions, periodDays, now)
}

func (s *Service) DiscoverExperiencesAI(request ExperienceDiscoveryRequest) ExperienceDiscoveryResult {
	periodDays := request.PeriodDays
	if periodDays <= 0 {
		periodDays = 7
	}
	now := s.now()
	cutoff := now.Add(-time.Duration(periodDays) * 24 * time.Hour).Unix()

	s.mu.RLock()
	entries := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if !entryUsableForExtraction(entry) {
			continue
		}
		if entry.CreatedAt != 0 && entry.CreatedAt < cutoff {
			continue
		}
		entries = append(entries, entry)
	}
	decisions := cloneExperienceDecisions(s.decisions)
	privacyMode := s.status.PrivacyMode
	policy := normalizeExperienceDiscoveryPolicy(s.experiencePolicy)
	discoverer := s.experienceDiscoverer
	s.mu.RUnlock()

	localReport := buildExperienceReport(entries, decisions, periodDays, now)
	externalEvidence := experienceDiscoveryEvidence(entries)
	if !request.External {
		return ExperienceDiscoveryResult{
			OK:      true,
			Message: "本地经验发现完成",
			Report:  localReport,
		}
	}

	result := ExperienceDiscoveryResult{
		Report:      localReport,
		External:    true,
		Provider:    policy.Provider,
		Model:       policy.Model,
		RiskReasons: experienceDiscoveryRiskReasons(policy, len(externalEvidence)),
	}
	if privacyMode {
		result.Message = "隐私模式已开启，外部 AI 经验发现已阻止"
		return result
	}
	if !policy.Enabled {
		result.Message = "AI 经验发现未启用"
		return result
	}
	if policy.Model == "" {
		result.Message = "AI 经验发现缺少模型配置"
		return result
	}
	if discoverer == nil {
		result.Message = "AI 经验发现客户端未注册"
		return result
	}
	if len(externalEvidence) == 0 {
		result.Message = "没有可用于 AI 经验发现的非敏感工作记忆"
		return result
	}
	if !request.Confirmed {
		result.RequiresConfirmation = true
		result.Message = "AI 经验发现需要二次确认"
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	report, err := discoverer.DiscoverExperiences(ctx, ExperienceDiscoveryJob{
		Evidence:   externalEvidence,
		PeriodDays: periodDays,
		Provider:   policy.Provider,
		BaseURL:    policy.BaseURL,
		Model:      policy.Model,
		Now:        now,
	})
	if err != nil {
		result.Message = "外部 AI 经验发现失败，已保留本地规则报告: " + err.Error()
		return result
	}
	result.OK = true
	result.Message = "AI 经验发现完成"
	result.Report = normalizeExternalExperienceReport(report, entries, decisions, periodDays, now)
	return result
}

func (s *Service) SetExperienceInsightDecision(insightID string, status string, note string, taskPackageID string) ExperienceDecisionResult {
	insightID = strings.TrimSpace(insightID)
	status = normalizeExperienceDecisionStatus(status)
	note = strings.TrimSpace(note)
	taskPackageID = strings.TrimSpace(taskPackageID)
	if insightID == "" {
		return ExperienceDecisionResult{OK: false, Message: "缺少经验线索 ID"}
	}
	if status == "" {
		return ExperienceDecisionResult{OK: false, Message: "不支持的处理状态"}
	}
	now := s.now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.decisions == nil {
		s.decisions = map[string]ExperienceDecision{}
	}
	if status == "pending" {
		delete(s.decisions, insightID)
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
			return ExperienceDecisionResult{OK: false, Message: err.Error()}
		}
		return ExperienceDecisionResult{
			OK:      true,
			Message: "已清除经验线索处理状态",
			Decision: ExperienceDecision{
				InsightID: insightID,
				Status:    "pending",
				UpdatedAt: now,
			},
		}
	}
	decision := ExperienceDecision{
		InsightID:     insightID,
		Status:        status,
		Note:          note,
		TaskPackageID: taskPackageID,
		UpdatedAt:     now,
	}
	s.decisions[insightID] = decision
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
		return ExperienceDecisionResult{OK: false, Message: err.Error(), Decision: decision}
	}
	return ExperienceDecisionResult{OK: true, Message: "经验线索处理状态已保存", Decision: decision}
}

func (s *Service) runAutonomousFlow(force bool) AutonomousRunResult {
	now := s.now()
	s.mu.RLock()
	policy := normalizeDraftSchedulePolicy(s.draftSchedule)
	enabled := s.status.Enabled
	privacyMode := s.status.PrivacyMode
	entries := cloneEntries(s.entries)
	decisions := cloneExperienceDecisions(s.decisions)
	artifacts := cloneAutonomousArtifacts(s.autonomousArtifacts)
	rejections := cloneAutonomousRejections(s.autonomousRejections)
	lastRunAt := s.lastAutonomousRunAt
	s.mu.RUnlock()

	result := AutonomousRunResult{OK: false, CreatedAt: now.Unix()}
	if !enabled {
		result.Message = "工作记忆已停用"
		return result
	}
	if privacyMode {
		result.Message = "隐私模式已开启，自主沉淀暂停"
		return result
	}
	usable := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entryUsableForExtraction(entry) {
			usable = append(usable, entry)
		}
	}
	if len(usable) == 0 {
		result.Message = "没有可用于自主沉淀的非敏感工作记忆"
		return result
	}
	result = buildAutonomousArtifacts(usable, decisions, artifacts, rejections, lastRunAt, policy, now, force)
	if !result.OK {
		return result
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAutonomousRunAt = now.Unix()
	s.scheduledDrafts.LastAutonomousRunAt = s.lastAutonomousRunAt
	s.scheduledDrafts.AutonomousGenerated = len(result.Artifacts)
	s.scheduledDrafts.AutonomousMessage = result.Message
	if len(result.Artifacts) > 0 {
		s.mergeAutonomousArtifactsLocked(result.Artifacts)
	}
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
		result.OK = false
		result.Message = err.Error()
	}
	result.Status = s.scheduledDraftStatusLocked()
	return result
}

func (s *Service) runScheduledDrafts(force bool) ScheduledDraftStatus {
	now := s.now()
	s.mu.RLock()
	policy := normalizeDraftSchedulePolicy(s.draftSchedule)
	enabled := s.status.Enabled
	privacyMode := s.status.PrivacyMode
	previousLatest := s.scheduledDrafts.LastEntryCreatedAt
	entries := cloneEntries(s.entries)
	decisions := cloneExperienceDecisions(s.decisions)
	autonomousArtifacts := cloneAutonomousArtifacts(s.autonomousArtifacts)
	autonomousRejections := cloneAutonomousRejections(s.autonomousRejections)
	lastAutonomousRunAt := s.lastAutonomousRunAt
	s.mu.RUnlock()

	nonSensitive := make([]Entry, 0, len(entries))
	latestCreatedAt := int64(0)
	for _, entry := range entries {
		if !entryUsableForExtraction(entry) {
			continue
		}
		nonSensitive = append(nonSensitive, entry)
		if entry.CreatedAt > latestCreatedAt {
			latestCreatedAt = entry.CreatedAt
		}
	}

	s.mu.Lock()
	s.scheduledDrafts.Enabled = policy.Enabled
	s.scheduledDrafts.IntervalMinutes = policy.IntervalMinutes
	s.scheduledDrafts.DailyDraftEnabled = policy.DailyDraftEnabled
	s.scheduledDrafts.RetrospectiveEnabled = policy.RetrospectiveEnabled
	s.scheduledDrafts.ExperienceReportEnabled = policy.ExperienceReportEnabled
	s.scheduledDrafts.LastCheckedAt = now.Unix()
	if !enabled {
		s.scheduledDrafts.LastError = "工作记忆已停用"
		status := s.scheduledDraftStatusLocked()
		s.mu.Unlock()
		return status
	}
	if !policy.Enabled && !force {
		s.scheduledDrafts.LastError = "定期草稿未启用"
		status := s.scheduledDraftStatusLocked()
		s.mu.Unlock()
		return status
	}
	if privacyMode {
		s.scheduledDrafts.LastError = "隐私模式已开启，定期草稿暂停"
		status := s.scheduledDraftStatusLocked()
		s.mu.Unlock()
		return status
	}
	if len(nonSensitive) == 0 {
		s.scheduledDrafts.LastError = "没有可用于调度草稿的非敏感工作记忆"
		status := s.scheduledDraftStatusLocked()
		s.mu.Unlock()
		return status
	}
	if !force && previousLatest >= latestCreatedAt {
		s.scheduledDrafts.LastError = "没有新的非敏感工作记忆"
		status := s.scheduledDraftStatusLocked()
		s.mu.Unlock()
		return status
	}
	s.mu.Unlock()

	var daily Draft
	if policy.DailyDraftEnabled {
		selected, todayCount, skippedSensitive := dailyDraftEntries(entries, now, 12)
		daily = Draft{
			ID:        "daily-" + now.Format("20060102"),
			Title:     "今日工作日报草稿",
			Body:      renderDailyDraftBody(selected, now, todayCount, skippedSensitive),
			Evidence:  entryIDs(selected, 0),
			CreatedAt: now.Unix(),
		}
	}
	var retrospective Draft
	if policy.RetrospectiveEnabled {
		retrospectiveEntries := scheduledRetrospectiveEntries(nonSensitive, 12)
		if len(retrospectiveEntries) > 0 {
			retrospective = Draft{
				ID:        "retrospective-" + now.Format("20060102150405"),
				Title:     "问题复盘草稿：" + shortDraftTitle(firstNonEmpty(retrospectiveEntries[0].Title, retrospectiveEntries[0].Summary, retrospectiveEntries[0].ID), 28),
				Body:      renderRetrospectiveDraftBody(retrospectiveEntries, now, len(retrospectiveEntries), 0),
				Evidence:  entryIDs(retrospectiveEntries, 0),
				CreatedAt: now.Unix(),
			}
		}
	}
	var experience ExperienceReport
	if policy.ExperienceReportEnabled {
		experience = buildExperienceReport(nonSensitive, decisions, policy.ExperiencePeriodDays, now)
	}
	autonomous := buildAutonomousArtifacts(nonSensitive, decisions, autonomousArtifacts, autonomousRejections, lastAutonomousRunAt, policy, now, force)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduledDrafts.LastRunAt = now.Unix()
	s.scheduledDrafts.LastEntryCount = len(nonSensitive)
	s.scheduledDrafts.LastEntryCreatedAt = latestCreatedAt
	s.scheduledDrafts.LastError = ""
	s.scheduledDrafts.AutonomousGenerated = len(autonomous.Artifacts)
	s.scheduledDrafts.AutonomousMessage = autonomous.Message
	if daily.ID != "" {
		s.scheduledDrafts.DailyDraft = daily
	}
	if retrospective.ID != "" {
		s.scheduledDrafts.RetrospectiveDraft = retrospective
	}
	if experience.ID != "" {
		s.scheduledDrafts.ExperienceReport = experience
	}
	if autonomous.OK {
		s.lastAutonomousRunAt = now.Unix()
		s.scheduledDrafts.LastAutonomousRunAt = s.lastAutonomousRunAt
		if len(autonomous.Artifacts) > 0 {
			s.mergeAutonomousArtifactsLocked(autonomous.Artifacts)
		}
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
			s.scheduledDrafts.LastError = err.Error()
		}
	}
	return s.scheduledDraftStatusLocked()
}

func (s *Service) Delete(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		return s.statusSnapshotLocked()
	}
	next := s.entries[:0]
	for _, entry := range s.entries {
		if entry.ID != id {
			next = append(next, entry)
		}
	}
	s.entries = next
	s.refreshEntryCountLocked()
	s.saveLockedWithStatus()
	return s.statusSnapshotLocked()
}

func (s *Service) ClearUnpinned() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.entries[:0]
	for _, entry := range s.entries {
		if entry.Favorite {
			next = append(next, entry)
		}
	}
	s.entries = next
	s.refreshEntryCountLocked()
	s.saveLockedWithStatus()
	return s.statusSnapshotLocked()
}

func (s *Service) ApplyRetentionPolicy(retentionDays int, keepFavorites bool) RetentionResult {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	now := s.now()
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	result := RetentionResult{
		OK:            true,
		RetentionDays: retentionDays,
		KeepFavorites: keepFavorites,
		CutoffAt:      cutoff,
		AppliedAt:     now.Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.entries[:0]
	for _, entry := range s.entries {
		if keepFavorites && entry.Favorite {
			result.Kept++
			result.KeptFavorites++
			next = append(next, entry)
			continue
		}
		if entry.CreatedAt == 0 || entry.CreatedAt >= cutoff {
			result.Kept++
			next = append(next, entry)
			continue
		}
		result.Removed++
	}
	s.entries = next
	s.refreshEntryCountLocked()
	if result.Removed > 0 {
		s.saveLockedWithStatus()
	}
	result.RemainingCount = len(s.entries)
	return result
}

func (s *Service) ExportData(includeSensitive bool) ExportResult {
	return s.ExportDataWithOptions(ExportRequest{IncludeSensitive: includeSensitive})
}

func (s *Service) ExportDataWithOptions(request ExportRequest) ExportResult {
	filter := normalizeExportFilter(request)
	s.mu.RLock()
	if s.status.PrivacyMode {
		s.mu.RUnlock()
		return ExportResult{OK: false, Message: "隐私模式已开启，已阻止导出", IncludesSensitive: request.IncludeSensitive, Filter: filter}
	}
	entries := cloneEntries(s.entries)
	policy := s.policy
	basePath := s.path
	now := s.now()
	s.mu.RUnlock()

	exported := make([]Entry, 0, len(entries))
	skippedSensitive := 0
	skippedExcluded := 0
	filteredOut := 0
	for _, entry := range entries {
		if !entryMatchesExportFilter(entry, filter) {
			filteredOut++
			continue
		}
		if excluded, _ := entryExcluded(entry, policy); excluded {
			skippedExcluded++
			continue
		}
		if entry.Sensitive && !request.IncludeSensitive {
			skippedSensitive++
			continue
		}
		exported = append(exported, entry)
	}

	exportPath := exportPackagePath(basePath, now)
	if err := writeExportPackage(exportPath, exported, request.IncludeSensitive, skippedSensitive, skippedExcluded, filteredOut, filter, now); err != nil {
		return ExportResult{
			OK:                    false,
			Message:               err.Error(),
			Path:                  exportPath,
			EntryCount:            len(exported),
			SkippedSensitiveCount: skippedSensitive,
			SkippedExcludedCount:  skippedExcluded,
			FilteredOutCount:      filteredOut,
			IncludesSensitive:     request.IncludeSensitive,
			Filter:                filter,
			CreatedAt:             now.Unix(),
		}
	}

	bytes := int64(0)
	if info, err := os.Stat(exportPath); err == nil {
		bytes = info.Size()
	}
	return ExportResult{
		OK:                    true,
		Message:               "工作记忆数据包已导出",
		Path:                  exportPath,
		EntryCount:            len(exported),
		SkippedSensitiveCount: skippedSensitive,
		SkippedExcludedCount:  skippedExcluded,
		FilteredOutCount:      filteredOut,
		IncludesSensitive:     request.IncludeSensitive,
		Filter:                filter,
		Bytes:                 bytes,
		CreatedAt:             now.Unix(),
	}
}

func (s *Service) ReviewPendingCaptures() QualityReviewResult {
	now := s.now().Unix()
	result := QualityReviewResult{
		OK:         true,
		ReviewedAt: now,
	}
	entriesForQualityOCR := []Entry{}
	s.mu.Lock()
	activeSessionID := s.currentWindowSessionEntryID
	policy := s.policy
	for i := range s.entries {
		entry := &s.entries[i]
		if !entryQualityPending(*entry) {
			continue
		}
		if entry.ID != "" && entry.ID == activeSessionID {
			continue
		}
		if needsQualityOCRPrecheck(*entry, policy) {
			entriesForQualityOCR = append(entriesForQualityOCR, *entry)
		}
	}
	s.mu.Unlock()

	for _, entry := range entriesForQualityOCR {
		s.processAutoOCR(entry, policy)
	}
	result.QualityOCR = len(entriesForQualityOCR)

	entriesToPromote := []Entry{}
	s.mu.Lock()
	activeSessionID = s.currentWindowSessionEntryID
	for i := range s.entries {
		entry := &s.entries[i]
		if !entryQualityPending(*entry) {
			continue
		}
		if entry.ID != "" && entry.ID == activeSessionID {
			result.SkippedActive++
			continue
		}
		result.Checked++
		if removed := collapseRedundantFrames(entry); removed > 0 {
			result.CollapsedEntries++
			result.RemovedFrames += removed
		}
		entry.QualityStatus = qualityStatusChecked
		entry.QualityCheckedAt = now
		if entry.QualityReason == "" || strings.HasPrefix(entry.QualityReason, "待质检") {
			entry.QualityReason = qualityReviewReason(*entry)
		}
		entry.Tags = removeStringFold(entry.Tags, "待质检")
		entry.Tags = cleanStrings(append(entry.Tags, "已质检"))
		if needsQualityOCRPromotion(*entry) {
			entriesToPromote = append(entriesToPromote, *entry)
		}
	}
	for _, entry := range s.entries {
		if entryQualityPending(entry) {
			result.PendingRemaining++
		}
	}
	if result.Checked > 0 || result.CollapsedEntries > 0 {
		s.refreshEntryCountLocked()
		s.saveLockedWithStatus()
	}
	result.Message = fmt.Sprintf("自动质检完成 · 检查 %d 条，折叠 %d 条，移除 %d 帧", result.Checked, result.CollapsedEntries, result.RemovedFrames)
	s.mu.Unlock()

	for _, entry := range entriesToPromote {
		if promoted := s.promoteQualityOCR(entry); promoted.ID != "" {
			result.OCRPromoted++
		}
	}
	return result
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopWorkerLocked()
	s.stopDraftSchedulerLocked()
	s.flushDirtyFTSLocked()
	s.ftsRebuildDisabled = true
	s.ftsRebuildScheduled = false
	if s.fts != nil {
		s.fts.Close()
		s.fts = nil
	}
}

func (s *Service) captureScreen(source string, title string, summary string) Entry {
	s.mu.RLock()
	if s.status.PrivacyMode {
		s.mu.RUnlock()
		s.mu.Lock()
		s.status.PauseReason = "隐私模式已开启"
		s.mu.Unlock()
		return Entry{}
	}
	if !s.status.Enabled {
		s.mu.RUnlock()
		s.mu.Lock()
		s.status.PauseReason = "工作记忆已停用"
		s.mu.Unlock()
		return Entry{}
	}
	capturer := s.capturer
	context := s.currentContextLocked()
	policy := s.policy
	activity := s.activity
	now := s.now
	s.mu.RUnlock()

	if source == "time_machine" && activity != nil {
		snapshot := activity.Snapshot(now())
		s.recordActivitySnapshot(snapshot)
		if paused, reason := capturePausedByActivity(policy, snapshot); paused {
			s.recordSkippedCapture(reason)
			return Entry{}
		}
	}

	if skipped, reason := captureExcluded(context, policy); skipped {
		s.recordSkippedCapture(reason)
		return Entry{}
	}

	if capturer == nil {
		entry := s.addEntry(s.syntheticEntry(source, title, summary, policy))
		if entryQualityPending(entry) {
			return entry
		}
		return s.processAutoOCR(entry, policy)
	}

	captureSource := "work_memory_" + source
	captureStatus := capturehistory.Status{}
	if optionCapturer, ok := capturer.(OptionScreenCapturer); ok {
		captureStatus = optionCapturer.CaptureScreenWithOptions(captureSource, capturehistory.CaptureOptions{
			CaptureScope: policy.CaptureScope,
			MultiMonitor: policy.MultiMonitor,
		})
	} else {
		captureStatus = capturer.CaptureScreen(captureSource)
	}
	if captureStatus.LastCaptureError != "" {
		s.mu.Lock()
		s.status.LastCaptureError = captureStatus.LastCaptureError
		s.status.PauseReason = "截图采集失败"
		s.mu.Unlock()
		return Entry{}
	}
	captureEntry, ok := latestCapture(captureStatus.Entries, captureSource)
	if !ok {
		s.mu.Lock()
		s.status.LastCaptureError = "截图服务未返回新记录"
		s.status.PauseReason = "截图采集失败"
		s.mu.Unlock()
		return Entry{}
	}
	entry := s.addEntry(s.entryFromCapture(source, title, summary, captureEntry, policy))
	if entryQualityPending(entry) {
		return entry
	}
	return s.processAutoOCR(entry, policy)
}

func (s *Service) addEntry(entry Entry) Entry {
	entry = normalizeEntry(entry)
	if entry.ID == "" {
		return Entry{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if appended, ok := s.appendTimeMachineSessionFrameLocked(entry); ok {
		return appended
	}

	if merged := s.mergeDuplicateTimeMachineEntryLocked(entry); merged.ID != "" {
		return merged
	}

	next := []Entry{entry}
	for _, existing := range s.entries {
		if existing.ID != entry.ID {
			next = append(next, existing)
		}
	}
	s.entries = next
	s.trimLocked()
	s.status.LastCaptureAt = entry.CreatedAt
	s.status.LastCaptureID = entry.ID
	s.status.LastCaptureError = ""
	if s.status.PauseReason == "截图采集失败" {
		s.status.PauseReason = ""
	}
	if strings.HasPrefix(s.status.PauseReason, "排除规则") {
		s.status.PauseReason = ""
	}
	s.status.CaptureCount++
	if entry.Source == "time_machine" {
		signature := windowSignature(windowContext{title: entry.WindowTitle, app: entry.AppName})
		if signature != "" && signature == s.currentWindowSessionSignature {
			s.currentWindowSessionEntryID = entry.ID
		}
	}
	s.refreshEntryCountLocked()
	s.saveLockedWithStatus()
	return entry
}

func (s *Service) appendTimeMachineSessionFrameLocked(entry Entry) (Entry, bool) {
	if entry.Source != "time_machine" || s.currentWindowSessionEntryID == "" {
		return Entry{}, false
	}
	signature := windowSignature(windowContext{title: entry.WindowTitle, app: entry.AppName})
	if signature == "" || signature != s.currentWindowSessionSignature {
		return Entry{}, false
	}
	for i := range s.entries {
		if s.entries[i].ID != s.currentWindowSessionEntryID {
			continue
		}
		existing := &s.entries[i]
		if existing.Source != "time_machine" {
			return Entry{}, false
		}
		if len(existing.Frames) == 0 {
			existing.Frames = []CaptureFrame{captureFrameFromEntry(*existing)}
		}
		frame := captureFrameFromEntry(entry)
		existing.Frames = append(existing.Frames, frame)
		applyFrameToEntry(existing, frame)
		existing.FrameCount = len(existing.Frames)
		existing.LastMergedAt = entry.CreatedAt
		existing.QualityStatus = qualityStatusPending
		existing.QualityCheckedAt = 0
		existing.QualityReason = "待质检：窗口保持期间多帧采集"
		existing.Tags = cleanStrings(append(existing.Tags, "多帧采集", "待质检"))
		existing.Summary = fmt.Sprintf("窗口保持期间已采集 %d 帧，等待自动质检。", existing.FrameCount)
		s.status.LastCaptureAt = entry.CreatedAt
		s.status.LastCaptureID = existing.ID
		s.status.LastCaptureError = ""
		s.status.CaptureCount++
		s.refreshEntryCountLocked()
		s.saveLockedWithStatus()
		return *existing, true
	}
	return Entry{}, false
}

func (s *Service) mergeDuplicateTimeMachineEntryLocked(entry Entry) Entry {
	if entry.Source != "time_machine" {
		return Entry{}
	}
	for i := range s.entries {
		existing := &s.entries[i]
		if existing.Source != "time_machine" {
			continue
		}
		switch {
		case entry.ImageSignature != "" && existing.ImageSignature != "" && existing.ImageSignature == entry.ImageSignature:
			return s.mergeTimeMachineEntryLocked(existing, entry, "重复画面已合并", "重复画面合并", "重复画面")
		case similarImageFingerprint(*existing, entry):
			return s.mergeTimeMachineEntryLocked(existing, entry, "相似画面已合并", "相似画面合并", "相似画面")
		}
	}
	return Entry{}
}

func (s *Service) mergeTimeMachineEntryLocked(existing *Entry, incoming Entry, reason string, tag string, summaryMarker string) Entry {
	existing.MergedCount++
	existing.LastMergedAt = incoming.CreatedAt
	existing.QualityStatus = qualityStatusPending
	existing.QualityCheckedAt = 0
	existing.QualityReason = "待质检：" + reason
	existing.Tags = cleanStrings(append(existing.Tags, tag))
	if existing.Summary != "" && !strings.Contains(existing.Summary, summaryMarker) {
		existing.Summary = existing.Summary + "（" + reason + "）"
	}
	s.status.LastSkippedAt = incoming.CreatedAt
	s.status.LastSkippedReason = reason
	s.status.LastCaptureAt = incoming.CreatedAt
	s.status.LastCaptureID = existing.ID
	s.status.LastCaptureError = ""
	s.refreshEntryCountLocked()
	s.saveLockedWithStatus()
	return *existing
}

func (s *Service) reviewPendingCapturesAsync() {
	s.mu.Lock()
	if s.qualityReviewRunning {
		s.mu.Unlock()
		return
	}
	s.qualityReviewRunning = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.qualityReviewRunning = false
			s.mu.Unlock()
		}()
		s.ReviewPendingCaptures()
	}()
}

func (s *Service) entryFromCapture(source string, title string, summary string, capture capturehistory.Entry, policy CapturePolicy) Entry {
	context := s.currentContext()
	dimension := fmt.Sprintf("%dx%d", capture.Width, capture.Height)
	scope := captureScopeLabel(policy.CaptureScope)
	multiMonitor := multiMonitorLabel(policy.MultiMonitor)
	entry := Entry{
		ID:               "memory-" + source + "-" + capture.ID,
		Source:           source,
		ContentType:      "screenshot",
		Title:            title,
		Summary:          summary,
		Text:             fmt.Sprintf("截图路径: %s\n尺寸: %s\n来源: %s\n采集范围: %s\n多屏策略: %s", capture.ImagePath, dimension, sourceLabel(source), scope, multiMonitor),
		WindowTitle:      context.title,
		AppName:          context.app,
		CaptureID:        capture.ID,
		ImagePath:        capture.ImagePath,
		ImageSignature:   capture.Signature,
		ImageFingerprint: imageFingerprint(capture.ImagePath),
		Width:            capture.Width,
		Height:           capture.Height,
		Bytes:            capture.Bytes,
		Tags:             []string{sourceLabel(source), "截图", "屏幕时间机器", dimension, "范围:" + scope, "多屏:" + multiMonitor},
		Favorite:         false,
		Sensitive:        false,
		CreatedAt:        capture.CreatedAt,
	}
	if source == "time_machine" {
		entry.QualityStatus = qualityStatusPending
		entry.QualityReason = "待质检：时间机器自动采集"
		entry.Tags = cleanStrings(append(entry.Tags, "待质检"))
		entry.Frames = []CaptureFrame{captureFrameFromEntry(entry)}
		entry.FrameCount = 1
	}
	return entry
}

func (s *Service) syntheticEntry(source string, title string, summary string, policy CapturePolicy) Entry {
	now := s.now()
	context := s.currentContext()
	entry := Entry{
		ID:          "memory-" + source + "-" + now.Format("20060102150405"),
		Source:      source,
		ContentType: "screenshot",
		Title:       title,
		Summary:     summary,
		Text:        "截图服务未接入当前测试实例；已记录工作记忆事件。",
		WindowTitle: context.title,
		AppName:     context.app,
		Tags:        []string{sourceLabel(source), "屏幕时间机器", "范围:" + captureScopeLabel(policy.CaptureScope), "多屏:" + multiMonitorLabel(policy.MultiMonitor)},
		Favorite:    false,
		Sensitive:   false,
		CreatedAt:   now.Unix(),
	}
	if source == "time_machine" {
		entry.QualityStatus = qualityStatusPending
		entry.QualityReason = "待质检：时间机器自动采集"
		entry.Tags = cleanStrings(append(entry.Tags, "待质检"))
		entry.Frames = []CaptureFrame{captureFrameFromEntry(entry)}
		entry.FrameCount = 1
	}
	return entry
}

func captureFrameFromEntry(entry Entry) CaptureFrame {
	return CaptureFrame{
		CaptureID:        strings.TrimSpace(entry.CaptureID),
		ImagePath:        strings.TrimSpace(entry.ImagePath),
		ImageSignature:   strings.TrimSpace(entry.ImageSignature),
		ImageFingerprint: strings.TrimSpace(entry.ImageFingerprint),
		Width:            entry.Width,
		Height:           entry.Height,
		Bytes:            entry.Bytes,
		WindowTitle:      strings.TrimSpace(entry.WindowTitle),
		AppName:          strings.TrimSpace(entry.AppName),
		CreatedAt:        entry.CreatedAt,
	}
}

func applyFrameToEntry(entry *Entry, frame CaptureFrame) {
	entry.CaptureID = strings.TrimSpace(frame.CaptureID)
	entry.ImagePath = strings.TrimSpace(frame.ImagePath)
	entry.ImageSignature = strings.TrimSpace(frame.ImageSignature)
	entry.ImageFingerprint = strings.TrimSpace(frame.ImageFingerprint)
	entry.Width = frame.Width
	entry.Height = frame.Height
	entry.Bytes = frame.Bytes
	entry.WindowTitle = strings.TrimSpace(firstNonEmpty(frame.WindowTitle, entry.WindowTitle))
	entry.AppName = strings.TrimSpace(firstNonEmpty(frame.AppName, entry.AppName))
	if frame.CreatedAt > 0 {
		entry.LastMergedAt = frame.CreatedAt
	}
}

func (s *Service) startWorkerLocked() {
	if s.stopWorker != nil {
		s.status.WorkerRunning = true
		return
	}
	stop := make(chan struct{})
	interval := s.interval
	if interval <= 0 {
		interval = defaultAutoCaptureInterval
	}
	s.stopWorker = stop
	s.status.WorkerRunning = true
	go s.runWorker(stop, interval)
}

func (s *Service) restartWorkerLocked() {
	s.stopWorkerLocked()
	s.startWorkerLocked()
}

func (s *Service) stopWorkerLocked() {
	if s.stopWorker == nil {
		s.status.WorkerRunning = false
		return
	}
	close(s.stopWorker)
	s.stopWorker = nil
	s.status.WorkerRunning = false
}

func (s *Service) runWorker(stop <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	qualityTicker := time.NewTicker(qualityReviewInterval)
	defer qualityTicker.Stop()
	foregroundEvents := make(chan struct{}, 1)
	foregroundWatchErrors := make(chan error, 1)
	go func() {
		if err := watchForegroundWindow(stop, func() {
			select {
			case foregroundEvents <- struct{}{}:
			default:
			}
		}); err != nil {
			select {
			case foregroundWatchErrors <- err:
			default:
			case <-stop:
			}
		}
	}()

	var fallbackTicker *time.Ticker
	var fallbackC <-chan time.Time
	startFallbackTicker := func() {
		if fallbackTicker != nil {
			return
		}
		interval := windowSwitchPollInterval
		if interval <= 0 {
			interval = time.Second
		}
		fallbackTicker = time.NewTicker(interval)
		fallbackC = fallbackTicker.C
	}
	if windowSwitchPollInterval < time.Second {
		startFallbackTicker()
	}
	defer func() {
		if fallbackTicker != nil {
			fallbackTicker.Stop()
		}
	}()

	var windowTimer *time.Timer
	var windowTimerC <-chan time.Time
	stopWindowTimer := func() {
		if windowTimer == nil {
			return
		}
		if !windowTimer.Stop() {
			select {
			case <-windowTimer.C:
			default:
			}
		}
		windowTimer = nil
		windowTimerC = nil
	}
	resetWindowTimer := func() {
		stopWindowTimer()
		delay, ok := s.nextWindowCaptureDelay()
		if !ok {
			return
		}
		if delay < time.Millisecond {
			delay = time.Millisecond
		}
		windowTimer = time.NewTimer(delay)
		windowTimerC = windowTimer.C
	}

	s.captureIfWindowChanged()
	s.reviewPendingCapturesAsync()
	resetWindowTimer()
	for {
		select {
		case <-stop:
			stopWindowTimer()
			return
		case <-ticker.C:
			s.captureScheduledTimeMachine()
		case <-foregroundEvents:
			s.captureIfWindowChanged()
			resetWindowTimer()
		case <-fallbackC:
			s.captureIfWindowChanged()
			resetWindowTimer()
		case <-windowTimerC:
			windowTimer = nil
			windowTimerC = nil
			s.captureIfWindowChanged()
			resetWindowTimer()
		case err := <-foregroundWatchErrors:
			if err != nil {
				startFallbackTicker()
			}
		case <-qualityTicker.C:
			s.ReviewPendingCaptures()
		}
	}
}

func (s *Service) nextWindowCaptureDelay() (time.Duration, bool) {
	s.mu.RLock()
	policy := s.policy
	intervalSeconds := int(s.interval.Seconds())
	enabled := s.status.Enabled && s.status.TimeMachineEnabled && !s.status.PrivacyMode && (policy.CaptureOnWindowChange || hasEnabledAppCaptureProfiles(policy))
	signature := s.lastWindowSignature
	context := s.lastWindowContext
	pendingSignature := s.pendingWindowSignature
	pendingDueAt := s.pendingWindowDueAt
	lastAppCaptureAt := s.lastAppCaptureAt[signature]
	now := s.now().Unix()
	s.mu.RUnlock()
	if !enabled || signature == "" {
		return 0, false
	}
	if pendingSignature == signature && pendingDueAt > 0 {
		return unixDelay(now, pendingDueAt), true
	}
	if profile, ok := windowCaptureProfileForContext(context, policy, intervalSeconds); ok && profile.ActiveIntervalSeconds > 0 && lastAppCaptureAt > 0 {
		return unixDelay(now, lastAppCaptureAt+int64(profile.ActiveIntervalSeconds)), true
	}
	return 0, false
}

func unixDelay(now int64, dueAt int64) time.Duration {
	if dueAt <= now {
		return time.Millisecond
	}
	return time.Duration(dueAt-now) * time.Second
}

func (s *Service) captureScheduledTimeMachine() {
	s.mu.RLock()
	policy := s.policy
	enabled := s.status.Enabled && s.status.TimeMachineEnabled && !s.status.PrivacyMode
	s.mu.RUnlock()
	if !enabled {
		return
	}
	if policy.CaptureOnWindowChange {
		return
	}
	if _, ok := appCaptureProfileForContext(s.currentContext(), policy); ok {
		return
	}
	s.CaptureTimeMachineNow()
}

func (s *Service) captureIfWindowChanged() {
	s.mu.RLock()
	policy := s.policy
	intervalSeconds := int(s.interval.Seconds())
	enabled := s.status.Enabled && s.status.TimeMachineEnabled && !s.status.PrivacyMode && (policy.CaptureOnWindowChange || hasEnabledAppCaptureProfiles(policy))
	lastSignature := s.lastWindowSignature
	s.mu.RUnlock()
	if !enabled {
		return
	}

	context := s.currentContext()
	signature := windowSignature(context)
	if signature == "" {
		return
	}

	now := s.now().Unix()
	if lastSignature == "" {
		profile, hasProfile := windowCaptureProfileForContext(context, policy, intervalSeconds)
		s.mu.Lock()
		if s.lastWindowSignature == "" {
			s.lastWindowSignature = signature
			s.lastWindowContext = context
			s.status.LastWindowSwitchAt = now
			s.startWindowSessionLocked(signature, now)
			if hasProfile {
				if s.lastAppCaptureAt == nil {
					s.lastAppCaptureAt = map[string]int64{}
				}
				s.lastAppCaptureAt[signature] = now
				s.pendingWindowSignature = signature
				s.pendingWindowDueAt = now + int64(profile.WindowSwitchDelaySeconds)
			}
		}
		s.mu.Unlock()
		if hasProfile && profile.WindowSwitchDelaySeconds <= 0 {
			s.captureAppProfileIfDue(signature, profile, now)
		}
		return
	}
	if signature == lastSignature {
		s.mu.Lock()
		if signature == s.lastWindowSignature {
			s.lastWindowContext = context
		}
		s.mu.Unlock()
		if profile, ok := windowCaptureProfileForContext(context, policy, intervalSeconds); ok {
			s.captureAppProfileIfDue(signature, profile, now)
		} else if policy.CaptureOnWindowChange {
			s.capturePendingWindowSwitchIfDue(signature, now)
		}
		return
	}

	if profile, ok := windowCaptureProfileForContext(context, policy, intervalSeconds); ok {
		changed := false
		s.mu.Lock()
		if signature != s.lastWindowSignature {
			changed = true
			s.lastWindowSignature = signature
			s.lastWindowContext = context
			s.status.LastWindowSwitchAt = now
			s.startWindowSessionLocked(signature, now)
			if s.lastAppCaptureAt == nil {
				s.lastAppCaptureAt = map[string]int64{}
			}
			s.lastAppCaptureAt[signature] = now
			s.pendingWindowSignature = signature
			s.pendingWindowDueAt = now + int64(profile.WindowSwitchDelaySeconds)
		}
		s.mu.Unlock()
		if changed {
			s.reviewPendingCapturesAsync()
		}
		if profile.WindowSwitchDelaySeconds <= 0 {
			s.captureAppProfileIfDue(signature, profile, now)
		}
		return
	}

	if !policy.CaptureOnWindowChange {
		changed := false
		s.mu.Lock()
		if signature != s.lastWindowSignature {
			changed = true
			s.lastWindowSignature = signature
			s.lastWindowContext = context
			s.status.LastWindowSwitchAt = now
			s.pendingWindowSignature = ""
			s.pendingWindowDueAt = 0
		}
		s.mu.Unlock()
		if changed {
			s.reviewPendingCapturesAsync()
		}
		return
	}

	cooldown := policy.WindowChangeCooldown
	if cooldown <= 0 {
		cooldown = 30
	}
	s.mu.Lock()
	if signature == s.lastWindowSignature {
		s.mu.Unlock()
		return
	}
	s.lastWindowSignature = signature
	s.lastWindowContext = context
	s.status.LastWindowSwitchAt = now
	s.startWindowSessionLocked(signature, now)
	s.pendingWindowSignature = signature
	s.pendingWindowDueAt = now + int64(cooldown)
	s.mu.Unlock()
	s.reviewPendingCapturesAsync()

	if cooldown > 0 {
		return
	}
	s.capturePendingWindowSwitchIfDue(signature, now)
}

func (s *Service) capturePendingWindowSwitchIfDue(signature string, now int64) {
	s.mu.RLock()
	pendingSignature := s.pendingWindowSignature
	pendingDueAt := s.pendingWindowDueAt
	s.mu.RUnlock()
	if pendingSignature != signature || pendingDueAt <= 0 || now < pendingDueAt {
		return
	}

	entry := s.CaptureTimeMachineWindowSwitch()
	s.mu.Lock()
	if s.pendingWindowSignature == signature {
		s.pendingWindowSignature = ""
		s.pendingWindowDueAt = 0
	}
	if entry.ID != "" {
		s.lastWindowCaptureAt = entry.CreatedAt
		s.status.LastWindowSwitchCaptureAt = entry.CreatedAt
	}
	s.mu.Unlock()
}

func (s *Service) captureAppProfileIfDue(signature string, profile AppCaptureProfile, now int64) {
	s.mu.RLock()
	pendingSignature := s.pendingWindowSignature
	pendingDueAt := s.pendingWindowDueAt
	lastCaptureAt := s.lastAppCaptureAt[signature]
	s.mu.RUnlock()

	if pendingSignature == signature && pendingDueAt > 0 {
		if now < pendingDueAt {
			return
		}
		entry := s.captureTimeMachineAppProfile(profile, "在窗口切换稳定后记录当前屏幕")
		statusAt := now
		if entry.ID != "" {
			statusAt = entry.CreatedAt
		}
		s.recordAppProfileCaptureAttempt(signature, now, entry.ID != "", true, statusAt)
		return
	}

	interval := profile.ActiveIntervalSeconds
	if interval <= 0 {
		return
	}
	if lastCaptureAt <= 0 {
		s.recordAppProfileCaptureAttempt(signature, now, false, false, now)
		return
	}
	if now-lastCaptureAt < int64(interval) {
		return
	}
	entry := s.captureTimeMachineAppProfile(profile, "按应用驻留间隔记录当前屏幕")
	statusAt := now
	if entry.ID != "" {
		statusAt = entry.CreatedAt
	}
	s.recordAppProfileCaptureAttempt(signature, now, entry.ID != "", false, statusAt)
}

func (s *Service) captureTimeMachineAppProfile(profile AppCaptureProfile, trigger string) Entry {
	label := firstNonEmpty(profile.DisplayName, profile.ProcessName, profile.ID, "当前应用")
	return s.captureScreen("time_machine", "屏幕时间机器应用策略记录", fmt.Sprintf("应用 %s %s。", label, trigger))
}

func (s *Service) recordAppProfileCaptureAttempt(signature string, at int64, captured bool, windowSwitch bool, statusAt int64) {
	if at <= 0 {
		at = s.now().Unix()
	}
	if statusAt <= 0 {
		statusAt = at
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastAppCaptureAt == nil {
		s.lastAppCaptureAt = map[string]int64{}
	}
	s.lastAppCaptureAt[signature] = at
	if s.pendingWindowSignature == signature {
		s.pendingWindowSignature = ""
		s.pendingWindowDueAt = 0
	}
	if captured {
		s.lastWindowCaptureAt = statusAt
		if windowSwitch {
			s.status.LastWindowSwitchCaptureAt = statusAt
		}
	}
}

func (s *Service) startWindowSessionLocked(signature string, startedAt int64) {
	s.currentWindowSessionSignature = signature
	s.currentWindowSessionEntryID = ""
	s.currentWindowSessionStartedAt = startedAt
}

func (s *Service) startDraftSchedulerLocked() {
	if s.stopDraftScheduler != nil {
		s.scheduledDrafts.Running = true
		return
	}
	stop := make(chan struct{})
	interval := s.draftScheduleInterval
	if interval <= 0 {
		interval = 240 * time.Minute
	}
	s.stopDraftScheduler = stop
	s.scheduledDrafts.Running = true
	go s.runDraftScheduler(stop, interval)
}

func (s *Service) restartDraftSchedulerLocked() {
	s.stopDraftSchedulerLocked()
	s.startDraftSchedulerLocked()
}

func (s *Service) stopDraftSchedulerLocked() {
	if s.stopDraftScheduler == nil {
		s.scheduledDrafts.Running = false
		return
	}
	close(s.stopDraftScheduler)
	s.stopDraftScheduler = nil
	s.scheduledDrafts.Running = false
}

func (s *Service) runDraftScheduler(stop <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.runScheduledDrafts(false)
		}
	}
}

func (s *Service) setIntervalLocked(intervalSeconds int) {
	if intervalSeconds < 10 {
		intervalSeconds = int(defaultAutoCaptureInterval.Seconds())
	}
	s.interval = time.Duration(intervalSeconds) * time.Second
	s.status.AutoCaptureIntervalSeconds = intervalSeconds
}

func (s *Service) refreshEntryCountLocked() {
	s.status.EntryCount = len(s.entries)
	s.status.StoragePath = firstNonEmpty(appdb.DatabasePathForPath(s.path), s.path)
}

func (s *Service) statusSnapshotLocked() Status {
	status := s.status
	status.WorkerRunning = s.stopWorker != nil
	status.AutoOCREnabled = s.policy.AutoOCR
	status.AppCaptureProfiles = cloneAppCaptureProfiles(s.policy.AppCaptureProfiles)
	status.EntryCount = len(s.entries)
	status.StoragePath = firstNonEmpty(appdb.DatabasePathForPath(s.path), s.path)
	return status
}

func buildHealthSummary(entries []Entry, status Status, now time.Time) HealthSummary {
	if now.IsZero() {
		now = time.Now()
	}
	generatedAt := now.Unix()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	summary := HealthSummary{
		OK:                true,
		Total:             len(entries),
		LastCaptureAt:     status.LastCaptureAt,
		LastAutoOCRAt:     status.LastAutoOCRAt,
		LastSkippedReason: strings.TrimSpace(status.LastSkippedReason),
		LastAutoOCRError:  strings.TrimSpace(status.LastAutoOCRError),
		GeneratedAt:       generatedAt,
	}
	appStats := map[string]*HealthAppStat{}
	events := []HealthRecentEvent{}

	for _, entry := range entries {
		if entry.CreatedAt >= todayStart {
			summary.Today++
		}
		appName := strings.TrimSpace(entry.AppName)
		if appName == "" {
			appName = strings.TrimSpace(entry.Source)
		}
		if appName == "" {
			appName = "Unknown"
		}
		stat := appStats[strings.ToLower(appName)]
		if stat == nil {
			stat = &HealthAppStat{AppName: appName}
			appStats[strings.ToLower(appName)] = stat
		}
		stat.Count++
		if entry.CreatedAt > stat.LastSeenAt {
			stat.LastSeenAt = entry.CreatedAt
		}

		hasImage := entryHasImage(entry)
		if hasImage {
			summary.Images++
		}
		if len(entry.Frames) > 1 || entry.FrameCount > 1 {
			summary.MultiFrame++
		}
		if entry.MergedCount > 0 {
			summary.CollapsedEntries++
			summary.RemovedFrames += entry.MergedCount
		}
		if strings.Contains(entry.QualityReason, "重复") && entry.MergedCount == 0 {
			summary.CollapsedEntries++
		}
		if entry.Sensitive {
			summary.Sensitive++
			summary.SkippedSensitive++
			stat.Sensitive++
			events = append(events, healthEventFromEntry(entry, "sensitive", "敏感记录已隔离", entry.QualityReason))
		}
		if entryQualityOCRDone(entry) {
			summary.QualityOCRDone++
			stat.QualityOCR++
		} else if entryQualityOCRFailed(entry) {
			summary.QualityOCRFailed++
			events = append(events, healthEventFromEntry(entry, "quality_ocr_failed", "质检 OCR 需要重跑", entry.QualityOCRStatus))
		} else if entryQualityPending(entry) && status.AutoOCREnabled && hasImage && !entry.Sensitive && !entryQualityOCRBlocked(entry) {
			summary.QualityOCRPending++
		}
		if entryQualityPending(entry) {
			summary.Pending++
			summary.SkippedPending++
			stat.Pending++
			pendingTitle := "等待质检后再沉淀"
			if entryQualityOCRDone(entry) {
				pendingTitle = "已完成质检 OCR，等待质检"
			}
			events = append(events, healthEventFromEntry(entry, "pending", pendingTitle, entry.QualityReason))
			continue
		}
		if normalizeQualityStatus(entry.QualityStatus) == qualityStatusChecked {
			summary.Checked++
			stat.Checked++
			if entry.QualityCheckedAt > summary.LastQualityCheckAt {
				summary.LastQualityCheckAt = entry.QualityCheckedAt
			}
		}
		if entryOCRDone(entry) {
			summary.OCRDone++
			stat.OCRDone++
		} else if entryOCRFailed(entry) {
			summary.OCRFailed++
			events = append(events, healthEventFromEntry(entry, "ocr_failed", "OCR 需要重跑", entry.OCRStatus))
		} else if hasImage && !entry.Sensitive && !entryOCRBlocked(entry) {
			summary.OCRPending++
			events = append(events, healthEventFromEntry(entry, "ocr_pending", "截图等待 OCR", entry.QualityReason))
		}
	}

	if status.LastSkippedReason != "" {
		events = append(events, HealthRecentEvent{
			Kind:      "skipped",
			Title:     "最近一次采集被跳过",
			Detail:    strings.TrimSpace(status.LastSkippedReason),
			CreatedAt: status.LastSkippedAt,
		})
	}
	if status.LastAutoOCRError != "" {
		events = append(events, HealthRecentEvent{
			Kind:      "ocr_error",
			Title:     "最近一次自动 OCR 异常",
			Detail:    strings.TrimSpace(status.LastAutoOCRError),
			CreatedAt: status.LastAutoOCRAt,
		})
	}

	summary.AppStats = make([]HealthAppStat, 0, len(appStats))
	for _, stat := range appStats {
		summary.AppStats = append(summary.AppStats, *stat)
	}
	sort.SliceStable(summary.AppStats, func(i, j int) bool {
		if summary.AppStats[i].Count != summary.AppStats[j].Count {
			return summary.AppStats[i].Count > summary.AppStats[j].Count
		}
		return summary.AppStats[i].LastSeenAt > summary.AppStats[j].LastSeenAt
	})
	if len(summary.AppStats) > 6 {
		summary.AppStats = summary.AppStats[:6]
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].CreatedAt > events[j].CreatedAt
	})
	for _, event := range events {
		if event.Title == "" {
			continue
		}
		summary.RecentEvents = append(summary.RecentEvents, event)
		if len(summary.RecentEvents) >= 6 {
			break
		}
	}

	switch {
	case summary.Pending > 0:
		if summary.QualityOCRDone > 0 {
			summary.Message = fmt.Sprintf("还有 %d 条采集等待质检，其中 %d 条已完成质检 OCR", summary.Pending, summary.QualityOCRDone)
		} else {
			summary.Message = fmt.Sprintf("还有 %d 条采集等待质检，心流会先跳过这些记录", summary.Pending)
		}
	case summary.OCRFailed > 0 || summary.LastAutoOCRError != "":
		summary.Message = "最近 OCR 有异常，建议重跑 OCR + 总结后再沉淀"
	case summary.OCRPending > 0:
		summary.Message = fmt.Sprintf("还有 %d 张截图等待 OCR，后台会继续处理", summary.OCRPending)
	case summary.CollapsedEntries > 0:
		summary.Message = fmt.Sprintf("已折叠 %d 组重复画面，保留更完整的最后一帧", summary.CollapsedEntries)
	default:
		summary.Message = "采集健康，后台会继续质检、OCR 和清理"
	}
	return summary
}

func qualityReviewReason(entry Entry) string {
	switch {
	case entryQualityOCRDone(entry):
		return "自动质检通过 · OCR 预处理完成"
	case entryQualityOCRFailed(entry):
		return "自动质检通过 · OCR 预处理异常，保留截图证据"
	case entryQualityOCRBlocked(entry):
		return "自动质检通过 · OCR 写回被安全规则阻止"
	case entryQualityOCREmpty(entry):
		return "自动质检通过 · OCR 未识别到文字"
	default:
		return "自动质检通过"
	}
}

func needsQualityOCRPrecheck(entry Entry, policy CapturePolicy) bool {
	if !policy.AutoOCR || entry.ID == "" || entry.Sensitive || !entryHasImage(entry) {
		return false
	}
	if entry.OCRText != "" || entry.OCRStatus != "" || entry.QualityOCRText != "" || entry.QualityOCRStatus != "" {
		return false
	}
	return true
}

func needsQualityOCRPromotion(entry Entry) bool {
	if entry.ID == "" || entry.OCRText != "" || entry.OCRStatus != "" {
		return false
	}
	return entry.QualityOCRText != "" || entry.QualityOCRStatus != ""
}

func (s *Service) promoteQualityOCR(entry Entry) Entry {
	if entry.ID == "" || entry.OCRText != "" || entry.OCRStatus != "" {
		return Entry{}
	}
	status := strings.TrimSpace(entry.QualityOCRStatus)
	switch {
	case entry.QualityOCRText != "":
		return s.ApplyOCRText(entry.ID, entry.QualityOCRText, qualityOCRProvider(status))
	case status == "empty":
		return s.ApplyOCRText(entry.ID, "", "")
	case strings.HasPrefix(status, "failed:") || strings.HasPrefix(status, "blocked_"):
		return s.ApplyOCRText(entry.ID, "", status)
	default:
		return Entry{}
	}
}

func qualityOCRProvider(status string) string {
	status = strings.TrimSpace(status)
	if strings.HasPrefix(status, "done:") {
		return strings.TrimSpace(strings.TrimPrefix(status, "done:"))
	}
	return ""
}

func healthEventFromEntry(entry Entry, kind string, title string, detail string) HealthRecentEvent {
	if detail == "" {
		detail = entry.Summary
	}
	return HealthRecentEvent{
		ID:        entry.ID,
		Kind:      kind,
		Title:     title,
		Detail:    strings.TrimSpace(detail),
		AppName:   strings.TrimSpace(entry.AppName),
		CreatedAt: entry.CreatedAt,
	}
}

func (s *Service) scheduledDraftStatusLocked() ScheduledDraftStatus {
	status := s.scheduledDrafts
	policy := normalizeDraftSchedulePolicy(s.draftSchedule)
	status.Enabled = policy.Enabled
	status.Running = s.stopDraftScheduler != nil
	status.IntervalMinutes = policy.IntervalMinutes
	status.DailyDraftEnabled = policy.DailyDraftEnabled
	status.RetrospectiveEnabled = policy.RetrospectiveEnabled
	status.ExperienceReportEnabled = policy.ExperienceReportEnabled
	status.LastAutonomousRunAt = s.lastAutonomousRunAt
	return status
}

func (s *Service) mergeAutonomousArtifactsLocked(items []AutonomousArtifact) {
	if len(items) == 0 {
		return
	}
	byID := map[string]int{}
	for index, artifact := range s.autonomousArtifacts {
		byID[artifact.ID] = index
	}
	for _, item := range items {
		item = normalizeAutonomousArtifact(item, s.now())
		if item.ID == "" || item.Kind == "" {
			continue
		}
		if index, ok := byID[item.ID]; ok {
			s.autonomousArtifacts[index] = item
			continue
		}
		byID[item.ID] = len(s.autonomousArtifacts)
		s.autonomousArtifacts = append(s.autonomousArtifacts, item)
	}
	sort.SliceStable(s.autonomousArtifacts, func(i, j int) bool {
		return s.autonomousArtifacts[i].CreatedAt > s.autonomousArtifacts[j].CreatedAt
	})
}

func (s *Service) processAutoOCR(entry Entry, policy CapturePolicy) Entry {
	if !policy.AutoOCR || entry.ID == "" || entry.ImagePath == "" {
		return entry
	}
	s.mu.RLock()
	processor := s.autoOCR
	s.mu.RUnlock()
	if processor == nil {
		s.recordAutoOCRResult(entry.ID, "OCR 服务未接入")
		return entry
	}
	updated := processor(entry)
	if updated.ID == "" {
		s.recordAutoOCRResult(entry.ID, "OCR 未返回工作记忆更新")
		return entry
	}
	resultStatus := autoOCRResultStatus(updated)
	if strings.HasPrefix(resultStatus, "failed:") {
		s.recordAutoOCRResult(updated.ID, strings.TrimSpace(strings.TrimPrefix(resultStatus, "failed:")))
		return updated
	}
	if resultStatus == "blocked_sensitive" {
		s.recordAutoOCRResult(updated.ID, "敏感条目默认不执行 OCR")
		return updated
	}
	if strings.HasPrefix(resultStatus, "blocked_excluded") {
		s.recordAutoOCRResult(updated.ID, "排除规则阻止 OCR 写回")
		return updated
	}
	s.recordAutoOCRResult(updated.ID, "")
	return updated
}

func autoOCRResultStatus(entry Entry) string {
	if entryQualityPending(entry) && entry.QualityOCRStatus != "" {
		return strings.TrimSpace(entry.QualityOCRStatus)
	}
	return strings.TrimSpace(entry.OCRStatus)
}

func (s *Service) recordAutoOCRResult(id string, errText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastAutoOCRAt = s.now().Unix()
	s.status.LastAutoOCRID = id
	s.status.LastAutoOCRError = strings.TrimSpace(errText)
}

func (s *Service) recordSkippedCapture(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.PauseReason = reason
	s.status.LastSkippedAt = s.now().Unix()
	s.status.LastSkippedReason = reason
}

func (s *Service) recordActivitySnapshot(snapshot activitySnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot.IdleSeconds >= 0 {
		s.status.IdleSeconds = snapshot.IdleSeconds
	}
	if snapshot.LastActivityAt > 0 {
		s.status.LastActivityAt = snapshot.LastActivityAt
	}
	s.status.SessionLocked = snapshot.SessionLocked
	if snapshot.Error != "" {
		s.status.LastCaptureError = snapshot.Error
	}
}

func (s *Service) currentContext() windowContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentContextLocked()
}

func (s *Service) currentContextLocked() windowContext {
	if s.context == nil {
		return windowContext{}
	}
	return s.context()
}

func (s *Service) initFTSIndex() {
	if s.path == "" {
		return
	}
	if s.fts != nil {
		return
	}
	index, err := openFTSIndex(ftsPathForMemoryPath(s.path))
	if err != nil {
		s.ftsError = err.Error()
		return
	}
	s.fts = index
	if err := s.rebuildFTSLocked(); err != nil {
		s.ftsError = err.Error()
	} else {
		s.ftsError = ""
	}
}

func (s *Service) ensureFTSReady() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fts != nil || s.path == "" || len(s.entries) == 0 {
		s.flushDirtyFTSLocked()
		return
	}
	s.initFTSIndex()
}

func (s *Service) rebuildFTSLocked() error {
	if s.fts == nil {
		return nil
	}
	return s.fts.Rebuild(cloneEntries(s.entries))
}

func (s *Service) scheduleFTSRebuildLocked() {
	if s.path == "" || s.ftsRebuildDisabled {
		return
	}
	if s.fts == nil {
		if len(s.entries) == 0 {
			return
		}
		s.initFTSIndex()
		return
	}
	s.ftsDirty = true
	if s.ftsRebuildScheduled {
		return
	}
	delay := ftsRebuildDebounceDelay
	if delay <= 0 {
		s.flushDirtyFTSLocked()
		return
	}
	s.ftsRebuildScheduled = true
	time.AfterFunc(delay, s.runScheduledFTSRebuild)
}

func (s *Service) runScheduledFTSRebuild() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ftsRebuildScheduled = false
	if s.ftsRebuildDisabled {
		return
	}
	s.flushDirtyFTSLocked()
}

func (s *Service) flushDirtyFTSLocked() {
	if !s.ftsDirty {
		return
	}
	if s.fts == nil {
		if s.path == "" || len(s.entries) == 0 {
			s.ftsDirty = false
			return
		}
		s.initFTSIndex()
		if s.ftsError == "" {
			s.ftsDirty = false
		}
		return
	}
	if err := s.rebuildFTSLocked(); err != nil {
		s.ftsError = err.Error()
		return
	}
	s.ftsDirty = false
	s.ftsError = ""
}

func (s *Service) recordFTSError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.ftsError = ""
		return
	}
	s.ftsError = err.Error()
}

func (s *Service) load() bool {
	if s.path == "" {
		return false
	}
	state, ok, err := loadMemoryStateFromSQLite(s.path, s.now)
	if err != nil || !ok {
		return false
	}
	s.entries = nil
	s.decisions = map[string]ExperienceDecision{}
	s.autonomousArtifacts = nil
	s.autonomousRejections = map[string]AutonomousRejection{}
	for key, decision := range state.ExperienceDecisions {
		decision = normalizeExperienceDecision(key, decision)
		if decision.InsightID != "" && decision.Status != "" {
			s.decisions[decision.InsightID] = decision
		}
	}
	for _, artifact := range state.AutonomousArtifacts {
		artifact = normalizeAutonomousArtifact(artifact, s.now())
		if artifact.ID != "" && artifact.Kind != "" {
			s.autonomousArtifacts = append(s.autonomousArtifacts, artifact)
		}
	}
	for key, rejection := range state.AutonomousRejections {
		key = strings.TrimSpace(strings.ToLower(firstNonEmpty(key, rejection.Key)))
		if key == "" {
			continue
		}
		rejection.Key = key
		s.autonomousRejections[key] = rejection
	}
	s.lastAutonomousRunAt = state.LastAutonomousRunAt
	for _, entry := range state.Entries {
		entry = normalizeEntry(entry)
		if entry.ID != "" {
			s.entries = append(s.entries, entry)
		}
	}
	sortEntries(s.entries)
	s.trimLocked()
	return true
}

func (s *Service) saveLockedWithStatus() {
	s.saveError = ""
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
		s.status.LastCaptureError = err.Error()
		return
	}
	s.scheduleFTSRebuildLocked()
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	state := memoryState{
		Entries:              s.entries,
		ExperienceDecisions:  cloneExperienceDecisions(s.decisions),
		AutonomousArtifacts:  cloneAutonomousArtifacts(s.autonomousArtifacts),
		AutonomousRejections: cloneAutonomousRejections(s.autonomousRejections),
		LastAutonomousRunAt:  s.lastAutonomousRunAt,
	}
	return saveMemoryStateToSQLite(s.path, state)
}

func (s *Service) trimLocked() {
	for len(s.entries) > s.maxEntries {
		remove := len(s.entries) - 1
		for i := len(s.entries) - 1; i >= 0; i-- {
			if !s.entries[i].Favorite {
				remove = i
				break
			}
		}
		s.entries = append(s.entries[:remove], s.entries[remove+1:]...)
	}
}

func seedEntries() []Entry {
	return []Entry{
		{
			ID:          "memory-gateway",
			Source:      "clipboard",
			ContentType: "issue_note",
			Title:       "网关代理异常排查记录",
			Summary:     "WiFi 代理失败优先确认默认网关是否指向 OpenWrt 192.168.1.10。",
			Text:        "Cloudflare Tunnel 入口正常，OpenWrt 网关疑似仍指向 192.168.1.1。",
			WindowTitle: "Windows Terminal",
			AppName:     "Terminal",
			Tags:        []string{"网络", "证据"},
			Favorite:    true,
			Sensitive:   false,
			CreatedAt:   1770000000,
		},
	}
}

func latestCapture(entries []capturehistory.Entry, source string) (capturehistory.Entry, bool) {
	var newest capturehistory.Entry
	for _, entry := range entries {
		if source != "" && entry.Source != source {
			continue
		}
		if newest.ID == "" || entry.CreatedAt > newest.CreatedAt {
			newest = entry
		}
	}
	return newest, newest.ID != ""
}

func imageFingerprint(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return ""
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return ""
	}

	var cells [64]int
	total := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			index := y*8 + x
			cells[index] = sampledCellLuma(img, bounds, x, y)
			total += cells[index]
		}
	}
	average := (total + len(cells)/2) / len(cells)
	hash := uint64(0)
	for index, value := range cells {
		if value >= average {
			hash |= uint64(1) << uint(63-index)
		}
	}
	return fmt.Sprintf("%02x:%016x", clampByte(average), hash)
}

func sampledCellLuma(img image.Image, bounds image.Rectangle, cellX int, cellY int) int {
	width := bounds.Dx()
	height := bounds.Dy()
	total := 0
	for sampleY := 0; sampleY < 4; sampleY++ {
		for sampleX := 0; sampleX < 4; sampleX++ {
			gridX := cellX*4 + sampleX
			gridY := cellY*4 + sampleY
			x := bounds.Min.X + ((gridX*2+1)*width)/(8*4*2)
			y := bounds.Min.Y + ((gridY*2+1)*height)/(8*4*2)
			if x >= bounds.Max.X {
				x = bounds.Max.X - 1
			}
			if y >= bounds.Max.Y {
				y = bounds.Max.Y - 1
			}
			r, g, b, _ := img.At(x, y).RGBA()
			total += (299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8) + 500) / 1000
		}
	}
	return (total + 8) / 16
}

func similarImageFingerprint(existing Entry, incoming Entry) bool {
	existingAverage, existingHash, ok := parseImageFingerprint(existing.ImageFingerprint)
	if !ok {
		return false
	}
	incomingAverage, incomingHash, ok := parseImageFingerprint(incoming.ImageFingerprint)
	if !ok {
		return false
	}
	if !similarImageDimensions(existing, incoming) {
		return false
	}
	if absInt(existingAverage-incomingAverage) > 18 {
		return false
	}
	return bits.OnesCount64(existingHash^incomingHash) <= similarImageHashMaxBits
}

func collapseRedundantFrames(entry *Entry) int {
	if entry == nil || len(entry.Frames) <= 1 {
		if entry != nil && entry.FrameCount <= 0 && len(entry.Frames) == 1 {
			entry.FrameCount = 1
		}
		return 0
	}
	if !framesAllRedundant(entry.Frames) {
		entry.FrameCount = len(entry.Frames)
		return 0
	}
	originalCount := len(entry.Frames)
	last := entry.Frames[len(entry.Frames)-1]
	removed := len(entry.Frames) - 1
	applyFrameToEntry(entry, last)
	entry.Frames = []CaptureFrame{last}
	entry.FrameCount = 1
	entry.MergedCount += removed
	entry.LastMergedAt = last.CreatedAt
	entry.QualityReason = fmt.Sprintf("自动质检：%d 帧重复或近似重复，保留最后一帧", originalCount)
	entry.Summary = firstNonEmpty(entry.Summary, "重复画面已自动折叠")
	if !strings.Contains(entry.Summary, "重复画面") {
		entry.Summary = entry.Summary + "（重复画面已自动折叠）"
	}
	entry.Tags = cleanStrings(append(entry.Tags, "重复画面合并", "自动质检"))
	return removed
}

func framesAllRedundant(frames []CaptureFrame) bool {
	if len(frames) <= 1 {
		return false
	}
	base := entryFromFrameForCompare(frames[0])
	for _, frame := range frames[1:] {
		next := entryFromFrameForCompare(frame)
		if base.ImageSignature != "" && next.ImageSignature != "" && base.ImageSignature == next.ImageSignature {
			continue
		}
		if similarImageFingerprint(base, next) {
			continue
		}
		return false
	}
	return true
}

func entryFromFrameForCompare(frame CaptureFrame) Entry {
	return Entry{
		ImageSignature:   strings.TrimSpace(frame.ImageSignature),
		ImageFingerprint: strings.TrimSpace(frame.ImageFingerprint),
		Width:            frame.Width,
		Height:           frame.Height,
	}
}

func parseImageFingerprint(value string) (int, uint64, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	average, err := strconv.ParseUint(parts[0], 16, 8)
	if err != nil {
		return 0, 0, false
	}
	hash, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		return 0, 0, false
	}
	return int(average), hash, true
}

func similarImageDimensions(existing Entry, incoming Entry) bool {
	if existing.Width <= 0 || existing.Height <= 0 || incoming.Width <= 0 || incoming.Height <= 0 {
		return false
	}
	widthTolerance := maxInt(2, maxInt(existing.Width, incoming.Width)/50)
	heightTolerance := maxInt(2, maxInt(existing.Height, incoming.Height)/50)
	return absInt(existing.Width-incoming.Width) <= widthTolerance &&
		absInt(existing.Height-incoming.Height) <= heightTolerance
}

func clampByte(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func normalizeEntry(entry Entry) Entry {
	entry.ID = strings.TrimSpace(entry.ID)
	entry.Source = strings.TrimSpace(entry.Source)
	entry.ContentType = strings.TrimSpace(entry.ContentType)
	entry.Title = strings.TrimSpace(entry.Title)
	entry.Summary = strings.TrimSpace(entry.Summary)
	entry.Text = strings.TrimSpace(entry.Text)
	entry.OCRText = strings.TrimSpace(entry.OCRText)
	entry.OCRStatus = strings.TrimSpace(entry.OCRStatus)
	entry.QualityOCRText = strings.TrimSpace(entry.QualityOCRText)
	entry.QualityOCRStatus = strings.TrimSpace(entry.QualityOCRStatus)
	entry.WindowTitle = strings.TrimSpace(entry.WindowTitle)
	entry.AppName = strings.TrimSpace(entry.AppName)
	entry.CaptureID = strings.TrimSpace(entry.CaptureID)
	entry.ImagePath = strings.TrimSpace(entry.ImagePath)
	entry.ImageSignature = strings.TrimSpace(entry.ImageSignature)
	entry.ImageFingerprint = strings.TrimSpace(entry.ImageFingerprint)
	entry.QualityStatus = normalizeQualityStatus(entry.QualityStatus)
	entry.QualityReason = strings.TrimSpace(entry.QualityReason)
	entry.Frames = normalizeCaptureFrames(entry.Frames)
	if len(entry.Frames) > 0 {
		entry.FrameCount = len(entry.Frames)
	}
	if entry.FrameCount < 0 {
		entry.FrameCount = 0
	}
	if entry.QualityCheckedAt < 0 {
		entry.QualityCheckedAt = 0
	}
	if entry.Source == "" {
		entry.Source = "manual_note"
	}
	if entry.ContentType == "" {
		entry.ContentType = "note"
	}
	if entry.Title == "" {
		entry.Title = "工作记忆"
	}
	if entry.Summary == "" {
		entry.Summary = firstNonEmpty(entry.Text, entry.OCRText)
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if entry.MergedCount < 0 {
		entry.MergedCount = 0
	}
	if entry.LastMergedAt < 0 {
		entry.LastMergedAt = 0
	}
	return enrichEntry(entry)
}

func normalizeCaptureFrames(frames []CaptureFrame) []CaptureFrame {
	if len(frames) == 0 {
		return nil
	}
	normalized := make([]CaptureFrame, 0, len(frames))
	for _, frame := range frames {
		frame.CaptureID = strings.TrimSpace(frame.CaptureID)
		frame.ImagePath = strings.TrimSpace(frame.ImagePath)
		frame.ImageSignature = strings.TrimSpace(frame.ImageSignature)
		frame.ImageFingerprint = strings.TrimSpace(frame.ImageFingerprint)
		frame.WindowTitle = strings.TrimSpace(frame.WindowTitle)
		frame.AppName = strings.TrimSpace(frame.AppName)
		if frame.CaptureID == "" && frame.ImagePath == "" && frame.ImageSignature == "" && frame.ImageFingerprint == "" {
			continue
		}
		normalized = append(normalized, frame)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeQualityStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case qualityStatusPending:
		return qualityStatusPending
	case qualityStatusChecked:
		return qualityStatusChecked
	default:
		return ""
	}
}

func entryQualityPending(entry Entry) bool {
	return normalizeQualityStatus(entry.QualityStatus) == qualityStatusPending
}

func entryHasImage(entry Entry) bool {
	return entry.ImagePath != "" || entry.CaptureID != "" || len(entry.Frames) > 0
}

func entryOCRDone(entry Entry) bool {
	status := strings.ToLower(strings.TrimSpace(entry.OCRStatus))
	return strings.TrimSpace(entry.OCRText) != "" || status == "done" || strings.HasPrefix(status, "done:")
}

func entryOCRFailed(entry Entry) bool {
	status := strings.ToLower(strings.TrimSpace(entry.OCRStatus))
	return strings.HasPrefix(status, "failed:") || strings.Contains(status, "error")
}

func entryOCRBlocked(entry Entry) bool {
	status := strings.ToLower(strings.TrimSpace(entry.OCRStatus))
	return strings.HasPrefix(status, "blocked_") || status == "empty"
}

func entryQualityOCRDone(entry Entry) bool {
	status := strings.ToLower(strings.TrimSpace(entry.QualityOCRStatus))
	return strings.TrimSpace(entry.QualityOCRText) != "" || status == "done" || strings.HasPrefix(status, "done:")
}

func entryQualityOCRFailed(entry Entry) bool {
	status := strings.ToLower(strings.TrimSpace(entry.QualityOCRStatus))
	return strings.HasPrefix(status, "failed:") || strings.Contains(status, "error")
}

func entryQualityOCRBlocked(entry Entry) bool {
	status := strings.ToLower(strings.TrimSpace(entry.QualityOCRStatus))
	return strings.HasPrefix(status, "blocked_")
}

func entryQualityOCREmpty(entry Entry) bool {
	return strings.EqualFold(strings.TrimSpace(entry.QualityOCRStatus), "empty")
}

func entryUsableForExtraction(entry Entry) bool {
	return entry.ID != "" && !entry.Sensitive && !entryQualityPending(entry)
}

func normalizeClipboardMemorySource(entry clipboardhistory.Entry) clipboardhistory.Entry {
	entry.ID = strings.TrimSpace(entry.ID)
	entry.Text = strings.TrimSpace(entry.Text)
	entry.ImagePath = strings.TrimSpace(entry.ImagePath)
	entry.Signature = strings.TrimSpace(entry.Signature)
	entry.ContentType = strings.TrimSpace(entry.ContentType)
	entry.Source = strings.TrimSpace(entry.Source)
	entry.Summary = strings.TrimSpace(entry.Summary)
	entry.Tags = cleanStrings(entry.Tags)
	if entry.Source == "" {
		entry.Source = "clipboard"
	}
	if entry.Type == "" {
		if entry.ImagePath != "" {
			entry.Type = clipboardhistory.EntryImage
		} else {
			entry.Type = clipboardhistory.EntryText
		}
	}
	if entry.ID == "" {
		identity := firstNonEmpty(entry.Signature, entry.Text, entry.ImagePath)
		if identity != "" {
			entry.ID = "clipboard-" + shortHash(identity)
		}
	}
	return entry
}

func normalizeCaptureMemorySource(entry capturehistory.Entry) capturehistory.Entry {
	entry.ID = strings.TrimSpace(entry.ID)
	entry.ImagePath = strings.TrimSpace(entry.ImagePath)
	entry.SavedPath = strings.TrimSpace(entry.SavedPath)
	entry.Source = strings.TrimSpace(entry.Source)
	entry.Signature = strings.TrimSpace(entry.Signature)
	entry.Actions = cleanStrings(entry.Actions)
	entry.Tags = cleanStrings(entry.Tags)
	if entry.Source == "" {
		entry.Source = "capture_history"
	}
	if entry.ID == "" {
		identity := firstNonEmpty(entry.Signature, entry.ImagePath)
		if identity != "" {
			entry.ID = "capture-" + shortHash(identity)
		}
	}
	return entry
}

func shouldSkipCaptureHistoryMemory(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "manual_capture", "time_machine":
		return true
	default:
		return false
	}
}

func entryMatches(entry Entry, query string) bool {
	parts := []string{
		entry.ID,
		entry.Source,
		entry.ContentType,
		entry.Title,
		entry.Summary,
		entry.Text,
		entry.OCRText,
		entry.OCRStatus,
		entry.WindowTitle,
		entry.AppName,
		entry.CaptureID,
		entry.ImagePath,
		fmt.Sprintf("%dx%d", entry.Width, entry.Height),
	}
	parts = append(parts, entry.Tags...)
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), query)
}

func scoreSearchMatch(entry Entry, query string) (float64, string) {
	if query == "" {
		if entry.Favorite {
			return 10, "timeline"
		}
		return 1, "timeline"
	}
	if entryMatches(entry, query) {
		score := 92.0
		if strings.Contains(strings.ToLower(entry.Title), query) {
			score += 8
		}
		if entry.Favorite {
			score += 10
		}
		return score, "keyword"
	}
	score := semanticSimilarity(query, entrySemanticText(entry))
	if score >= 0.18 {
		boost := 0.0
		if entry.Favorite {
			boost += 6
		}
		return 62 + score*28 + boost, "semantic_local"
	}
	return 0, ""
}

func scoreFTSMatch(entry Entry, hit ftsHit) float64 {
	rank := hit.Rank
	if rank < 0 {
		rank = -rank
	}
	if rank > 20 {
		rank = 20
	}
	score := 96 - rank
	if entry.Favorite {
		score += 10
	}
	return score
}

func ftsMatch(snippet string) string {
	snippet = strings.TrimSpace(snippet)
	if snippet == "" {
		return "fts"
	}
	return "fts:" + snippet
}

func entryToResult(entry Entry) contracts.SearchResult {
	return entryToResultWithMatch(entry, "")
}

func entryToResultWithMatch(entry Entry, match string) contracts.SearchResult {
	payload := map[string]interface{}{"entryId": entry.ID, "source": entry.Source}
	if entry.CaptureID != "" {
		payload["captureId"] = entry.CaptureID
	}
	if entry.ImagePath != "" {
		payload["imagePath"] = entry.ImagePath
	}
	meta := []contracts.LabelValue{
		{Label: "来源", Value: entry.Source},
		{Label: "应用", Value: entry.AppName},
		{Label: "类型", Value: entry.ContentType},
	}
	if entry.CaptureID != "" {
		meta = append(meta, contracts.LabelValue{Label: "截图", Value: entry.CaptureID})
	}
	if entry.MergedCount > 0 {
		meta = append(meta, contracts.LabelValue{Label: "重复画面", Value: fmt.Sprintf("已合并 %d 次", entry.MergedCount)})
	}
	if entry.OCRStatus != "" {
		meta = append(meta, contracts.LabelValue{Label: "OCR", Value: entry.OCRStatus})
	}
	previewText := entry.Text
	if entry.OCRText != "" {
		previewText = strings.TrimSpace(entry.Text + "\n\nOCR:\n" + entry.OCRText)
	}
	evidence := []contracts.LabelValue{
		{Label: "记忆 ID", Value: entry.ID},
		{Label: "敏感", Value: boolLabel(entry.Sensitive)},
		{Label: "匹配", Value: matchLabel(match)},
	}
	if snippet := matchSnippet(match); snippet != "" {
		evidence = append(evidence, contracts.LabelValue{Label: "命中", Value: snippet})
	}
	return contracts.SearchResult{
		ID:       entry.ID,
		Type:     contracts.ResultMemory,
		Title:    entry.Title,
		Subtitle: "工作记忆 · " + entry.AppName,
		Detail:   entry.Summary,
		Icon:     "memory",
		Tags:     entry.Tags,
		Payload:  payload,
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewMemory,
			Title:    entry.Title,
			Subtitle: entry.WindowTitle,
			Text:     previewText,
			Meta:     meta,
			Evidence: evidence,
		},
		Actions: []contracts.PreviewAction{
			contracts.CopyAction("copy_summary", "复制摘要", entry.Summary, "Enter"),
			{ID: "open_memory", Label: "打开记忆", Icon: "open", Kind: contracts.ActionOpen, Payload: map[string]interface{}{"entryId": entry.ID}},
			contracts.RememberAction("pin_review", "加入复盘", entry.ID),
			contracts.PluginAction("agent_task", "生成任务包", "agent-task:"+entry.ID),
		},
	}
}

func matchLabel(match string) string {
	if strings.HasPrefix(match, "fts") {
		return "SQLite FTS"
	}
	switch match {
	case "keyword":
		return "关键词"
	case "semantic_local":
		return "本地语义匹配"
	case "timeline":
		return "时间线"
	default:
		return "未标注"
	}
}

func matchSnippet(match string) string {
	if !strings.HasPrefix(match, "fts:") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(match, "fts:"))
}

type workflowDraftProfileResult struct {
	title     string
	trigger   string
	input     string
	steps     []WorkflowDraftStep
	output    string
	riskLevel string
}

type checklistDraftProfileResult struct {
	title   string
	context string
	items   []string
}

func workflowDraftProfile(entries []Entry, title string) workflowDraftProfileResult {
	text := entriesExperienceText(entries, title)
	switch {
	case containsAny(text, "hosts", "dns", "gateway", "openwrt", "host", "网关", "域名"):
		return workflowDraftProfileResult{
			title:     "Hosts 与网络切换候选工作流",
			trigger:   "需要切换 Hosts、DNS、网关或代理场景时，从启动器输入 hosts/network 相关关键词触发。",
			input:     "目标环境、域名/IP、可选备注；默认也可读取剪贴板中的 Hosts 片段。",
			output:    "生成 Hosts 预览和差异摘要；只有用户二次确认后才能应用。",
			riskLevel: "high",
			steps: []WorkflowDraftStep{
				{ID: "collect-target", Label: "收集目标环境和 Hosts 片段", Command: "clip text", RequiresConfirm: false},
				{ID: "validate-hosts", Label: "检查域名/IP 格式和冲突", Command: "hosts preview {input}", RequiresConfirm: false},
				{ID: "show-diff", Label: "展示 Ariadne marker 合并预览", Command: "hosts diff {prev}", RequiresConfirm: false},
				{ID: "apply-after-confirm", Label: "用户确认后应用系统 Hosts", Command: "hosts apply {prev}", RequiresConfirm: true},
			},
		}
	case containsAny(text, "json", "base64", "hash", "url", "clipboard", "workflow", "macro", "剪贴板", "格式化"):
		return workflowDraftProfileResult{
			title:     "剪贴板文本处理候选工作流",
			trigger:   "剪贴板中存在 JSON、URL、Base64、Hash 或日志片段时，从启动器输入 wf/clip 相关关键词触发。",
			input:     "默认读取剪贴板文本；也允许用户在启动器中输入覆盖值。",
			output:    "输出格式化或转换后的文本，并把结果复制回剪贴板。",
			riskLevel: "low",
			steps: []WorkflowDraftStep{
				{ID: "read-clipboard", Label: "读取剪贴板或启动器输入", Command: "clip text", RequiresConfirm: false},
				{ID: "normalize-text", Label: "清洗空白并识别 JSON/URL/编码类型", Command: "text normalize {input}", RequiresConfirm: false},
				{ID: "transform", Label: "执行格式化、解码或哈希转换", Command: "json format {prev}", RequiresConfirm: false},
				{ID: "copy-result", Label: "复制最终结果并记录本地反馈", Command: "copy {prev}", RequiresConfirm: false},
			},
		}
	case containsAny(text, "screenshot", "capture", "ocr", "image", "截图", "贴图", "识别", "文字"):
		return workflowDraftProfileResult{
			title:     "截图 OCR 整理候选工作流",
			trigger:   "需要从当前屏幕、截图历史或剪贴板图片提取文字时触发。",
			input:     "当前屏幕、选中的截图历史记录或剪贴板图片。",
			output:    "OCR 文本、可选行选择和工作记忆证据链接。",
			riskLevel: "medium",
			steps: []WorkflowDraftStep{
				{ID: "select-image", Label: "选择当前屏幕或已有图片证据", Command: "capture current", RequiresConfirm: false},
				{ID: "run-ocr", Label: "本地 OCR 识别并生成行级结果", Command: "ocr recognize {prev}", RequiresConfirm: false},
				{ID: "review-sensitive", Label: "检查敏感词和排除规则", Command: "memory sensitive-check {prev}", RequiresConfirm: false},
				{ID: "save-evidence", Label: "用户确认后写入工作记忆或复制选中文本", Command: "memory note {prev}", RequiresConfirm: true},
			},
		}
	default:
		return workflowDraftProfileResult{
			title:     "证据整理候选工作流",
			trigger:   "同类工作记忆重复出现时，由用户在经验发现中触发。",
			input:     "选中的工作记忆证据和用户补充目标。",
			output:    "一份可确认后保存为正式工作流的步骤草稿。",
			riskLevel: "medium",
			steps: []WorkflowDraftStep{
				{ID: "collect-evidence", Label: "收集并去重证据条目", Command: "memory collect {input}", RequiresConfirm: false},
				{ID: "summarize-intent", Label: "整理触发意图和输入输出", Command: "memory summarize {prev}", RequiresConfirm: false},
				{ID: "draft-steps", Label: "拆分为可审查步骤", Command: "workflow draft {prev}", RequiresConfirm: false},
				{ID: "review-save", Label: "用户确认后再保存为正式工作流", Command: "workflow save {prev}", RequiresConfirm: true},
			},
		}
	}
}

func checklistDraftProfile(entries []Entry, title string) checklistDraftProfileResult {
	text := entriesExperienceText(entries, title)
	switch {
	case containsAny(text, "hosts", "dns", "gateway", "openwrt", "proxy", "网关", "代理", "网络", "域名"):
		return checklistDraftProfileResult{
			title:   "网络与 Hosts 排查检查清单",
			context: "由工作记忆中的网络、代理、DNS 或 Hosts 证据生成，适合保存为排查前置清单。",
			items: []string{
				"确认当前网络、VPN、代理和网关路径，不只看 ping/TCP 可达性。",
				"检查 Hosts、DNS、OpenWrt/代理订阅是否存在冲突或覆盖。",
				"保留操作前预览、差异和可回滚记录。",
				"只在用户确认后应用系统 Hosts 或网络配置变更。",
				"把最终判断和证据写回工作记忆或知识草稿。",
			},
		}
	case containsAny(text, "build", "deploy", "wails", "vite", "test", "构建", "发布", "打包"):
		return checklistDraftProfileResult{
			title:   "构建发布检查清单",
			context: "由构建、发布、Wails/Vite 或测试相关证据生成，适合发布前复核。",
			items: []string{
				"确认本轮改动范围和未跟踪/无关文件，不回退用户改动。",
				"运行目标单测、前端构建和 Wails 构建，并记录产物元数据。",
				"检查 bindings 数量变化是否符合新增服务或模型。",
				"完成必要的 Computer Use 桌面烟测，不用浏览器模拟桌面行为。",
				"更新 Ariadne 台账中的验证命令、产物大小、时间和剩余缺口。",
			},
		}
	case containsAny(text, "auth", "login", "permission", "token", "secret", "登录", "权限", "鉴权", "密码"):
		return checklistDraftProfileResult{
			title:   "认证与敏感信息检查清单",
			context: "由登录、权限、token 或敏感内容证据生成，默认要求人工复核。",
			items: []string{
				"确认是否涉及密码、token、验证码、密钥或个人信息。",
				"敏感内容不进入外部 AI、embedding 或导出包，除非用户明确允许。",
				"先确认账号、权限边界和目标系统，再判断是否需要操作。",
				"记录脱敏后的错误现象、时间、窗口和证据来源。",
				"形成知识草稿时保留敏感检查结论，不保留明文凭据。",
			},
		}
	default:
		return checklistDraftProfileResult{
			title:   "经验沉淀检查清单",
			context: "由当前经验线索证据生成，适合把重复问题转为可复用流程。",
			items: []string{
				"确认这不是一次性事件，至少有两条独立证据支持。",
				"整理触发条件、输入、输出、风险和失败回滚方式。",
				"先生成知识草稿或复盘，再决定是否保存为工作流或 skill。",
				"所有执行类动作必须保留用户确认点。",
				"补充验收方式：单测、桌面验证或可复现操作记录。",
			},
		}
	}
}

func entriesExperienceText(entries []Entry, title string) string {
	parts := []string{title}
	for _, entry := range entries {
		parts = append(parts, entryExperienceText(entry))
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func containsAny(text string, terms ...string) bool {
	text = strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func discoverExperienceInsights(entries []Entry, now time.Time) []ExperienceInsight {
	insights := []ExperienceInsight{}
	insights = append(insights, discoverRepeatedIssueInsights(entries, now)...)
	insights = append(insights, discoverAutomationInsights(entries, now)...)
	insights = append(insights, discoverKnowledgeInsights(entries, now)...)
	sort.SliceStable(insights, func(i, j int) bool {
		if insights[i].Confidence != insights[j].Confidence {
			return insights[i].Confidence > insights[j].Confidence
		}
		return insights[i].Title < insights[j].Title
	})
	if len(insights) > 6 {
		insights = insights[:6]
	}
	return insights
}

func buildExperienceReport(entries []Entry, decisions map[string]ExperienceDecision, periodDays int, now time.Time) ExperienceReport {
	if periodDays <= 0 {
		periodDays = 7
	}
	insights := discoverExperienceInsights(entries, now)
	insights = applyExperienceDecisions(insights, decisions)
	evidence := map[string]bool{}
	for _, insight := range insights {
		for _, id := range insight.Evidence {
			evidence[id] = true
		}
	}
	summary := "本地规则暂未发现稳定模式；继续积累工作记忆后再生成经验发现。"
	if len(insights) > 0 {
		summary = fmt.Sprintf("发现 %d 条可解释的本地经验线索，所有建议都需要用户确认后才能转成工作流、skill 或外部代理任务。", len(insights))
	}
	return ExperienceReport{
		ID:            "experience-" + now.Format("20060102150405"),
		Title:         "经验发现报告",
		Summary:       summary,
		PeriodDays:    periodDays,
		EntryCount:    len(entries),
		EvidenceCount: len(evidence),
		Insights:      insights,
		GeneratedAt:   now.Unix(),
	}
}

func buildAutonomousArtifacts(entries []Entry, decisions map[string]ExperienceDecision, existing []AutonomousArtifact, rejections map[string]AutonomousRejection, lastRunAt int64, policy DraftSchedulePolicy, now time.Time, force bool) AutonomousRunResult {
	result := AutonomousRunResult{OK: true, CreatedAt: now.Unix()}
	if !force && sameLocalDay(lastRunAt, now) {
		result.OK = false
		result.Message = "今天已经执行过自主沉淀"
		return result
	}
	if len(entries) == 0 {
		result.Message = "没有足够证据生成自主产物"
		return result
	}
	policy = normalizeDraftSchedulePolicy(policy)
	existingKeys := activeAutonomousKeys(existing)
	add := func(artifact AutonomousArtifact) {
		artifact = normalizeAutonomousArtifact(artifact, now)
		if artifact.ID == "" || artifact.DedupKey == "" {
			result.Skipped++
			return
		}
		if existingKeys[artifact.DedupKey] {
			result.Skipped++
			return
		}
		if _, rejected := rejections[artifact.DedupKey]; rejected {
			result.Skipped++
			return
		}
		existingKeys[artifact.DedupKey] = true
		result.Artifacts = append(result.Artifacts, artifact)
	}

	if policy.DailyDraftEnabled {
		selected, todayCount, skippedSensitive := dailyDraftEntries(entries, now, 12)
		if todayCount >= 3 && len(selected) > 0 {
			body := renderDailyDraftBody(selected, now, todayCount, skippedSensitive)
			add(AutonomousArtifact{
				ID:       "auto-daily-" + now.Format("20060102"),
				Kind:     "daily",
				Title:    "今日自动摘要",
				Summary:  fmt.Sprintf("心流自动整理了今天 %d 条上下文。", todayCount),
				Body:     body,
				Evidence: entryIDs(selected, 0),
				DedupKey: "daily:" + now.Format("20060102"),
			})
		}
	}

	if policy.RetrospectiveEnabled {
		retrospectiveEntries := scheduledRetrospectiveEntries(entries, 12)
		if len(retrospectiveEntries) >= 3 {
			title := "自动复盘：" + shortDraftTitle(firstNonEmpty(retrospectiveEntries[0].Title, retrospectiveEntries[0].Summary, retrospectiveEntries[0].ID), 28)
			add(AutonomousArtifact{
				ID:       "auto-retro-" + shortHash(strings.Join(entryIDs(retrospectiveEntries, 0), ":")) + "-" + now.Format("20060102"),
				Kind:     "retrospective",
				Title:    title,
				Summary:  "发现一组适合复盘的连续问题或处理线索。",
				Body:     renderRetrospectiveDraftBody(retrospectiveEntries, now, len(retrospectiveEntries), 0),
				Evidence: entryIDs(retrospectiveEntries, 0),
				DedupKey: "retrospective:" + shortHash(strings.Join(entryIDs(retrospectiveEntries, 0), ":")),
			})
		}
	}

	if policy.ExperienceReportEnabled {
		report := buildExperienceReport(entries, decisions, policy.ExperiencePeriodDays, now)
		for _, insight := range report.Insights {
			if strings.TrimSpace(insight.DecisionStatus) != "" && insight.DecisionStatus != "pending" {
				continue
			}
			insightEntries := entriesByIDsSnapshot(entries, insight.Evidence)
			if len(insightEntries) == 0 {
				continue
			}
			switch insight.Kind {
			case "knowledge_gap", "repeated_issue", "ai_insight":
				if insight.Confidence >= 0.74 && len(insight.Evidence) >= 3 {
					add(autonomousKnowledgeArtifact(insight, insightEntries, now))
				}
			case "automation_opportunity":
				if insight.Confidence >= 0.8 && len(insight.Evidence) >= 3 {
					if artifact, ok := autonomousSkillArtifact(insight, insightEntries, now); ok {
						add(artifact)
					}
				}
			}
		}
	}

	result.Generated = len(result.Artifacts)
	if result.Generated == 0 {
		result.Message = "已完成自主沉淀检查，暂时没有足够清晰的新产物"
	} else {
		result.Message = fmt.Sprintf("心流自动生成 %d 个候选产物，未删除即默认采纳", result.Generated)
	}
	return result
}

func autonomousKnowledgeArtifact(insight ExperienceInsight, entries []Entry, now time.Time) AutonomousArtifact {
	title := firstNonEmpty(insight.Title, "自动知识沉淀")
	body := renderAutonomousKnowledgeBody(insight, entries)
	key := "knowledge:" + insight.ID
	if insight.ID == "" {
		key = "knowledge:" + shortHash(title+strings.Join(entryIDs(entries, 0), ":"))
	}
	return AutonomousArtifact{
		ID:              "auto-knowledge-" + shortHash(key) + "-" + now.Format("20060102"),
		Kind:            "knowledge",
		Title:           title,
		Summary:         firstNonEmpty(insight.Summary, insight.Reason),
		Body:            body,
		Evidence:        entryIDs(entries, 0),
		SourceInsightID: insight.ID,
		DedupKey:        key,
		Confidence:      insight.Confidence,
	}
}

func autonomousSkillArtifact(insight ExperienceInsight, entries []Entry, now time.Time) (AutonomousArtifact, bool) {
	profile := workflowDraftProfile(entries, insight.Title)
	if profile.riskLevel != "low" {
		return AutonomousArtifact{}, false
	}
	for _, step := range profile.steps {
		if step.RequiresConfirm {
			return AutonomousArtifact{}, false
		}
	}
	title := strings.TrimSpace(profile.title)
	if title == "" {
		title = firstNonEmpty(insight.Title, "自动 Skill")
	}
	key := "skill:" + shortHash(title+strings.Join(entryIDs(entries, 0), ":"))
	return AutonomousArtifact{
		ID:              "auto-skill-" + shortHash(key) + "-" + now.Format("20060102"),
		Kind:            "skill",
		Title:           title,
		Summary:         "证据足够且流程清晰，心流已自动整理为可由 agent 直接执行的 Skill 草稿。",
		Body:            renderAutonomousSkillBody(insight, entries, profile),
		Evidence:        entryIDs(entries, 0),
		SourceInsightID: insight.ID,
		DedupKey:        key,
		Confidence:      insight.Confidence,
		AgentExecutable: true,
	}, true
}

func renderAutonomousKnowledgeBody(insight ExperienceInsight, entries []Entry) string {
	var builder strings.Builder
	builder.WriteString("## 摘要\n")
	builder.WriteString(firstNonEmpty(insight.Summary, insight.Reason, "心流自动整理出的知识线索。"))
	builder.WriteString("\n\n## 判断依据\n")
	builder.WriteString(firstNonEmpty(insight.Reason, "多条工作记忆证据指向同一主题。"))
	builder.WriteString("\n\n## 可复用要点\n")
	for _, entry := range entries {
		builder.WriteString("- ")
		builder.WriteString(firstNonEmpty(entry.Title, entry.Summary, entry.ID))
		if summary := firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText)); summary != "" {
			builder.WriteString("：")
			builder.WriteString(summary)
		}
		builder.WriteString("\n")
	}
	builder.WriteString("\n## 证据\n")
	for _, id := range entryIDs(entries, 0) {
		builder.WriteString("- ")
		builder.WriteString(id)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func renderAutonomousSkillBody(insight ExperienceInsight, entries []Entry, profile workflowDraftProfileResult) string {
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(firstNonEmpty(profile.title, insight.Title, "Ariadne 自动 Skill"))
	builder.WriteString("\n\n## When To Use\n")
	builder.WriteString(firstNonEmpty(profile.trigger, insight.Summary, "当同类上下文再次出现时使用。"))
	builder.WriteString("\n\n## Inputs\n")
	builder.WriteString(firstNonEmpty(profile.input, "当前上下文、剪贴板或用户输入。"))
	builder.WriteString("\n\n## Steps\n")
	for i, step := range profile.steps {
		builder.WriteString(fmt.Sprintf("%d. %s\n   - Command: `%s`\n", i+1, step.Label, step.Command))
	}
	builder.WriteString("\n## Output\n")
	builder.WriteString(firstNonEmpty(profile.output, "生成可复用处理结果。"))
	builder.WriteString("\n\n## Evidence\n")
	for _, entry := range entries {
		builder.WriteString("- ")
		builder.WriteString(entry.ID)
		builder.WriteString(" · ")
		builder.WriteString(firstNonEmpty(entry.Title, entry.Summary))
		builder.WriteString("\n")
	}
	builder.WriteString("\n## Autonomous Boundary\n")
	builder.WriteString("- 该 Skill 由心流自动生成，当前判断为低风险且不需要人工输入确认。\n")
	builder.WriteString("- 如你删除该产物并填写原因，心流会避免再次生成同类 Skill。\n")
	return strings.TrimSpace(builder.String())
}

func entriesByIDsSnapshot(entries []Entry, ids []string) []Entry {
	if len(ids) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			allowed[id] = true
		}
	}
	result := []Entry{}
	for _, entry := range entries {
		if allowed[entry.ID] && entryUsableForExtraction(entry) {
			result = append(result, entry)
		}
	}
	return result
}

func normalizeAutonomousArtifact(artifact AutonomousArtifact, now time.Time) AutonomousArtifact {
	artifact.ID = strings.TrimSpace(artifact.ID)
	artifact.Kind = normalizeAutonomousKind(artifact.Kind)
	artifact.Title = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(artifact.Title)), " "), 80)
	artifact.Summary = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(artifact.Summary)), " "), 220)
	artifact.Body = strings.TrimSpace(artifact.Body)
	artifact.Evidence = cleanStrings(artifact.Evidence)
	artifact.SourceInsightID = strings.TrimSpace(artifact.SourceInsightID)
	artifact.DedupKey = strings.TrimSpace(strings.ToLower(artifact.DedupKey))
	if artifact.DedupKey == "" && artifact.Kind != "" && artifact.Title != "" {
		artifact.DedupKey = artifact.Kind + ":" + shortHash(artifact.Title+strings.Join(artifact.Evidence, ":"))
	}
	artifact.Status = strings.TrimSpace(strings.ToLower(artifact.Status))
	if artifact.Status == "" {
		artifact.Status = "active"
	}
	if artifact.CreatedAt == 0 {
		artifact.CreatedAt = now.Unix()
	}
	if artifact.UpdatedAt == 0 {
		artifact.UpdatedAt = artifact.CreatedAt
	}
	if artifact.ID == "" && artifact.Kind != "" {
		artifact.ID = "auto-" + artifact.Kind + "-" + shortHash(artifact.DedupKey)
	}
	return artifact
}

func normalizeAutonomousKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "daily", "retrospective", "knowledge", "skill":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return ""
	}
}

func activeAutonomousArtifacts(items []AutonomousArtifact) []AutonomousArtifact {
	result := []AutonomousArtifact{}
	for _, item := range items {
		if strings.EqualFold(item.Status, "rejected") || strings.EqualFold(item.Status, "deleted") {
			continue
		}
		if item.ID == "" {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})
	return result
}

func activeAutonomousKeys(items []AutonomousArtifact) map[string]bool {
	keys := map[string]bool{}
	for _, item := range activeAutonomousArtifacts(items) {
		if key := strings.TrimSpace(strings.ToLower(item.DedupKey)); key != "" {
			keys[key] = true
		}
	}
	return keys
}

func cloneAutonomousArtifacts(source []AutonomousArtifact) []AutonomousArtifact {
	if len(source) == 0 {
		return nil
	}
	cloned := make([]AutonomousArtifact, len(source))
	copy(cloned, source)
	for index := range cloned {
		cloned[index].Evidence = append([]string(nil), cloned[index].Evidence...)
	}
	return cloned
}

func cloneAutonomousRejections(source map[string]AutonomousRejection) map[string]AutonomousRejection {
	if len(source) == 0 {
		return map[string]AutonomousRejection{}
	}
	cloned := make(map[string]AutonomousRejection, len(source))
	for key, value := range source {
		cloned[strings.ToLower(strings.TrimSpace(key))] = value
	}
	return cloned
}

func sameLocalDay(timestamp int64, now time.Time) bool {
	if timestamp == 0 {
		return false
	}
	previous := time.Unix(timestamp, 0).In(now.Location())
	current := now.In(now.Location())
	return previous.Year() == current.Year() && previous.YearDay() == current.YearDay()
}

func experienceDiscoveryEvidence(entries []Entry) []ExperienceDiscoveryEvidence {
	evidence := make([]ExperienceDiscoveryEvidence, 0, len(entries))
	for _, entry := range entries {
		if !entryUsableForExtraction(entry) {
			continue
		}
		rawText := strings.Join([]string{entry.Title, entry.Summary, entry.Text, entry.OCRText, strings.Join(entry.Tags, " ")}, "\n")
		if looksSensitive(rawText) {
			continue
		}
		text := firstNonEmpty(entry.Text, entry.OCRText, entry.Summary, entry.Title)
		evidence = append(evidence, ExperienceDiscoveryEvidence{
			ID:        entry.ID,
			Source:    strings.TrimSpace(entry.Source),
			Title:     truncateExperienceText(firstNonEmpty(entry.Title, entry.Summary, entry.ID), 120),
			Summary:   truncateExperienceText(firstNonEmpty(entry.Summary, entry.Text, entry.OCRText), 240),
			Text:      truncateExperienceText(text, 900),
			AppName:   truncateExperienceText(entry.AppName, 80),
			Tags:      append([]string(nil), entry.Tags...),
			CreatedAt: entry.CreatedAt,
		})
	}
	return evidence
}

func experienceDiscoveryRiskReasons(policy ExperienceDiscoveryPolicy, evidenceCount int) []string {
	reasons := []string{
		fmt.Sprintf("将发送 %d 条非敏感工作记忆摘要和 evidence ID 到外部 AI provider。", evidenceCount),
		"不会发送截图文件路径、图片文件或已标记敏感的记忆。",
		"外部 AI 经验发现只由手动确认触发，定期经验报告仍使用本地规则。",
		"返回线索仍需人工审核后才能转任务包、工作流、清单或 Skill。",
	}
	target := strings.TrimSpace(strings.Join([]string{policy.Provider, policy.Model}, " / "))
	if target != "/" {
		reasons = append(reasons, "目标模型: "+target)
	}
	return reasons
}

func normalizeExternalExperienceReport(report ExperienceReport, entries []Entry, decisions map[string]ExperienceDecision, periodDays int, now time.Time) ExperienceReport {
	allowedEvidence := map[string]bool{}
	for _, entry := range entries {
		if entryUsableForExtraction(entry) {
			allowedEvidence[entry.ID] = true
		}
	}
	insights := make([]ExperienceInsight, 0, len(report.Insights))
	for _, insight := range report.Insights {
		evidence := filterExperienceEvidence(insight.Evidence, allowedEvidence)
		if len(evidence) == 0 {
			continue
		}
		kind := normalizeExperienceKind(insight.Kind)
		title := truncateExperienceText(firstNonEmpty(insight.Title, insight.Summary, "AI 经验线索"), 80)
		summary := truncateExperienceText(firstNonEmpty(insight.Summary, insight.Reason, title), 260)
		reason := truncateExperienceText(firstNonEmpty(insight.Reason, "外部 AI 根据 evidence 聚合出的模式。"), 260)
		recommendation := truncateExperienceText(firstNonEmpty(insight.Recommendation, "请先人工审核，再决定是否沉淀为工作流、清单或 Skill。"), 260)
		id := strings.TrimSpace(insight.ID)
		if id == "" || strings.ContainsAny(id, " \t\r\n") {
			id = "ai-" + kind + "-" + shortHash(kind+"\n"+title+"\n"+strings.Join(evidence, "\n"))
		}
		insights = append(insights, ExperienceInsight{
			ID:             id,
			Kind:           kind,
			Title:          title,
			Summary:        summary,
			Reason:         reason,
			Recommendation: recommendation,
			Evidence:       evidence,
			Confidence:     clampExperienceConfidence(insight.Confidence),
			Severity:       normalizeExperienceSeverity(insight.Severity),
			RequiresReview: true,
			CreatedAt:      firstNonZeroInt64(insight.CreatedAt, now.Unix()),
		})
	}
	sort.SliceStable(insights, func(i, j int) bool {
		if insights[i].Confidence != insights[j].Confidence {
			return insights[i].Confidence > insights[j].Confidence
		}
		return insights[i].Title < insights[j].Title
	})
	if len(insights) > 8 {
		insights = insights[:8]
	}
	insights = applyExperienceDecisions(insights, decisions)
	usedEvidence := map[string]bool{}
	for _, insight := range insights {
		for _, id := range insight.Evidence {
			usedEvidence[id] = true
		}
	}
	summary := strings.TrimSpace(report.Summary)
	if summary == "" {
		if len(insights) == 0 {
			summary = "外部 AI 未返回可落地的经验线索；可继续使用本地规则报告。"
		} else {
			summary = fmt.Sprintf("AI 发现 %d 条经验线索，所有结果都需要人工审核。", len(insights))
		}
	}
	return ExperienceReport{
		ID:            "ai-experience-" + now.Format("20060102150405"),
		Title:         firstNonEmpty(report.Title, "AI 经验发现报告"),
		Summary:       truncateExperienceText(summary, 320),
		PeriodDays:    periodDays,
		EntryCount:    len(entries),
		EvidenceCount: len(usedEvidence),
		Insights:      insights,
		GeneratedAt:   now.Unix(),
	}
}

func filterExperienceEvidence(ids []string, allowed map[string]bool) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || !allowed[id] || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func normalizeExperienceKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "repeated_issue", "issue", "problem":
		return "repeated_issue"
	case "automation_opportunity", "automation", "workflow":
		return "automation_opportunity"
	case "knowledge_gap", "knowledge", "skill":
		return "knowledge_gap"
	default:
		return "ai_insight"
	}
}

func normalizeExperienceSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(severity))
	default:
		return "medium"
	}
}

func clampExperienceConfidence(confidence float64) float64 {
	if confidence <= 0 {
		return 0.55
	}
	if confidence < 0.1 {
		return 0.1
	}
	if confidence > 0.95 {
		return 0.95
	}
	return confidence
}

func truncateExperienceText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func applyExperienceDecisions(insights []ExperienceInsight, decisions map[string]ExperienceDecision) []ExperienceInsight {
	if len(insights) == 0 || len(decisions) == 0 {
		return insights
	}
	for index := range insights {
		decision, ok := decisions[insights[index].ID]
		if !ok {
			continue
		}
		insights[index].DecisionStatus = decision.Status
		insights[index].DecisionNote = decision.Note
		insights[index].DecisionUpdatedAt = decision.UpdatedAt
		insights[index].TaskPackageID = decision.TaskPackageID
	}
	return insights
}

func cloneExperienceDecisions(source map[string]ExperienceDecision) map[string]ExperienceDecision {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]ExperienceDecision, len(source))
	for key, decision := range source {
		decision = normalizeExperienceDecision(key, decision)
		if decision.InsightID != "" && decision.Status != "" {
			cloned[decision.InsightID] = decision
		}
	}
	return cloned
}

func normalizeExperienceDecision(key string, decision ExperienceDecision) ExperienceDecision {
	if decision.InsightID == "" {
		decision.InsightID = strings.TrimSpace(key)
	} else {
		decision.InsightID = strings.TrimSpace(decision.InsightID)
	}
	decision.Status = normalizeExperienceDecisionStatus(decision.Status)
	decision.Note = strings.TrimSpace(decision.Note)
	decision.TaskPackageID = strings.TrimSpace(decision.TaskPackageID)
	return decision
}

func normalizeExperienceDecisionStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "accepted", "accept", "done":
		return "accepted"
	case "rejected", "reject", "dismissed":
		return "rejected"
	case "later", "snoozed", "todo":
		return "later"
	case "task_package", "task-package", "task", "package":
		return "task_package"
	case "workflow_draft", "workflow-draft", "workflow":
		return "workflow_draft"
	case "checklist_draft", "checklist-draft", "checklist":
		return "checklist_draft"
	case "pending", "clear", "reset":
		return "pending"
	default:
		return ""
	}
}

func discoverRepeatedIssueInsights(entries []Entry, now time.Time) []ExperienceInsight {
	groups := collectExperienceGroups(entries, []experienceGroupRule{
		{key: "network", label: "网络与代理", terms: []string{"network", "proxy", "gateway", "cloudflare", "openwrt", "dns", "timeout", "网关", "代理", "网络", "隧道"}},
		{key: "database", label: "数据库连接", terms: []string{"database", "postgres", "postgresql", "mysql", "sql", "connection refused", "数据库", "连接", "连不上"}},
		{key: "deploy", label: "发布与构建", terms: []string{"build", "deploy", "pipeline", "compile", "wails", "vite", "构建", "发布", "打包"}},
		{key: "auth", label: "认证与权限", terms: []string{"auth", "login", "permission", "forbidden", "unauthorized", "token", "登录", "权限", "鉴权"}},
	})
	insights := []ExperienceInsight{}
	for _, group := range groups {
		if len(group.entries) < 2 {
			continue
		}
		ids := entryIDs(group.entries, 5)
		insights = append(insights, ExperienceInsight{
			ID:             "insight-repeated-issue-" + group.key,
			Kind:           "repeated_issue",
			Title:          "重复问题：" + group.label,
			Summary:        fmt.Sprintf("最近记录中有 %d 条与%s相关的错误、排障或异常线索。", len(group.entries), group.label),
			Reason:         "同一主题在标题、正文、OCR 或标签中多次出现，符合重复问题发现规则。",
			Recommendation: "建议生成一份问题复盘，沉淀排查入口、判断标准和常见修复步骤；如步骤稳定，再转为检查清单或工作流草稿。",
			Evidence:       ids,
			Confidence:     experienceConfidence(len(group.entries), 0.58),
			Severity:       severityForCount(len(group.entries)),
			RequiresReview: true,
			CreatedAt:      now.Unix(),
		})
	}
	return insights
}

func discoverAutomationInsights(entries []Entry, now time.Time) []ExperienceInsight {
	groups := collectExperienceGroups(entries, []experienceGroupRule{
		{key: "workflow", label: "工作流宏", terms: []string{"workflow", "macro", "hash", "base64", "json", "url", "clipboard", "工作流", "宏", "剪贴板", "格式化"}},
		{key: "hosts", label: "Hosts 与网络切换", terms: []string{"hosts", "dns", "gateway", "ip", "openwrt", "host", "网关", "域名"}},
		{key: "screenshot-ocr", label: "截图与 OCR 整理", terms: []string{"screenshot", "capture", "ocr", "image", "截图", "贴图", "识别", "文字"}},
	})
	insights := []ExperienceInsight{}
	for _, group := range groups {
		if len(group.entries) < 2 {
			continue
		}
		ids := entryIDs(group.entries, 5)
		insights = append(insights, ExperienceInsight{
			ID:             "insight-automation-" + group.key,
			Kind:           "automation_opportunity",
			Title:          "自动化机会：" + group.label,
			Summary:        fmt.Sprintf("检测到 %d 条可能属于同一类重复操作或材料整理的记录。", len(group.entries)),
			Reason:         "同类动作、工具关键词或处理对象多次出现，适合先形成候选工作流草稿。",
			Recommendation: "建议把证据整理为候选工作流：明确触发意图、输入、输出、步骤和风险确认点；保存为正式工作流前必须由用户确认。",
			Evidence:       ids,
			Confidence:     experienceConfidence(len(group.entries), 0.52),
			Severity:       "medium",
			RequiresReview: true,
			CreatedAt:      now.Unix(),
		})
	}
	return insights
}

func discoverKnowledgeInsights(entries []Entry, now time.Time) []ExperienceInsight {
	candidates := []Entry{}
	for _, entry := range entries {
		text := entryExperienceText(entry)
		if entry.Favorite || strings.Contains(text, "todo") || strings.Contains(text, "待办") || strings.Contains(text, "复盘") || strings.Contains(text, "总结") || strings.Contains(text, "原因") || strings.Contains(text, "解决") {
			candidates = append(candidates, entry)
			continue
		}
		if len([]rune(strings.TrimSpace(entry.Text+" "+entry.OCRText))) >= 160 {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) < 2 {
		return nil
	}
	return []ExperienceInsight{{
		ID:             "insight-knowledge-assets",
		Kind:           "knowledge_gap",
		Title:          "知识沉淀机会",
		Summary:        fmt.Sprintf("发现 %d 条较高信息密度或已收藏的记录，可能适合整理成知识条目、检查清单或复盘模板。", len(candidates)),
		Reason:         "收藏、待办、总结性词汇或长文本记录通常代表用户已经投入整理成本，但尚未形成稳定知识资产。",
		Recommendation: "建议先生成知识草稿，保留证据引用；同步到 OpsCore 或其他知识库前必须做敏感检查并由用户确认。",
		Evidence:       entryIDs(candidates, 6),
		Confidence:     experienceConfidence(len(candidates), 0.5),
		Severity:       "medium",
		RequiresReview: true,
		CreatedAt:      now.Unix(),
	}}
}

type experienceGroupRule struct {
	key   string
	label string
	terms []string
}

type experienceGroup struct {
	key     string
	label   string
	entries []Entry
}

func collectExperienceGroups(entries []Entry, rules []experienceGroupRule) []experienceGroup {
	groups := make([]experienceGroup, 0, len(rules))
	for _, rule := range rules {
		group := experienceGroup{key: rule.key, label: rule.label}
		for _, entry := range entries {
			text := entryExperienceText(entry)
			for _, term := range rule.terms {
				if strings.Contains(text, strings.ToLower(term)) {
					group.entries = append(group.entries, entry)
					break
				}
			}
		}
		groups = append(groups, group)
	}
	return groups
}

func entryExperienceText(entry Entry) string {
	parts := []string{
		entry.Source,
		entry.ContentType,
		entry.Title,
		entry.Summary,
		entry.Text,
		entry.OCRText,
		entry.WindowTitle,
		entry.AppName,
	}
	parts = append(parts, entry.Tags...)
	return strings.ToLower(strings.Join(parts, " "))
}

func entryIDs(entries []Entry, limit int) []string {
	ids := []string{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.ID == "" || seen[entry.ID] {
			continue
		}
		ids = append(ids, entry.ID)
		seen[entry.ID] = true
		if limit > 0 && len(ids) >= limit {
			break
		}
	}
	return ids
}

func dailyDraftEntries(entries []Entry, now time.Time, limit int) ([]Entry, int, int) {
	nonSensitive := []Entry{}
	skippedSensitive := 0
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	for _, entry := range entries {
		if entry.Sensitive {
			skippedSensitive++
			continue
		}
		if !entryUsableForExtraction(entry) {
			continue
		}
		nonSensitive = append(nonSensitive, entry)
	}
	sort.SliceStable(nonSensitive, func(i, j int) bool {
		return nonSensitive[i].CreatedAt > nonSensitive[j].CreatedAt
	})
	today := []Entry{}
	for _, entry := range nonSensitive {
		if entry.CreatedAt >= start && entry.CreatedAt <= now.Unix() {
			today = append(today, entry)
		}
	}
	selected := today
	if len(selected) == 0 {
		selected = nonSensitive
	}
	if limit > 0 && len(selected) > limit {
		selected = selected[:limit]
	}
	return selected, len(today), skippedSensitive
}

func renderDailyDraftBody(entries []Entry, now time.Time, todayCount int, skippedSensitive int) string {
	var builder strings.Builder
	builder.WriteString("## 今日概览\n")
	builder.WriteString(fmt.Sprintf("- 生成时间：%s\n", now.Format("2006-01-02 15:04")))
	if len(entries) == 0 {
		builder.WriteString("- 当前没有可用于日报的非敏感工作记忆。\n")
		builder.WriteString("- 建议先通过手动笔记、截图补记或时间机器积累证据，再生成日报。\n\n")
		builder.WriteString("## 隐私与边界\n")
		builder.WriteString(fmt.Sprintf("- 本轮跳过敏感记忆 %d 条；外发、同步知识库或交给 AI 前仍需用户确认。\n", skippedSensitive))
		return builder.String()
	}
	scope := "今日"
	if todayCount == 0 {
		scope = "最近"
	}
	builder.WriteString(fmt.Sprintf("- 范围：%s非敏感记录 %d 条，纳入草稿 %d 条。\n", scope, maxInt(todayCount, len(entries)), len(entries)))
	if skippedSensitive > 0 {
		builder.WriteString(fmt.Sprintf("- 隐私：已跳过敏感记忆 %d 条，不写入正文和证据列表。\n", skippedSensitive))
	}
	builder.WriteString("- 来源：")
	builder.WriteString(strings.Join(dailySourceSummary(entries), "，"))
	builder.WriteString("\n\n")

	builder.WriteString("## 主要工作\n")
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("- %s · %s · %s：%s\n", formatDailyTime(entry.CreatedAt), sourceLabel(entry.Source), firstNonEmpty(entry.Title, entry.Summary), firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText))))
	}

	followUps := dailyFollowUps(entries)
	builder.WriteString("\n## 待跟进\n")
	if len(followUps) == 0 {
		builder.WriteString("- 暂未从本地记录中发现明确待办、失败或阻塞线索。\n")
	} else {
		for _, item := range followUps {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n## 复盘线索\n")
	insights := discoverExperienceInsights(entries, now)
	if len(insights) == 0 {
		builder.WriteString("- 暂未形成稳定重复模式；保留证据，后续记录增多后再生成经验发现。\n")
	} else {
		limit := len(insights)
		if limit > 3 {
			limit = 3
		}
		for _, insight := range insights[:limit] {
			builder.WriteString(fmt.Sprintf("- %s：%s 建议：%s\n", insight.Title, insight.Summary, insight.Recommendation))
		}
	}

	builder.WriteString("\n## 隐私与边界\n")
	builder.WriteString("- 本草稿完全基于本地工作记忆生成，未调用外部 AI、embedding 或同步接口。\n")
	builder.WriteString("- 外发日报、同步知识库或交给代理执行前，需要用户再次确认敏感内容和证据范围。\n")

	builder.WriteString("\n## 证据 ID\n")
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("- %s · %s\n", entry.ID, firstNonEmpty(entry.Title, entry.Summary)))
	}
	return builder.String()
}

func retrospectiveDraftEntries(entries []Entry, ids []string, limit int) ([]Entry, int, int) {
	requested := cleanStrings(ids)
	allowed := map[string]bool{}
	for _, id := range requested {
		allowed[id] = true
	}
	selected := []Entry{}
	skippedSensitive := 0
	for _, entry := range entries {
		if entry.ID == "" {
			continue
		}
		if len(allowed) > 0 && !allowed[entry.ID] {
			continue
		}
		if entry.Sensitive {
			skippedSensitive++
			continue
		}
		if entryQualityPending(entry) {
			continue
		}
		if len(allowed) == 0 {
			continue
		}
		selected = append(selected, entry)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].CreatedAt < selected[j].CreatedAt
	})
	if limit > 0 && len(selected) > limit {
		selected = selected[len(selected)-limit:]
	}
	return selected, len(requested), skippedSensitive
}

func scheduledRetrospectiveEntries(entries []Entry, limit int) []Entry {
	selected := []Entry{}
	for _, entry := range entries {
		if !entryUsableForExtraction(entry) || !isRetrospectiveContextEntry(entry) {
			continue
		}
		selected = append(selected, entry)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].CreatedAt < selected[j].CreatedAt
	})
	if limit > 0 && len(selected) > limit {
		selected = selected[len(selected)-limit:]
	}
	return selected
}

func renderRetrospectiveDraftBody(entries []Entry, now time.Time, requestedCount int, skippedSensitive int) string {
	var builder strings.Builder
	builder.WriteString("## 复盘概览\n")
	builder.WriteString(fmt.Sprintf("- 生成时间：%s\n", now.Format("2006-01-02 15:04")))
	if requestedCount > 0 {
		builder.WriteString(fmt.Sprintf("- 选中范围：%d 条工作记忆，纳入非敏感证据 %d 条。\n", requestedCount, len(entries)))
	} else {
		builder.WriteString("- 选中范围：未提供工作记忆 ID；请先选择一组记忆再生成问题复盘。\n")
	}
	if skippedSensitive > 0 {
		builder.WriteString(fmt.Sprintf("- 隐私：已跳过敏感记忆 %d 条，不进入正文和证据列表。\n", skippedSensitive))
	}
	if len(entries) == 0 {
		builder.WriteString("- 当前没有可用于复盘的非敏感工作记忆。\n\n")
		builder.WriteString("## 隐私与边界\n")
		builder.WriteString("- 本草稿不会调用外部 AI、embedding 或同步接口；补充证据后再生成可追溯复盘。\n")
		return builder.String()
	}
	builder.WriteString("\n## 问题背景\n")
	for _, entry := range entries {
		if !isRetrospectiveContextEntry(entry) {
			continue
		}
		builder.WriteString(fmt.Sprintf("- %s：%s\n", firstNonEmpty(entry.Title, entry.ID), firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText))))
	}
	if !strings.Contains(builder.String(), "## 时间线\n") && !hasRetrospectiveContext(entries) {
		builder.WriteString(fmt.Sprintf("- %s：%s\n", firstNonEmpty(entries[0].Title, entries[0].ID), firstNonEmpty(entries[0].Summary, summaryText(entries[0].Text), summaryText(entries[0].OCRText))))
	}

	builder.WriteString("\n## 时间线\n")
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("- %s · %s · %s：%s\n", formatDailyTime(entry.CreatedAt), sourceLabel(entry.Source), firstNonEmpty(entry.Title, entry.ID), firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText))))
	}

	builder.WriteString("\n## 初步原因\n")
	causes := retrospectiveCauses(entries)
	if len(causes) == 0 {
		builder.WriteString("- 尚未从本地证据中形成稳定归因；需要继续补充现象、日志、操作步骤和最终结论。\n")
	} else {
		for _, item := range causes {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n## 处理过程\n")
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("- %s：%s\n", firstNonEmpty(entry.Title, entry.ID), firstNonEmpty(summaryText(entry.Text), summaryText(entry.OCRText), entry.Summary)))
	}

	builder.WriteString("\n## 遗留风险与后续动作\n")
	followUps := dailyFollowUps(entries)
	if len(followUps) == 0 {
		builder.WriteString("- 暂未发现明确待办；建议补充最终修复动作、验证命令和是否需要沉淀为检查清单。\n")
	} else {
		for _, item := range followUps {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n## 隐私与边界\n")
	builder.WriteString("- 本复盘草稿完全基于本地工作记忆生成，未调用外部 AI、embedding 或同步接口。\n")
	builder.WriteString("- 保存为知识、工作流或交给外部代理前，需要用户确认敏感内容和证据范围。\n")

	builder.WriteString("\n## 证据 ID\n")
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("- %s · %s\n", entry.ID, firstNonEmpty(entry.Title, entry.Summary)))
	}
	return builder.String()
}

func hasRetrospectiveContext(entries []Entry) bool {
	for _, entry := range entries {
		if isRetrospectiveContextEntry(entry) {
			return true
		}
	}
	return false
}

func isRetrospectiveContextEntry(entry Entry) bool {
	text := strings.ToLower(strings.Join([]string{entry.Title, entry.Summary, entry.Text, entry.OCRText}, " "))
	return containsAny(text, "error", "failed", "issue", "problem", "todo", "verify", "verified", "fix", "rollback", "异常", "失败", "报错", "问题", "阻塞", "原因", "解决", "修复", "验证", "回滚", "待办", "复盘")
}

func retrospectiveCauses(entries []Entry) []string {
	items := []string{}
	seen := map[string]bool{}
	for _, entry := range entries {
		text := strings.ToLower(strings.Join([]string{entry.Title, entry.Summary, entry.Text, entry.OCRText}, " "))
		if !containsAny(text, "because", "cause", "root", "原因", "由于", "指向", "失败", "failed", "error", "异常", "网关", "代理", "权限", "token", "timeout") {
			continue
		}
		item := fmt.Sprintf("%s：%s", firstNonEmpty(entry.Title, entry.ID), firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText)))
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
		if len(items) >= 5 {
			break
		}
	}
	return items
}

func shortDraftTitle(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func dailySourceSummary(entries []Entry) []string {
	counts := map[string]int{}
	order := []string{}
	for _, entry := range entries {
		label := sourceLabel(entry.Source)
		if label == "" {
			label = "未知"
		}
		if _, ok := counts[label]; !ok {
			order = append(order, label)
		}
		counts[label]++
	}
	result := []string{}
	for _, label := range order {
		result = append(result, fmt.Sprintf("%s %d", label, counts[label]))
	}
	return result
}

func detectFlowAskIntent(question string) string {
	lower := strings.ToLower(strings.TrimSpace(question))
	switch {
	case containsAny(lower, "优化", "工作流", "自动化", "重复", "流程", "效率", "沉淀", "复用", "skill", "workflow"):
		return "optimization"
	case containsAny(lower, "谁", "人", "找过", "联系", "消息", "沟通", "聊天", "聊了", "聊什么", "聊", "说了什么", "说什么", "说了", "跟", "微信", "weixin", "wechat", "钉钉", "dingtalk", "会议", "meeting", "teams", "outlook", "邮件", "mail"):
		return "contacts"
	case containsAny(lower, "今天", "今日", "干了", "做了", "总结", "发生", "忙了", "上下文", "心流"):
		return "today"
	default:
		return "search"
	}
}

func flowAnswerTitle(question string, intent string) string {
	question = strings.TrimSpace(question)
	if question != "" {
		return question
	}
	switch intent {
	case "contacts":
		return "今天有哪些人找过我？"
	case "optimization":
		return "今天有哪些工作流可以优化？"
	case "today":
		return "我今天干了些什么？"
	default:
		return "心流回答"
	}
}

func flowSuggestedQuestions(intent string) []string {
	switch intent {
	case "contacts":
		return []string{"今天有哪些人找过我？", "哪些消息还需要我处理？", "最近谁经常出现在我的上下文里？"}
	case "optimization":
		return []string{"今天我的哪些工作流可以优化？", "最近我重复做了哪些事？", "哪些内容适合沉淀成 Skill？"}
	case "search":
		return []string{"刚才那个报错我后来怎么处理的？", "相关证据有哪些？", "能整理成复盘吗？"}
	default:
		return []string{"我今天干了些什么？", "今天有哪些人找过我？", "今天我的哪些工作流可以优化？"}
	}
}

func flowNonSensitiveEntries(entries []Entry, since int64) []Entry {
	selected := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if !entryUsableForExtraction(entry) {
			continue
		}
		if since > 0 && entry.CreatedAt < since {
			continue
		}
		selected = append(selected, entry)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].CreatedAt > selected[j].CreatedAt
	})
	return selected
}

func flowContactEntries(entries []Entry, now time.Time, since int64, limit int, question string) ([]Entry, int) {
	if since <= 0 {
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	}
	selected := []Entry{}
	for _, entry := range flowNonSensitiveEntries(entries, since) {
		if isFlowContactEntry(entry) {
			selected = append(selected, entry)
		}
	}
	total := len(selected)
	if terms := flowContactQueryTerms(question); len(terms) > 0 && len(selected) > 0 {
		type scoredContact struct {
			entry Entry
			score float64
		}
		scored := []scoredContact{}
		for _, entry := range selected {
			score := flowContactQuestionScore(entry, terms, question)
			if score <= 0 {
				continue
			}
			scored = append(scored, scoredContact{entry: entry, score: score})
		}
		if len(scored) > 0 {
			sort.SliceStable(scored, func(i, j int) bool {
				if scored[i].score != scored[j].score {
					return scored[i].score > scored[j].score
				}
				return scored[i].entry.CreatedAt > scored[j].entry.CreatedAt
			})
			selected = selected[:0]
			for _, item := range scored {
				selected = append(selected, item.entry)
			}
		}
	}
	if limit > 0 && len(selected) > limit {
		selected = selected[:limit]
	}
	return selected, total
}

func flowContactQueryTerms(question string) []string {
	question = strings.TrimSpace(strings.ToLower(question))
	if question == "" {
		return nil
	}
	terms := []string{}
	for _, marker := range []string{"跟", "和", "与", "同"} {
		if idx := strings.Index(question, marker); idx >= 0 {
			tail := question[idx+len(marker):]
			cut := len(tail)
			for _, end := range []string{"聊", "说", "联系", "沟通", "消息", "找", "谈"} {
				if endIdx := strings.Index(tail, end); endIdx >= 0 && endIdx < cut {
					cut = endIdx
				}
			}
			if term := normalizeFlowContactTerm(tail[:cut]); term != "" {
				terms = append(terms, term)
			}
		}
	}
	reduced := question
	for _, stop := range []string{
		"今天", "今日", "昨天", "最近", "跟", "和", "与", "同",
		"聊了什么", "聊什么", "聊了", "聊天", "聊", "谈了什么", "谈什么", "谈了", "谈",
		"说了什么", "说什么", "说了", "说", "什么", "哪些", "有啥", "有没有",
		"联系", "沟通", "消息", "微信", "weixin", "wechat", "找过我", "找我", "过我",
		"吗", "呢", "的", "？", "?", "，", ",", "。", ".", "：", ":",
	} {
		reduced = strings.ReplaceAll(reduced, stop, " ")
	}
	for _, field := range strings.Fields(reduced) {
		if term := normalizeFlowContactTerm(field); term != "" {
			terms = append(terms, term)
		}
	}
	return uniqueStrings(terms)
}

func normalizeFlowContactTerm(value string) string {
	value = strings.TrimSpace(strings.Trim(value, " \t\r\n,，.。:：?？!！[]【】()（）\"'“”‘’"))
	if value == "" || len([]rune(value)) < 2 {
		return ""
	}
	return strings.ToLower(value)
}

func flowContactQuestionScore(entry Entry, terms []string, question string) float64 {
	text := strings.ToLower(entrySemanticText(entry))
	score := 0.0
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(strings.ToLower(entry.Title), term) {
			score += 50
		}
		if strings.Contains(strings.ToLower(entry.WindowTitle), term) {
			score += 40
		}
		if strings.Contains(text, term) {
			score += 30
		}
	}
	if score > 0 {
		if isFlowContactEntry(entry) {
			score += 10
		}
		return score
	}
	if semantic := semanticSimilarity(question, text); semantic >= 0.18 {
		return 10 + semantic*30
	}
	return 0
}

func isFlowContactEntry(entry Entry) bool {
	text := strings.ToLower(entrySemanticText(entry))
	return containsAny(text,
		"微信", "weixin", "wechat", "企业微信", "钉钉", "dingtalk", "qq",
		"teams", "outlook", "邮件", "mail", "meeting", "会议", "腾讯会议", "飞书", "lark",
		"消息", "聊天", "群", "联系人",
	)
}

func flowSearchEntries(entries []Entry, question string, since int64, limit int) ([]Entry, map[string]float64) {
	normalized := strings.ToLower(strings.TrimSpace(question))
	type scoredEntry struct {
		entry Entry
		score float64
	}
	scored := []scoredEntry{}
	for _, entry := range flowNonSensitiveEntries(entries, since) {
		score, _ := scoreSearchMatch(entry, normalized)
		if normalized != "" && score <= 0 {
			continue
		}
		scored = append(scored, scoredEntry{entry: entry, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].entry.CreatedAt > scored[j].entry.CreatedAt
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	selected := make([]Entry, 0, len(scored))
	scores := map[string]float64{}
	for _, item := range scored {
		selected = append(selected, item.entry)
		scores[item.entry.ID] = item.score
	}
	return selected, scores
}

func flowEvidenceForInsights(insights []ExperienceInsight, entries []Entry, limit int) []FlowAskEvidence {
	byID := map[string]Entry{}
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	selected := []Entry{}
	scores := map[string]float64{}
	seen := map[string]bool{}
	for _, insight := range insights {
		for _, id := range insight.Evidence {
			if seen[id] {
				continue
			}
			entry, ok := byID[id]
			if !ok || !entryUsableForExtraction(entry) {
				continue
			}
			seen[id] = true
			selected = append(selected, entry)
			scores[id] = insight.Confidence
			if limit > 0 && len(selected) >= limit {
				return flowEvidenceFromEntries(selected, scores)
			}
		}
	}
	return flowEvidenceFromEntries(selected, scores)
}

func flowEvidenceFromEntries(entries []Entry, scores map[string]float64) []FlowAskEvidence {
	evidence := make([]FlowAskEvidence, 0, len(entries))
	for _, entry := range entries {
		if !entryUsableForExtraction(entry) {
			continue
		}
		item := FlowAskEvidence{
			ID:          entry.ID,
			Title:       firstNonEmpty(entry.Title, entry.Summary, entry.ID),
			Summary:     firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText)),
			Source:      entry.Source,
			AppName:     entry.AppName,
			WindowTitle: entry.WindowTitle,
			CreatedAt:   entry.CreatedAt,
			HasImage:    entry.CaptureID != "" || entry.ImagePath != "",
			Sensitive:   entry.Sensitive,
			Tags:        append([]string{}, entry.Tags...),
		}
		if scores != nil {
			item.Score = scores[entry.ID]
		}
		evidence = append(evidence, item)
	}
	return evidence
}

func flowEntriesFromAskEvidence(entries []Entry, evidence []FlowAskEvidence) []Entry {
	byID := map[string]Entry{}
	for _, entry := range entries {
		if entry.ID != "" {
			byID[entry.ID] = entry
		}
	}
	selected := make([]Entry, 0, len(evidence))
	for _, item := range evidence {
		entry, ok := byID[item.ID]
		if !ok || !entryUsableForExtraction(entry) {
			continue
		}
		selected = append(selected, entry)
	}
	return selected
}

func flowAgentEvidenceFromEntries(entries []Entry, limit int) []FlowAgentEvidence {
	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	evidence := make([]FlowAgentEvidence, 0, limit)
	for _, entry := range entries {
		if len(evidence) >= limit {
			break
		}
		if !entryUsableForExtraction(entry) {
			continue
		}
		rawText := strings.Join([]string{entry.Title, entry.Summary, entry.Text, entry.OCRText, strings.Join(entry.Tags, " ")}, "\n")
		if looksSensitive(rawText) {
			continue
		}
		evidence = append(evidence, FlowAgentEvidence{
			ID:          entry.ID,
			Title:       truncateExperienceText(firstNonEmpty(entry.Title, entry.Summary, entry.ID), 120),
			Summary:     truncateExperienceText(firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText), entry.Title), 260),
			Text:        truncateExperienceText(entry.Text, 900),
			OCRText:     truncateExperienceText(entry.OCRText, 900),
			Source:      strings.TrimSpace(entry.Source),
			AppName:     truncateExperienceText(entry.AppName, 80),
			WindowTitle: truncateExperienceText(entry.WindowTitle, 120),
			CreatedAt:   entry.CreatedAt,
			HasImage:    entry.CaptureID != "" || entry.ImagePath != "",
			Tags:        append([]string(nil), entry.Tags...),
		})
	}
	return evidence
}

func completeFlowAnswerWithAgent(base FlowAskResponse, selected []Entry, privacyMode bool, policy FlowAgentPolicy, runner FlowAgentRunner, now time.Time) FlowAskResponse {
	if privacyMode {
		return base
	}
	policy = normalizeFlowAgentPolicy(policy)
	if !policy.Enabled {
		return base
	}
	if runner == nil {
		base.OK = false
		base.Mode = "agent_error"
		base.Answer = "心流 agent 未注册，无法生成动态回答。你可以检查 Ariadne 运行时配置，或暂时关闭 AI 回答后查看本地时间线。"
		base.Message = appendFlowMessage(base.Message, "心流 agent 未注册")
		return base
	}
	evidence := flowAgentEvidenceFromEntries(selected, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	result, err := runner.AnswerFlow(ctx, FlowAgentJob{
		Question:     base.Question,
		Intent:       base.Intent,
		LocalAnswer:  base.Answer,
		Evidence:     evidence,
		Runner:       policy.Runner,
		Provider:     policy.Provider,
		BaseURL:      policy.BaseURL,
		Model:        policy.Model,
		NativeSkills: policy.NativeSkills,
		WorkDir:      policy.WorkDir,
		ToolCommand:  flowAgentToolCommand(),
		Now:          now,
	})
	if err != nil {
		base.OK = false
		base.Mode = "agent_error"
		base.Answer = "心流 agent 没有完成动态回答，未把本地统计当作最终回复。\n\n你可以稍后重试，或先打开证据查看本地检索结果。"
		base.Message = appendFlowMessage(base.Message, "Agent runner 调用失败："+truncateExperienceText(err.Error(), 180))
		return base
	}
	answer := strings.TrimSpace(result.Answer)
	if answer == "" {
		base.OK = false
		base.Mode = "agent_error"
		base.Answer = "心流 agent 未返回内容，未把本地统计当作最终回复。"
		base.Message = appendFlowMessage(base.Message, "Agent runner 未返回内容")
		return base
	}
	base.Answer = answer
	base.UsedAI = true
	base.Mode = firstNonEmpty(strings.TrimSpace(result.Mode), "agent:"+policy.Runner)
	base.Message = appendFlowMessage(base.Message, firstNonEmpty(strings.TrimSpace(result.Message), "心流 agent 已基于本地证据生成回答。"))
	return base
}

func flowAgentToolCommand() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(executable)
}

func appendFlowMessage(current string, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if next == "" {
		return current
	}
	return current + "；" + next
}

func renderFlowTodayAnswer(entries []Entry, now time.Time, todayCount int, skippedSensitive int) string {
	if len(entries) == 0 {
		if skippedSensitive > 0 {
			return fmt.Sprintf("今天还没有可用于回答的非敏感记忆；我已经跳过 %d 条敏感记录，不会把它们写进答案或证据。开启主动沉淀后，我会继续在后台归并截图、剪贴板和 OCR。", skippedSensitive)
		}
		return "今天还没有可用于回答的心流记忆。开启主动沉淀或时间机器后，我会在后台整理截图、剪贴板、OCR 和窗口上下文。"
	}
	scope := "今天"
	total := todayCount
	if todayCount == 0 {
		scope = "最近"
		total = len(entries)
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s已经沉淀 %d 条非敏感上下文，本次纳入 %d 条关键证据。", scope, total, len(entries)))
	if sourceText := strings.Join(dailySourceSummary(entries), "、"); sourceText != "" {
		builder.WriteString(fmt.Sprintf("来源主要是 %s。", sourceText))
	}
	if appText := flowTopApps(entries, 4); appText != "" {
		builder.WriteString(fmt.Sprintf("高频应用集中在 %s。", appText))
	}
	if skippedSensitive > 0 {
		builder.WriteString(fmt.Sprintf("另外有 %d 条敏感记忆已自动跳过。", skippedSensitive))
	}
	builder.WriteString("\n")
	builder.WriteString(flowEntryHighlights("主要脉络", entries, 4))
	builder.WriteString("\n")
	builder.WriteString("这些明细默认收在证据抽屉里；你可以继续问某个报错、某个窗口，或让我把它整理成复盘/清单。")
	return builder.String()
}

func renderFlowContactAnswer(entries []Entry, total int) string {
	if total == 0 {
		return "今天没有稳定识别到沟通类记忆。心流目前先按应用、窗口标题和 OCR 线索归并，还不会凭空推断联系人；后续看到微信、会议、邮件等上下文时会自动收束到这里。"
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("今天识别到 %d 条可能与沟通有关的非敏感记录。", total))
	if appText := flowTopApps(entries, 5); appText != "" {
		builder.WriteString(fmt.Sprintf("来源主要集中在 %s。", appText))
	}
	builder.WriteString("我不会要求你逐条确认这些记录，只把可追溯证据收在下方。\n")
	builder.WriteString(flowEntryHighlights("沟通线索", entries, 5))
	builder.WriteString("\n")
	builder.WriteString("当前还没有独立的联系人实体抽取，所以结论按应用和窗口线索呈现；等样本稳定后，可以再沉淀成联系人/项目维度的自动摘要。")
	return builder.String()
}

func renderFlowOptimizationAnswer(report ExperienceReport, evidence []FlowAskEvidence) string {
	if len(report.Insights) == 0 {
		return "最近还没有形成足够稳定的重复动作或可复用流程。我会继续在后台观察高频截图、OCR、剪贴板和窗口切换；只有真正要转成 Skill、工作流、清单或外部任务包时，才需要你确认。"
	}
	limit := len(report.Insights)
	if limit > 3 {
		limit = 3
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("我在最近 %d 天的非敏感记忆里发现 %d 条可优化线索，先自动归纳，不需要你逐条做决策。", report.PeriodDays, len(report.Insights)))
	if len(evidence) > 0 {
		builder.WriteString(fmt.Sprintf("下方保留 %d 条代表性证据。", len(evidence)))
	}
	builder.WriteString("\n")
	for _, insight := range report.Insights[:limit] {
		builder.WriteString(fmt.Sprintf("%s：%s 建议先做 %s。\n", insight.Title, insight.Summary, insight.Recommendation))
	}
	builder.WriteString("如果某条线索要落地为自动化、清单或 Skill，再进入确认流程；否则心流会继续安静沉淀。")
	return strings.TrimSpace(builder.String())
}

func renderFlowSearchAnswer(question string, entries []Entry) string {
	if len(entries) == 0 {
		return fmt.Sprintf("我没有在非敏感心流记忆里找到“%s”的稳定证据。可以换一个关键词，或先让时间机器/主动沉淀继续积累 OCR、剪贴板和窗口上下文。", question)
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("我找到了 %d 条与“%s”相关的非敏感记忆，已按相关度和时间排序。", len(entries), question))
	builder.WriteString("\n")
	builder.WriteString(flowEntryHighlights("相关证据", entries, 5))
	builder.WriteString("\n")
	builder.WriteString("你可以打开证据查看原始上下文，也可以继续追问“后来怎么处理”“有哪些待办”或“整理成复盘”。")
	return builder.String()
}

func flowEntryHighlights(prefix string, entries []Entry, limit int) string {
	if len(entries) == 0 {
		return prefix + "：暂无。"
	}
	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	items := []string{}
	for _, entry := range entries[:limit] {
		title := firstNonEmpty(entry.Title, entry.Summary, entry.ID)
		summary := firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText))
		if summary != "" && summary != title {
			items = append(items, fmt.Sprintf("%s %s：%s", formatDailyTime(entry.CreatedAt), title, summary))
		} else {
			items = append(items, fmt.Sprintf("%s %s", formatDailyTime(entry.CreatedAt), title))
		}
	}
	return prefix + "：" + strings.Join(items, "；") + "。"
}

func flowTopApps(entries []Entry, limit int) string {
	counts := map[string]int{}
	order := []string{}
	for _, entry := range entries {
		app := strings.TrimSpace(entry.AppName)
		if app == "" {
			app = sourceLabel(entry.Source)
		}
		if app == "" {
			app = "未知"
		}
		if _, ok := counts[app]; !ok {
			order = append(order, app)
		}
		counts[app]++
	}
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] > counts[order[j]]
		}
		return order[i] < order[j]
	})
	if limit <= 0 || limit > len(order) {
		limit = len(order)
	}
	parts := []string{}
	for _, app := range order[:limit] {
		parts = append(parts, fmt.Sprintf("%s %d 条", app, counts[app]))
	}
	return strings.Join(parts, "、")
}

func dailyFollowUps(entries []Entry) []string {
	items := []string{}
	seen := map[string]bool{}
	for _, entry := range entries {
		text := strings.ToLower(strings.Join([]string{entry.Title, entry.Summary, entry.Text, entry.OCRText}, " "))
		if !containsAny(text, "todo", "待办", "需要", "失败", "failed", "error", "异常", "阻塞", "blocked", "未完成", "需确认") {
			continue
		}
		item := fmt.Sprintf("%s：%s", firstNonEmpty(entry.Title, entry.ID), firstNonEmpty(entry.Summary, summaryText(entry.Text), summaryText(entry.OCRText)))
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
		if len(items) >= 5 {
			break
		}
	}
	return items
}

func formatDailyTime(timestamp int64) string {
	if timestamp <= 0 {
		return "--:--"
	}
	return time.Unix(timestamp, 0).Format("15:04")
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func experienceConfidence(count int, base float64) float64 {
	confidence := base + float64(count-2)*0.08
	if confidence > 0.92 {
		return 0.92
	}
	if confidence < 0.1 {
		return 0.1
	}
	return confidence
}

func severityForCount(count int) string {
	if count >= 5 {
		return "high"
	}
	if count >= 3 {
		return "medium"
	}
	return "low"
}

func semanticSimilarity(query string, document string) float64 {
	queryVector := semanticVector(query)
	documentVector := semanticVector(document)
	if len(queryVector) == 0 || len(documentVector) == 0 {
		return 0
	}
	dot := 0.0
	queryNorm := 0.0
	documentNorm := 0.0
	for token, weight := range queryVector {
		queryNorm += weight * weight
		if documentWeight, ok := documentVector[token]; ok {
			dot += weight * documentWeight
		}
	}
	for _, weight := range documentVector {
		documentNorm += weight * weight
	}
	if queryNorm == 0 || documentNorm == 0 {
		return 0
	}
	return dot / (sqrt(queryNorm) * sqrt(documentNorm))
}

func semanticVector(text string) map[string]float64 {
	vector := map[string]float64{}
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return vector
	}
	for _, token := range lexicalTokens(text) {
		vector[token] += 1
	}
	for _, token := range semanticAliases(text) {
		vector[token] += 1.8
	}
	return vector
}

func lexicalTokens(text string) []string {
	tokens := []string{}
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		token := builder.String()
		builder.Reset()
		if len([]rune(token)) >= 2 {
			tokens = append(tokens, token)
		}
	}
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			builder.WriteRune(r)
			continue
		}
		flush()
		if isCJK(r) {
			tokens = append(tokens, string(r))
		}
	}
	flush()
	return tokens
}

func semanticAliases(text string) []string {
	groups := []struct {
		patterns []string
		aliases  []string
	}{
		{[]string{"数据库", "db", "database", "postgres", "postgresql", "mysql", "sql"}, []string{"database", "db", "sql", "storage"}},
		{[]string{"连不上", "连接", "connect", "connection", "refused", "timeout", "timed out", "unreachable"}, []string{"connectivity", "network", "timeout", "unreachable"}},
		{[]string{"报错", "错误", "异常", "error", "exception", "failed", "failure", "panic"}, []string{"error", "failure", "exception", "incident"}},
		{[]string{"网关", "gateway", "openwrt", "router", "路由"}, []string{"gateway", "router", "network", "openwrt"}},
		{[]string{"代理", "proxy", "clash", "openclash", "vpn", "tunnel"}, []string{"proxy", "network", "tunnel"}},
		{[]string{"截图", "屏幕", "ocr", "图片", "image", "screenshot", "capture"}, []string{"image", "screenshot", "ocr", "capture"}},
		{[]string{"剪贴板", "clipboard", "复制", "copy"}, []string{"clipboard", "copy", "history"}},
		{[]string{"hosts", "dns", "域名", "domain"}, []string{"hosts", "dns", "domain"}},
		{[]string{"json", "yaml", "配置", "config"}, []string{"config", "structured-data", "json"}},
		{[]string{"工作流", "workflow", "宏", "自动化", "automation"}, []string{"workflow", "automation", "macro"}},
		{[]string{"日报", "复盘", "总结", "summary", "review"}, []string{"summary", "review", "draft"}},
	}
	aliases := []string{}
	for _, group := range groups {
		for _, pattern := range group.patterns {
			if strings.Contains(text, pattern) {
				aliases = append(aliases, group.aliases...)
				break
			}
		}
	}
	return aliases
}

func entrySemanticText(entry Entry) string {
	parts := []string{
		entry.Source,
		entry.ContentType,
		entry.Title,
		entry.Summary,
		entry.Text,
		entry.OCRText,
		entry.WindowTitle,
		entry.AppName,
		entry.CaptureID,
		entry.ImagePath,
	}
	parts = append(parts, entry.Tags...)
	return strings.Join(parts, " ")
}

func isCJK(r rune) bool {
	return (r >= 0x4e00 && r <= 0x9fff) || (r >= 0x3400 && r <= 0x4dbf)
}

func sqrt(value float64) float64 {
	if value <= 0 {
		return 0
	}
	guess := value
	for i := 0; i < 8; i++ {
		guess = 0.5 * (guess + value/guess)
	}
	return guess
}

func cloneEntries(entries []Entry) []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		entry.Tags = append([]string{}, entry.Tags...)
		result = append(result, entry)
	}
	return result
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Favorite != entries[j].Favorite {
			return entries[i].Favorite
		}
		return entries[i].CreatedAt > entries[j].CreatedAt
	})
}

func cleanStrings(items []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func removeStringFold(items []string, remove string) []string {
	remove = strings.TrimSpace(remove)
	if remove == "" {
		return cleanStrings(items)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), remove) {
			continue
		}
		result = append(result, item)
	}
	return cleanStrings(result)
}

func normalizeAppCaptureProfiles(profiles []AppCaptureProfile) []AppCaptureProfile {
	if len(profiles) == 0 {
		return []AppCaptureProfile{}
	}
	seen := map[string]bool{}
	result := make([]AppCaptureProfile, 0, len(profiles))
	for _, profile := range profiles {
		processName := strings.TrimSpace(profile.ProcessName)
		displayName := strings.TrimSpace(profile.DisplayName)
		icon := strings.TrimSpace(profile.Icon)
		key := appProfileKey(firstNonEmpty(processName, displayName, profile.ID))
		if key == "" || seen[key] {
			continue
		}
		if processName == "" {
			processName = firstNonEmpty(displayName, key)
		}
		if displayName == "" {
			displayName = processName
		}
		seen[key] = true
		result = append(result, AppCaptureProfile{
			ID:                       key,
			DisplayName:              displayName,
			ProcessName:              processName,
			Icon:                     icon,
			Enabled:                  profile.Enabled,
			WindowSwitchDelaySeconds: clampIntAllowZero(profile.WindowSwitchDelaySeconds, 3600),
			ActiveIntervalSeconds:    clampInt(profile.ActiveIntervalSeconds, 10, 86400, 120),
		})
	}
	return result
}

func cloneAppCaptureProfiles(profiles []AppCaptureProfile) []AppCaptureProfile {
	if len(profiles) == 0 {
		return nil
	}
	cloned := make([]AppCaptureProfile, len(profiles))
	copy(cloned, profiles)
	return cloned
}

func hasEnabledAppCaptureProfiles(policy CapturePolicy) bool {
	for _, profile := range policy.AppCaptureProfiles {
		if profile.Enabled && appProfileKey(firstNonEmpty(profile.ProcessName, profile.DisplayName, profile.ID)) != "" {
			return true
		}
	}
	return false
}

func windowCaptureProfileForContext(context windowContext, policy CapturePolicy, intervalSeconds int) (AppCaptureProfile, bool) {
	if profile, ok := appCaptureProfileForContext(context, policy); ok {
		return profile, true
	}
	if !policy.CaptureOnWindowChange {
		return AppCaptureProfile{}, false
	}
	if intervalSeconds <= 0 {
		intervalSeconds = int(defaultAutoCaptureInterval.Seconds())
	}
	if intervalSeconds > int(defaultAutoCaptureInterval.Seconds()) {
		intervalSeconds = int(defaultAutoCaptureInterval.Seconds())
	}
	return AppCaptureProfile{
		ID:                       "__default_window__",
		DisplayName:              firstNonEmpty(context.app, context.title, "当前窗口"),
		ProcessName:              context.app,
		Enabled:                  true,
		WindowSwitchDelaySeconds: normalizeWindowChangeCooldown(policy.WindowChangeCooldown),
		ActiveIntervalSeconds:    intervalSeconds,
	}, true
}

func appCaptureProfileForContext(context windowContext, policy CapturePolicy) (AppCaptureProfile, bool) {
	app := appProfileKey(context.app)
	appRaw := strings.ToLower(strings.TrimSpace(context.app))
	title := strings.ToLower(strings.TrimSpace(context.title))
	if app == "" && appRaw == "" && title == "" {
		return AppCaptureProfile{}, false
	}
	for _, profile := range policy.AppCaptureProfiles {
		if !profile.Enabled {
			continue
		}
		key := appProfileKey(firstNonEmpty(profile.ProcessName, profile.DisplayName, profile.ID))
		display := strings.ToLower(strings.TrimSpace(profile.DisplayName))
		process := strings.ToLower(strings.TrimSpace(profile.ProcessName))
		processBase := appProfileKey(process)
		switch {
		case key != "" && app == key:
			return profile, true
		case processBase != "" && app == processBase:
			return profile, true
		case process != "" && (appRaw == process || strings.Contains(appRaw, process)):
			return profile, true
		case display != "" && strings.Contains(title, display):
			return profile, true
		}
	}
	return AppCaptureProfile{}, false
}

func appProfileKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	return strings.ToLower(filepath.Base(value))
}

func clampInt(value int, min int, max int, fallback int) int {
	if value < min {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func clampIntAllowZero(value int, max int) int {
	if value < 0 {
		return 0
	}
	if value > max {
		return max
	}
	return value
}

func stringListContainsFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}

func entryExcluded(entry Entry, policy CapturePolicy) (bool, string) {
	if excluded, reason := pathExcluded(entry.ImagePath, policy); excluded {
		return true, reason
	}
	if excluded, reason := urlExcluded(entrySemanticText(entry), policy); excluded {
		return true, reason
	}
	if excluded, reason := contentExcluded(entrySemanticText(entry), policy); excluded {
		return true, reason
	}
	return false, ""
}

func normalizeExportFilter(request ExportRequest) ExportFilter {
	startAt := request.StartAt
	endAt := request.EndAt
	if startAt > 0 && endAt > 0 && startAt > endAt {
		startAt, endAt = endAt, startAt
	}
	return ExportFilter{
		StartAt:  startAt,
		EndAt:    endAt,
		Tags:     cleanStrings(request.Tags),
		EntryIDs: cleanStrings(request.EntryIDs),
	}
}

func entryMatchesExportFilter(entry Entry, filter ExportFilter) bool {
	if filter.StartAt > 0 && entry.CreatedAt < filter.StartAt {
		return false
	}
	if filter.EndAt > 0 && entry.CreatedAt > filter.EndAt {
		return false
	}
	if len(filter.EntryIDs) > 0 {
		matchesID := false
		for _, id := range filter.EntryIDs {
			if strings.EqualFold(entry.ID, id) {
				matchesID = true
				break
			}
		}
		if !matchesID {
			return false
		}
	}
	if len(filter.Tags) > 0 {
		matchesTag := false
		entryTags := map[string]bool{}
		for _, tag := range entry.Tags {
			entryTags[strings.ToLower(strings.TrimSpace(tag))] = true
		}
		for _, tag := range filter.Tags {
			if entryTags[strings.ToLower(strings.TrimSpace(tag))] {
				matchesTag = true
				break
			}
		}
		if !matchesTag {
			return false
		}
	}
	return true
}

func pathExcluded(path string, policy CapturePolicy) (bool, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, ""
	}
	normalizedPath := normalizeRulePath(path)
	base := strings.ToLower(filepath.Base(normalizedPath))
	for _, pattern := range policy.ExcludePaths {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		normalizedPattern := normalizeRulePath(pattern)
		patternBase := strings.ToLower(filepath.Base(normalizedPattern))
		if normalizedPath == normalizedPattern || strings.Contains(normalizedPath, normalizedPattern) || base == patternBase || (patternBase != "." && patternBase != "" && strings.Contains(base, patternBase)) {
			return true, pattern
		}
	}
	return false, ""
}

func contentExcluded(text string, policy CapturePolicy) (bool, string) {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false, ""
	}
	for _, pattern := range policy.ExcludeContentPatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(pattern)) {
			return true, pattern
		}
	}
	return false, ""
}

func urlExcluded(text string, policy CapturePolicy) (bool, string) {
	candidates := urlCandidates(text)
	if len(candidates) == 0 {
		return false, ""
	}
	for _, pattern := range policy.ExcludeURLs {
		normalizedPattern := normalizeURLRule(pattern)
		if normalizedPattern == "" {
			continue
		}
		for _, candidate := range candidates {
			if candidate == normalizedPattern || strings.Contains(candidate, normalizedPattern) {
				return true, pattern
			}
		}
	}
	return false, ""
}

func urlCandidates(text string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, field := range strings.FieldsFunc(text, isURLTokenSeparator) {
		normalized := normalizeURLRule(field)
		if normalized == "" || !looksURLLike(normalized) || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}

func isURLTokenSeparator(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '"', '\'', '<', '>', '[', ']', '(', ')', '{', '}', '，', '。', '；', '、':
		return true
	default:
		return false
	}
}

func normalizeURLRule(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'<>[](){}，。；;、`))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "www.")
	value = strings.TrimPrefix(value, "*.")
	value = strings.TrimRight(value, "/")
	for strings.HasSuffix(value, ".") || strings.HasSuffix(value, ",") || strings.HasSuffix(value, ";") || strings.HasSuffix(value, ":") {
		value = strings.TrimRight(value, ".,;:")
	}
	return value
}

func looksURLLike(value string) bool {
	if value == "" {
		return false
	}
	host := value
	if index := strings.IndexAny(host, "/?#"); index >= 0 {
		host = host[:index]
	}
	return strings.Contains(host, ".") && !strings.Contains(host, "\\")
}

func normalizeRulePath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), `"'`)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	return strings.ToLower(filepath.ToSlash(path))
}

func captureExcluded(context windowContext, policy CapturePolicy) (bool, string) {
	app := strings.ToLower(strings.TrimSpace(context.app))
	appBase := strings.ToLower(filepath.Base(app))
	windowTitle := strings.ToLower(strings.TrimSpace(context.title))
	for _, pattern := range policy.ExcludeApps {
		normalized := strings.ToLower(strings.TrimSpace(pattern))
		if normalized == "" {
			continue
		}
		normalizedBase := strings.ToLower(filepath.Base(normalized))
		if app == normalized || appBase == normalizedBase || strings.Contains(app, normalized) || strings.Contains(appBase, normalizedBase) {
			return true, "排除规则命中应用: " + pattern
		}
	}
	for _, keyword := range policy.ExcludeWindowKeywords {
		normalized := strings.ToLower(strings.TrimSpace(keyword))
		if normalized == "" {
			continue
		}
		if strings.Contains(windowTitle, normalized) {
			return true, "排除规则命中窗口: " + keyword
		}
	}
	if excluded, reason := urlExcluded(context.title, policy); excluded {
		return true, "排除规则命中 URL: " + reason
	}
	return false, ""
}

func capturePausedByActivity(policy CapturePolicy, snapshot activitySnapshot) (bool, string) {
	if policy.PauseOnLock && snapshot.SessionLocked {
		return true, "系统已锁定，时间机器暂停"
	}
	if policy.PauseOnIdle && policy.IdlePauseSeconds > 0 && snapshot.IdleSeconds >= policy.IdlePauseSeconds {
		return true, fmt.Sprintf("空闲 %d 秒超过阈值 %d 秒，时间机器暂停", snapshot.IdleSeconds, policy.IdlePauseSeconds)
	}
	return false, ""
}

func normalizeCaptureScope(value string) string {
	switch strings.TrimSpace(value) {
	case "active_window", "primary_screen":
		return strings.TrimSpace(value)
	default:
		return "all_screens"
	}
}

func normalizeMultiMonitor(value string) string {
	switch strings.TrimSpace(value) {
	case "per_monitor", "primary_only":
		return strings.TrimSpace(value)
	default:
		return "combined"
	}
}

func normalizeIdlePauseSeconds(value int) int {
	if value < 30 {
		return 600
	}
	return value
}

func normalizeWindowChangeCooldown(value int) int {
	if value < 3 {
		return 3
	}
	if value > 3600 {
		return 3600
	}
	return value
}

func windowSignature(context windowContext) string {
	app := strings.ToLower(strings.TrimSpace(context.app))
	title := strings.ToLower(strings.TrimSpace(context.title))
	if app == "" && title == "" {
		return ""
	}
	return app + "\x00" + title
}

func normalizeDraftSchedulePolicy(policy DraftSchedulePolicy) DraftSchedulePolicy {
	if policy.IntervalMinutes < 15 {
		policy.IntervalMinutes = 240
	}
	if policy.IntervalMinutes > 1440 {
		policy.IntervalMinutes = 1440
	}
	if policy.ExperiencePeriodDays <= 0 {
		policy.ExperiencePeriodDays = 7
	}
	if policy.ExperiencePeriodDays > 365 {
		policy.ExperiencePeriodDays = 365
	}
	if !policy.DailyDraftEnabled && !policy.RetrospectiveEnabled && !policy.ExperienceReportEnabled {
		policy.DailyDraftEnabled = true
	}
	return policy
}

func normalizeDraftPolishPolicy(policy DraftPolishPolicy) DraftPolishPolicy {
	policy.Provider = strings.TrimSpace(strings.ToLower(policy.Provider))
	if policy.Provider == "" {
		policy.Provider = "disabled"
	}
	policy.BaseURL = strings.TrimSpace(policy.BaseURL)
	policy.Model = strings.TrimSpace(policy.Model)
	if policy.Enabled && policy.Provider == "disabled" {
		policy.Enabled = false
	}
	return policy
}

func normalizeOCRSummaryPolicy(policy OCRSummaryPolicy) OCRSummaryPolicy {
	policy.Provider = strings.TrimSpace(strings.ToLower(policy.Provider))
	if policy.Provider == "" {
		policy.Provider = "disabled"
	}
	policy.BaseURL = strings.TrimSpace(policy.BaseURL)
	policy.Model = strings.TrimSpace(policy.Model)
	if policy.Enabled && (policy.Provider == "disabled" || policy.Model == "") {
		policy.Enabled = false
	}
	return policy
}

func normalizeFlowAgentPolicy(policy FlowAgentPolicy) FlowAgentPolicy {
	policy.Runner = strings.TrimSpace(strings.ToLower(policy.Runner))
	if policy.Runner == "" {
		policy.Runner = "openai-agent"
	}
	switch policy.Runner {
	case "openai-agent", "openai-agents", "agent", "internal", "internal-agent":
		policy.Runner = "openai-agent"
	case "codex", "codex-cli":
		policy.Runner = "codex"
	case "agent-sdk", "agents-sdk", "openai-agents-sdk":
		policy.Runner = "openai-agent"
	case "disabled", "none", "off":
		policy.Runner = "disabled"
	default:
		policy.Runner = "disabled"
	}
	policy.Provider = strings.TrimSpace(strings.ToLower(policy.Provider))
	if policy.Provider == "" && policy.Runner == "openai-agent" {
		policy.Provider = "openai-compatible"
	}
	policy.BaseURL = strings.TrimSpace(policy.BaseURL)
	policy.Model = strings.TrimSpace(policy.Model)
	policy.WorkDir = strings.TrimSpace(policy.WorkDir)
	if policy.Enabled && policy.Runner == "disabled" {
		policy.Enabled = false
	}
	if policy.Enabled && policy.Runner == "openai-agent" && (policy.Provider == "disabled" || policy.Model == "") {
		policy.Enabled = false
	}
	return policy
}

func normalizeExperienceDiscoveryPolicy(policy ExperienceDiscoveryPolicy) ExperienceDiscoveryPolicy {
	policy.Provider = strings.TrimSpace(strings.ToLower(policy.Provider))
	if policy.Provider == "" {
		policy.Provider = "disabled"
	}
	policy.BaseURL = strings.TrimSpace(policy.BaseURL)
	policy.Model = strings.TrimSpace(policy.Model)
	if policy.Enabled && (policy.Provider == "disabled" || policy.Model == "") {
		policy.Enabled = false
	}
	return policy
}

func normalizeEmbeddingPolicy(policy EmbeddingPolicy) EmbeddingPolicy {
	policy.Provider = strings.TrimSpace(strings.ToLower(policy.Provider))
	if policy.Provider == "" {
		policy.Provider = "disabled"
	}
	policy.BaseURL = strings.TrimSpace(policy.BaseURL)
	policy.Model = strings.TrimSpace(policy.Model)
	policy.VectorStoreURI = strings.TrimSpace(policy.VectorStoreURI)
	policy.VectorStoreType = strings.TrimSpace(strings.ToLower(policy.VectorStoreType))
	if policy.VectorStoreType == "" || policy.VectorStoreType == "disabled" {
		policy.VectorStoreType = "embedded"
	}
	switch policy.VectorStoreType {
	case "embedded", "milvus":
	default:
		policy.VectorStoreType = "embedded"
	}
	policy.VectorCollection = strings.TrimSpace(policy.VectorCollection)
	if policy.VectorCollection == "" {
		policy.VectorCollection = "ariadne_work_memory"
	}
	if policy.Enabled && (policy.Provider == "disabled" || policy.Model == "") {
		policy.Enabled = false
	}
	return policy
}

func normalizeDraftForPolish(draft Draft) Draft {
	draft.ID = strings.TrimSpace(draft.ID)
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Body = strings.TrimSpace(draft.Body)
	draft.Evidence = cleanStrings(draft.Evidence)
	return draft
}

func normalizeDraftKind(kind string) string {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	switch normalized {
	case "daily", "retrospective", "knowledge":
		return normalized
	default:
		return "daily"
	}
}

func polishRiskReasons(draft Draft, kind string, provider string, model string) []string {
	reasons := []string{
		"AI 润色会把当前草稿正文发送到外部 provider，必须由用户二次确认",
		"草稿证据 ID 会保留，外部模型不得新增无法追溯的事实",
	}
	if strings.TrimSpace(provider) != "" && provider != "disabled" {
		target := provider
		if model != "" {
			target += " / " + model
		}
		reasons = append(reasons, "目标: "+target)
	}
	if len(draft.Evidence) > 0 {
		reasons = append(reasons, fmt.Sprintf("草稿包含 %d 条 evidence 引用", len(draft.Evidence)))
	}
	if kind == "daily" {
		reasons = append(reasons, "日报润色只改写表达，不应扩大工作范围或补造结论")
	}
	if looksSensitive(draft.Body) {
		reasons = append(reasons, "草稿正文疑似包含敏感词，请确认后再外发")
	}
	return reasons
}

func normalizePolishedDraft(original Draft, polished Draft, kind string, now time.Time) Draft {
	polished.ID = strings.TrimSpace(polished.ID)
	if polished.ID == "" {
		polished.ID = original.ID + "-ai-polished"
	}
	polished.Title = strings.TrimSpace(polished.Title)
	if polished.Title == "" {
		switch kind {
		case "retrospective":
			polished.Title = "AI 润色复盘草稿：" + strings.TrimSpace(original.Title)
		case "knowledge":
			polished.Title = "AI 润色知识草稿：" + strings.TrimSpace(original.Title)
		default:
			polished.Title = "AI 润色日报草稿：" + strings.TrimSpace(original.Title)
		}
	}
	polished.Body = strings.TrimSpace(polished.Body)
	if polished.Body == "" {
		polished.Body = original.Body
	}
	polished.Evidence = cleanStrings(polished.Evidence)
	if len(polished.Evidence) == 0 {
		polished.Evidence = append([]string(nil), original.Evidence...)
	}
	if polished.CreatedAt == 0 {
		polished.CreatedAt = now.Unix()
	}
	return polished
}

func (s *Service) embeddingRefreshError(message string, skipped int, failed int) EmbeddingRefreshResult {
	s.mu.Lock()
	s.embeddingError = strings.TrimSpace(message)
	status := s.semanticStatusLocked()
	s.mu.Unlock()
	return EmbeddingRefreshResult{
		OK:       false,
		Message:  message,
		Status:   status,
		Skipped:  skipped,
		Failed:   failed,
		Provider: status.ExternalProvider,
		Model:    status.EmbeddingModel,
	}
}

func embeddingText(entry Entry) string {
	parts := []string{
		entry.Title,
		entry.Summary,
		entry.Text,
		entry.OCRText,
		entry.WindowTitle,
		entry.AppName,
		strings.Join(entry.Tags, " "),
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func normalizeDenseVector(vector []float64) []float64 {
	sum := 0.0
	for _, value := range vector {
		sum += value * value
	}
	if sum <= 0 {
		return nil
	}
	return append([]float64(nil), vector...)
}

func cosineSimilarity(left []float64, right []float64) float64 {
	if len(left) == 0 || len(right) == 0 || len(left) != len(right) {
		return 0
	}
	dot := 0.0
	leftNorm := 0.0
	rightNorm := 0.0
	for i := range left {
		dot += left[i] * right[i]
		leftNorm += left[i] * left[i]
		rightNorm += right[i] * right[i]
	}
	if leftNorm <= 0 || rightNorm <= 0 {
		return 0
	}
	return dot / (sqrt(leftNorm) * sqrt(rightNorm))
}

func cloneEmbeddingIndex(index map[string]embeddingRecord) map[string]embeddingRecord {
	result := make(map[string]embeddingRecord, len(index))
	for key, record := range index {
		record.Vector = append([]float64(nil), record.Vector...)
		result[key] = record
	}
	return result
}

func (s *Service) loadEmbeddingIndex() {
	if s.path == "" {
		return
	}
	payload, ok, err := loadEmbeddingStateFromSQLite(s.embeddingIndexPath())
	if err != nil {
		s.embeddingError = err.Error()
		return
	} else if !ok {
		return
	}
	index := map[string]embeddingRecord{}
	metadataCount := 0
	for _, record := range payload.Records {
		if record.EntryID == "" {
			continue
		}
		metadataCount++
		if len(record.Vector) > 0 {
			record.Vector = append([]float64(nil), record.Vector...)
			index[record.EntryID] = record
		}
		if record.IndexedAt > s.lastEmbeddingAt {
			s.lastEmbeddingAt = record.IndexedAt
		}
	}
	if payload.LastIndexedAt > s.lastEmbeddingAt {
		s.lastEmbeddingAt = payload.LastIndexedAt
	}
	s.embeddingIndex = index
	if len(index) > 0 {
		s.embeddingIndexed = len(index)
	} else {
		s.embeddingIndexed = metadataCount
	}
}

func (s *Service) saveEmbeddingIndexLocked() error {
	if s.path == "" {
		return nil
	}
	records := make([]embeddingRecord, 0, len(s.embeddingIndex))
	for _, record := range s.embeddingIndex {
		record.Vector = append([]float64(nil), record.Vector...)
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].EntryID < records[j].EntryID
	})
	payload := embeddingStateFile{
		Version:          1,
		Provider:         s.embeddingPolicy.Provider,
		Model:            s.embeddingPolicy.Model,
		VectorStoreType:  s.embeddingPolicy.VectorStoreType,
		VectorStoreURI:   s.embeddingPolicy.VectorStoreURI,
		VectorCollection: s.embeddingPolicy.VectorCollection,
		LastIndexedAt:    s.lastEmbeddingAt,
		Records:          records,
	}
	return saveEmbeddingStateToSQLite(s.embeddingIndexPath(), payload)
}

func (s *Service) saveEmbeddingMetadataLocked(records []embeddingRecord) error {
	if s.path == "" {
		return nil
	}
	metadata := make([]embeddingRecord, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.EntryID) == "" {
			continue
		}
		metadata = append(metadata, embeddingRecord{EntryID: record.EntryID, IndexedAt: record.IndexedAt})
	}
	sort.SliceStable(metadata, func(i, j int) bool {
		return metadata[i].EntryID < metadata[j].EntryID
	})
	payload := embeddingStateFile{
		Version:          1,
		Provider:         s.embeddingPolicy.Provider,
		Model:            s.embeddingPolicy.Model,
		VectorStoreType:  s.embeddingPolicy.VectorStoreType,
		VectorStoreURI:   s.embeddingPolicy.VectorStoreURI,
		VectorCollection: s.embeddingPolicy.VectorCollection,
		LastIndexedAt:    s.lastEmbeddingAt,
		Records:          metadata,
	}
	return saveEmbeddingStateToSQLite(s.embeddingIndexPath(), payload)
}

func (s *Service) embeddingIndexPath() string {
	if s.path == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.path), "work_memory_vectors.json")
}

func (s *Service) embeddingNamespace() string {
	source := strings.TrimSpace(s.path)
	if source == "" {
		source = "memory"
	}
	return "ariadne_" + shortHash(strings.ToLower(filepath.Clean(source)))
}

func captureScopeLabel(value string) string {
	switch normalizeCaptureScope(value) {
	case "active_window":
		return "前台窗口"
	case "primary_screen":
		return "主屏幕"
	default:
		return "全部屏幕"
	}
}

func multiMonitorLabel(value string) string {
	switch normalizeMultiMonitor(value) {
	case "per_monitor":
		return "按屏幕分条"
	case "primary_only":
		return "仅主屏"
	default:
		return "合并截图"
	}
}

func sourceLabel(source string) string {
	switch source {
	case "time_machine":
		return "屏幕时间机器"
	case "manual_capture":
		return "手动补记"
	case "manual_note":
		return "手动笔记"
	default:
		return source
	}
}

func boolLabel(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultMemoryPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "work_memory.json")
}

func cleanImportPaths(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.Trim(strings.TrimSpace(path), `"'`)
		if path == "" {
			continue
		}
		cleaned = append(cleaned, filepath.Clean(path))
	}
	return cleanStrings(cleaned)
}

func maxTextMaterialBytes() int64 {
	return 8 * 1024 * 1024
}

func maxDocumentMaterialBytes() int64 {
	return 32 * 1024 * 1024
}

func isTextMaterialExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".markdown", ".txt", ".log":
		return true
	default:
		return false
	}
}

func isImageMaterialExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif":
		return true
	default:
		return false
	}
}

func isDocumentMaterialExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".pdf", ".docx", ".xlsx", ".pptx", ".doc", ".xls", ".ppt":
		return true
	default:
		return false
	}
}

func textMaterialContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".md" || ext == ".markdown" {
		return "markdown"
	}
	return "text"
}

func textMaterialTag(path string) string {
	if textMaterialContentType(path) == "markdown" {
		return "Markdown"
	}
	return "文本"
}

func documentMaterialContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return "pdf"
	case ".xlsx", ".xls":
		return "spreadsheet"
	case ".pptx", ".ppt":
		return "presentation"
	default:
		return "office_document"
	}
}

func documentMaterialLabel(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return "PDF"
	case ".xlsx", ".xls":
		return "Office 表格"
	case ".pptx", ".ppt":
		return "Office 演示"
	default:
		return "Office 文档"
	}
}

func extractDocumentMaterialText(path string) (string, string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx", ".xlsx", ".pptx":
		return extractOpenXMLDocumentText(path)
	case ".pdf":
		text, err := extractPDFDocumentText(path)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(text) == "" {
			return "", "PDF 未提取到可搜索正文；后续可通过截图/OCR 或专用解析器补充。", nil
		}
		return text, "PDF 正文为本地 best-effort 提取，压缩或扫描版 PDF 可能需要 OCR。", nil
	case ".doc", ".xls", ".ppt":
		return "", "旧版 Office 二进制格式已记录文件元数据；如需全文检索，请另存为 docx/xlsx/pptx 后导入。", nil
	default:
		return "", "", fmt.Errorf("unsupported document type")
	}
}

func extractOpenXMLDocumentText(path string) (string, string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", "", err
	}
	defer archive.Close()
	ext := strings.ToLower(filepath.Ext(path))
	parts := []string{}
	for _, file := range archive.File {
		if !shouldReadOfficeXML(file.Name, ext) {
			continue
		}
		text, err := readOfficeXMLText(file)
		if err != nil {
			continue
		}
		if text != "" {
			parts = append(parts, text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return "", "Office Open XML 未提取到可搜索正文；文件可能为空或只包含图片。", nil
	}
	return trimTextRunes(text, 120000), "Office Open XML 正文已从本地 zip/xml 提取。", nil
}

func shouldReadOfficeXML(name string, ext string) bool {
	name = filepath.ToSlash(name)
	if !strings.HasSuffix(strings.ToLower(name), ".xml") {
		return false
	}
	switch ext {
	case ".docx":
		return name == "word/document.xml" ||
			strings.HasPrefix(name, "word/header") ||
			strings.HasPrefix(name, "word/footer") ||
			name == "word/footnotes.xml" ||
			name == "word/endnotes.xml"
	case ".xlsx":
		return name == "xl/sharedStrings.xml" ||
			strings.HasPrefix(name, "xl/worksheets/sheet")
	case ".pptx":
		return strings.HasPrefix(name, "ppt/slides/slide") ||
			strings.HasPrefix(name, "ppt/notesSlides/notesSlide")
	default:
		return false
	}
}

func readOfficeXMLText(file *zip.File) (string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	decoder := xml.NewDecoder(io.LimitReader(reader, 4*1024*1024))
	parts := []string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if charData, ok := token.(xml.CharData); ok {
			text := strings.Join(strings.Fields(string(charData)), " ")
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, " "), nil
}

func extractPDFDocumentText(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if int64(len(raw)) > maxDocumentMaterialBytes() {
		raw = raw[:int(maxDocumentMaterialBytes())]
	}
	parts := []string{}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '(' {
			continue
		}
		text, next, ok := parsePDFLiteralString(raw, i)
		if ok {
			text = strings.Join(strings.Fields(text), " ")
			if len([]rune(text)) >= 2 {
				parts = append(parts, text)
			}
		}
		if next > i {
			i = next
		}
	}
	return trimTextRunes(strings.Join(parts, "\n"), 120000), nil
}

func parsePDFLiteralString(raw []byte, start int) (string, int, bool) {
	if start >= len(raw) || raw[start] != '(' {
		return "", start, false
	}
	var builder strings.Builder
	depth := 1
	for i := start + 1; i < len(raw); i++ {
		char := raw[i]
		if char == '\\' {
			if i+1 >= len(raw) {
				break
			}
			i++
			escaped := raw[i]
			switch escaped {
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case 't':
				builder.WriteByte('\t')
			case 'b':
				builder.WriteByte('\b')
			case 'f':
				builder.WriteByte('\f')
			case '(', ')', '\\':
				builder.WriteByte(escaped)
			case '\n':
			case '\r':
				if i+1 < len(raw) && raw[i+1] == '\n' {
					i++
				}
			default:
				if escaped >= '0' && escaped <= '7' {
					value := int(escaped - '0')
					count := 1
					for count < 3 && i+1 < len(raw) && raw[i+1] >= '0' && raw[i+1] <= '7' {
						i++
						value = value*8 + int(raw[i]-'0')
						count++
					}
					builder.WriteByte(byte(value))
				} else {
					builder.WriteByte(escaped)
				}
			}
			continue
		}
		if char == '(' {
			depth++
			builder.WriteByte(char)
			continue
		}
		if char == ')' {
			depth--
			if depth == 0 {
				return builder.String(), i, true
			}
			builder.WriteByte(char)
			continue
		}
		builder.WriteByte(char)
	}
	return builder.String(), len(raw) - 1, builder.Len() > 0
}

func importedTextTitle(path string, text string) string {
	if title := firstMarkdownHeading(text); title != "" {
		return title
	}
	if title := noteTitle(text); title != "" && title != "手动笔记" {
		return title
	}
	base := strings.TrimSpace(filepath.Base(path))
	if base != "" && base != "." {
		return base
	}
	return "导入材料"
}

func firstMarkdownHeading(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		if line == "" {
			continue
		}
		if len([]rune(line)) > 48 {
			return string([]rune(line)[:48]) + "..."
		}
		return line
	}
	return ""
}

func importTags(request ImportMaterialRequest, tags ...string) []string {
	result := []string{"导入"}
	result = append(result, request.Tags...)
	result = append(result, tags...)
	return cleanStrings(result)
}

func trimTextRunes(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func noteTitle(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len([]rune(line)) > 32 {
			return string([]rune(line)[:32]) + "..."
		}
		return line
	}
	return "手动笔记"
}

func summaryText(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len([]rune(text)) > 96 {
		return string([]rune(text)[:96]) + "..."
	}
	return text
}

func enrichEntry(entry Entry) Entry {
	corpus := strings.TrimSpace(strings.Join([]string{
		entry.Title,
		entry.Summary,
		entry.Text,
		entry.OCRText,
		entry.WindowTitle,
		entry.AppName,
		entry.ImagePath,
	}, "\n"))
	if entry.Summary == "" {
		entry.Summary = summaryText(corpus)
	}
	if entry.ContentType == "" || entry.ContentType == "note" {
		entry.ContentType = inferNoteContentType(corpus)
	}
	entry.Tags = cleanStrings(append(entry.Tags, classifyNoteTags(corpus)...))
	if entry.Sensitive {
		entry.Tags = cleanStrings(append(entry.Tags, "敏感"))
	}
	return entry
}

func inferNoteContentType(text string) string {
	lower := strings.ToLower(text)
	switch {
	case looksJSONLike(lower):
		return "json"
	case looksSQLLike(lower):
		return "sql"
	case looksCommandLike(lower):
		return "command"
	case looksErrorLike(text):
		return "error_log"
	case strings.Contains(lower, "http://") || strings.Contains(lower, "https://"):
		return "url"
	case looksTodoLike(text):
		return "todo"
	case looksPathLike(text):
		return "file_path"
	case looksCodeLike(lower):
		return "code"
	default:
		return "note"
	}
}

func classifyNoteTags(text string) []string {
	lower := strings.ToLower(text)
	tags := []string{}
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
		tags = append(tags, "URL")
	}
	if looksErrorLike(text) {
		tags = append(tags, "错误")
	}
	if looksJSONLike(lower) {
		tags = append(tags, "JSON")
	}
	if looksSQLLike(lower) {
		tags = append(tags, "SQL")
	}
	if looksTodoLike(text) {
		tags = append(tags, "待办")
	}
	if looksCommandLike(lower) {
		tags = append(tags, "命令")
	}
	if looksPathLike(text) {
		tags = append(tags, "路径")
	}
	if strings.Contains(lower, "api") || strings.Contains(lower, "接口") || strings.Contains(lower, "endpoint") {
		tags = append(tags, "API")
	}
	if strings.Contains(lower, "ip ") || strings.Contains(lower, " ip") || strings.Contains(lower, "端口") || strings.Contains(lower, "port") || containsIPv4Like(lower) {
		tags = append(tags, "网络")
	}
	if strings.Contains(lower, "dns") || strings.Contains(lower, "hosts") || strings.Contains(lower, "gateway") || strings.Contains(lower, "openwrt") || strings.Contains(lower, "proxy") || strings.Contains(lower, "代理") || strings.Contains(lower, "网关") {
		tags = append(tags, "网络")
	}
	if strings.Contains(lower, "postgres") || strings.Contains(lower, "mysql") || strings.Contains(lower, "sqlite") || strings.Contains(lower, "redis") || strings.Contains(lower, "database") || strings.Contains(lower, "数据库") {
		tags = append(tags, "数据库")
	}
	if strings.Contains(lower, "yaml") || strings.Contains(lower, "toml") || strings.Contains(lower, "config") || strings.Contains(lower, "配置") || strings.Contains(lower, ".env") {
		tags = append(tags, "配置")
	}
	if strings.Contains(lower, "git ") || strings.Contains(lower, "go test") || strings.Contains(lower, "pnpm") || strings.Contains(lower, "npm ") || strings.Contains(lower, "wails3") {
		tags = append(tags, "开发")
	}
	if looksCodeLike(lower) {
		tags = append(tags, "代码")
	}
	return tags
}

func looksErrorLike(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "exception") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(text, "报错") ||
		strings.Contains(text, "失败") ||
		strings.Contains(text, "异常") ||
		strings.Contains(text, "超时")
}

func looksJSONLike(lower string) bool {
	return strings.Contains(lower, "{") && strings.Contains(lower, "}") &&
		(strings.Contains(lower, "\":") || strings.Contains(lower, ":{") || strings.Contains(lower, ":["))
}

func looksSQLLike(lower string) bool {
	return strings.Contains(lower, "select ") ||
		strings.Contains(lower, " from ") ||
		strings.Contains(lower, "insert into ") ||
		strings.Contains(lower, "update ") ||
		strings.Contains(lower, "delete from ") ||
		strings.Contains(lower, "where ")
}

func looksTodoLike(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "todo") ||
		strings.Contains(lower, "fixme") ||
		strings.Contains(text, "待办") ||
		strings.Contains(text, "未完成") ||
		strings.Contains(text, "后续")
}

func looksCommandLike(lower string) bool {
	return strings.Contains(lower, "powershell") ||
		strings.Contains(lower, "cmd") ||
		strings.Contains(lower, "bash") ||
		strings.Contains(lower, "kubectl ") ||
		strings.Contains(lower, "docker ") ||
		strings.Contains(lower, "go test") ||
		strings.Contains(lower, "pnpm ") ||
		strings.Contains(lower, "npm ") ||
		strings.Contains(lower, "wails3 ")
}

func looksPathLike(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, ":\\") ||
		strings.Contains(lower, "p:\\") ||
		strings.Contains(lower, "c:\\") ||
		strings.Contains(lower, "/var/") ||
		strings.Contains(lower, "/etc/") ||
		strings.Contains(lower, ".json") ||
		strings.Contains(lower, ".md") ||
		strings.Contains(lower, ".log")
}

func looksCodeLike(lower string) bool {
	return strings.Contains(lower, "func ") ||
		strings.Contains(lower, "def ") ||
		strings.Contains(lower, "class ") ||
		strings.Contains(lower, "const ") ||
		strings.Contains(lower, "let ") ||
		strings.Contains(lower, "import ") ||
		strings.Contains(lower, "package ")
}

func containsIPv4Like(lower string) bool {
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	for _, part := range parts {
		if strings.Count(part, ".") == 3 && len(part) >= 7 && len(part) <= 15 {
			return true
		}
	}
	return false
}

func LooksSensitiveText(text string) bool {
	return looksSensitive(text)
}

func autoSensitive(policy CapturePolicy, text string) bool {
	return policy.SensitiveRulesEnabled && looksSensitive(text)
}

func looksSensitive(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, pattern := range sensitiveCredentialPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func shortHash(text string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(text))
	return fmt.Sprintf("%08x", hash.Sum32())
}

func exportPackagePath(memoryPath string, now time.Time) string {
	base := filepath.Dir(memoryPath)
	if strings.TrimSpace(base) == "" || base == "." {
		base = "."
	}
	return filepath.Join(base, "exports", "ariadne-work-memory-"+now.Format("20060102-150405")+".zip")
}

func writeExportPackage(path string, entries []Entry, includeSensitive bool, skippedSensitive int, skippedExcluded int, filteredOut int, filter ExportFilter, exportedAt time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	archive := zip.NewWriter(file)
	defer archive.Close()

	state := struct {
		Version               int          `json:"version"`
		ExportedBy            string       `json:"exportedBy"`
		ExportedAt            int64        `json:"exportedAt"`
		IncludesSensitive     bool         `json:"includesSensitive"`
		SkippedSensitiveCount int          `json:"skippedSensitiveCount"`
		SkippedExcludedCount  int          `json:"skippedExcludedCount"`
		FilteredOutCount      int          `json:"filteredOutCount"`
		Filter                ExportFilter `json:"filter,omitempty"`
		Entries               []Entry      `json:"entries"`
	}{
		Version:               1,
		ExportedBy:            "Ariadne",
		ExportedAt:            exportedAt.Unix(),
		IncludesSensitive:     includeSensitive,
		SkippedSensitiveCount: skippedSensitive,
		SkippedExcludedCount:  skippedExcluded,
		FilteredOutCount:      filteredOut,
		Filter:                filter,
		Entries:               entries,
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := writeZipBytes(archive, "work_memory.json", raw); err != nil {
		return err
	}
	if err := writeZipBytes(archive, "README.md", []byte(exportReadme(state.ExportedAt, len(entries), includeSensitive, skippedSensitive, skippedExcluded, filteredOut, filter))); err != nil {
		return err
	}
	if err := writeZipBytes(archive, "timeline.md", []byte(renderTimelineMarkdown(entries))); err != nil {
		return err
	}

	seenEvidence := map[string]bool{}
	for _, entry := range entries {
		if entry.ImagePath == "" {
			continue
		}
		if seenEvidence[entry.ImagePath] {
			continue
		}
		seenEvidence[entry.ImagePath] = true
		if err := writeZipFile(archive, evidenceArchiveName(entry), entry.ImagePath); err != nil {
			continue
		}
	}
	return nil
}

func writeZipBytes(archive *zip.Writer, name string, data []byte) error {
	writer, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func writeZipFile(archive *zip.Writer, name string, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer, err := archive.Create(filepath.ToSlash(name))
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}

func exportReadme(exportedAt int64, entryCount int, includeSensitive bool, skippedSensitive int, skippedExcluded int, filteredOut int, filter ExportFilter) string {
	return fmt.Sprintf(`# Ariadne Work Memory Export

- Exported at: %s
- Entries: %d
- Includes sensitive entries: %t
- Skipped sensitive entries: %d
- Skipped excluded entries: %d
- Filtered out entries: %d
- Filter: %s

This package contains a human-readable timeline, the structured JSON export, and available image evidence files.
`, time.Unix(exportedAt, 0).Format(time.RFC3339), entryCount, includeSensitive, skippedSensitive, skippedExcluded, filteredOut, exportFilterSummary(filter))
}

func exportFilterSummary(filter ExportFilter) string {
	parts := []string{}
	if filter.StartAt > 0 {
		parts = append(parts, "start="+time.Unix(filter.StartAt, 0).Format(time.RFC3339))
	}
	if filter.EndAt > 0 {
		parts = append(parts, "end="+time.Unix(filter.EndAt, 0).Format(time.RFC3339))
	}
	if len(filter.Tags) > 0 {
		parts = append(parts, "tags="+strings.Join(filter.Tags, ","))
	}
	if len(filter.EntryIDs) > 0 {
		parts = append(parts, "entryIds="+strings.Join(filter.EntryIDs, ","))
	}
	if len(parts) == 0 {
		return "all entries"
	}
	return strings.Join(parts, "; ")
}

func renderTimelineMarkdown(entries []Entry) string {
	var builder strings.Builder
	builder.WriteString("# Work Memory Timeline\n\n")
	for _, entry := range entries {
		builder.WriteString("## ")
		builder.WriteString(entry.Title)
		builder.WriteString("\n\n")
		builder.WriteString("- ID: ")
		builder.WriteString(entry.ID)
		builder.WriteString("\n- Time: ")
		builder.WriteString(time.Unix(entry.CreatedAt, 0).Format(time.RFC3339))
		builder.WriteString("\n- Source: ")
		builder.WriteString(entry.Source)
		builder.WriteString("\n- App: ")
		builder.WriteString(entry.AppName)
		builder.WriteString("\n- Tags: ")
		builder.WriteString(strings.Join(entry.Tags, ", "))
		builder.WriteString("\n\n")
		builder.WriteString(entry.Summary)
		builder.WriteString("\n\n")
		if entry.Text != "" {
			builder.WriteString("```text\n")
			builder.WriteString(entry.Text)
			builder.WriteString("\n```\n\n")
		}
		if entry.OCRText != "" {
			builder.WriteString("OCR:\n\n```text\n")
			builder.WriteString(entry.OCRText)
			builder.WriteString("\n```\n\n")
		}
		if entry.ImagePath != "" {
			builder.WriteString("Evidence image: ")
			builder.WriteString(entry.ImagePath)
			builder.WriteString("\n\n")
		}
	}
	return builder.String()
}

func evidenceArchiveName(entry Entry) string {
	base := filepath.Base(entry.ImagePath)
	if base == "." || base == string(os.PathSeparator) {
		base = entry.ID + ".png"
	}
	return filepath.Join("evidence", sanitizeArchivePart(entry.ID), sanitizeArchivePart(base))
}

func sanitizeArchivePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "item"
	}
	var builder strings.Builder
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' || char == '.' {
			builder.WriteRune(char)
		} else {
			builder.WriteRune('_')
		}
	}
	cleaned := strings.Trim(builder.String(), "._")
	if cleaned == "" {
		return "item"
	}
	return cleaned
}
