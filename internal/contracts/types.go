package contracts

type SearchResultType string

const (
	ResultFile          SearchResultType = "file"
	ResultApp           SearchResultType = "app"
	ResultPluginTrigger SearchResultType = "plugin_trigger"
	ResultPluginResult  SearchResultType = "plugin_result"
	ResultWorkflow      SearchResultType = "workflow"
	ResultClipboard     SearchResultType = "clipboard"
	ResultMemory        SearchResultType = "memory"
	ResultCommand       SearchResultType = "command"
	ResultCapture       SearchResultType = "capture"
	ResultSettings      SearchResultType = "settings"
)

type PreviewActionKind string

const (
	ActionOpen       PreviewActionKind = "open"
	ActionOpenParent PreviewActionKind = "open_parent"
	ActionCopy       PreviewActionKind = "copy"
	ActionPin        PreviewActionKind = "pin"
	ActionRun        PreviewActionKind = "run"
	ActionPlugin     PreviewActionKind = "plugin"
	ActionRemember   PreviewActionKind = "remember"
	ActionDanger     PreviewActionKind = "danger"
)

type PreviewKind string

const (
	PreviewText     PreviewKind = "text"
	PreviewMemory   PreviewKind = "memory"
	PreviewImage    PreviewKind = "image"
	PreviewSettings PreviewKind = "settings"
	PreviewWorkflow PreviewKind = "workflow"
)

type LabelValue struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ActionFeedback struct {
	SuccessLabel string `json:"successLabel,omitempty"`
	DurationMS   int    `json:"durationMs,omitempty"`
}

type PreviewAction struct {
	ID       string                 `json:"id"`
	Label    string                 `json:"label"`
	Icon     string                 `json:"icon,omitempty"`
	Shortcut string                 `json:"shortcut,omitempty"`
	Kind     PreviewActionKind      `json:"kind"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
	Feedback *ActionFeedback        `json:"feedback,omitempty"`
}

type PreviewDescriptor struct {
	Kind      PreviewKind  `json:"kind"`
	Title     string       `json:"title"`
	Subtitle  string       `json:"subtitle,omitempty"`
	Text      string       `json:"text,omitempty"`
	Meta      []LabelValue `json:"meta,omitempty"`
	Evidence  []LabelValue `json:"evidence,omitempty"`
	ImageHint string       `json:"imageHint,omitempty"`
}

type SearchResult struct {
	ID       string                 `json:"id"`
	Type     SearchResultType       `json:"type"`
	Title    string                 `json:"title"`
	Subtitle string                 `json:"subtitle,omitempty"`
	Detail   string                 `json:"detail,omitempty"`
	Icon     string                 `json:"icon"`
	Score    float64                `json:"score,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
	Preview  PreviewDescriptor      `json:"preview"`
	Actions  []PreviewAction        `json:"actions"`
}

type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Elapsed int64          `json:"elapsedMs"`
}

type ActionResult struct {
	OK                   bool     `json:"ok"`
	Message              string   `json:"message"`
	RequiresConfirmation bool     `json:"requiresConfirmation,omitempty"`
	RiskReasons          []string `json:"riskReasons,omitempty"`
}
