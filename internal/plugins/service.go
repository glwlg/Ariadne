package plugins

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ariadne/internal/contracts"
)

type CommandSchema struct {
	Usage    string         `json:"usage"`
	Examples []string       `json:"examples"`
	Params   []CommandParam `json:"params"`
}

type CommandParam struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
}

type PluginManifest struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	Description          string        `json:"description"`
	Keywords             []string      `json:"keywords"`
	SupportedPlatforms   []string      `json:"supportedPlatforms"`
	RequiredCapabilities []string      `json:"requiredCapabilities"`
	CommandSchema        CommandSchema `json:"commandSchema"`
}

type handler func(query string) []contracts.SearchResult

type registeredPlugin struct {
	manifest PluginManifest
	handle   handler
}

type Service struct {
	mu      sync.RWMutex
	plugins []registeredPlugin
	enabled map[string]bool
}

func NewService() *Service {
	return &Service{plugins: []registeredPlugin{
		manifest("calculator", "计算器", "执行数学运算", []string{"c", "calc", "calculate"}, nil, CommandSchema{
			Usage:    "calc <expression>",
			Examples: []string{"calc 12*(8+3)", "c 3.14*8^2"},
			Params:   []CommandParam{{Name: "expression", Label: "表达式", Placeholder: "例如: 12*(8+3)", Required: true}},
		}, executeCalculator),
		manifest("timestamp", "时间戳转换", "Unix 时间戳与日期格式互转", []string{"t", "ts", "time", "timestamp"}, nil, CommandSchema{
			Usage:    "timestamp <now|unix|date>",
			Examples: []string{"t now", "timestamp 1710000000", "ts 2026-03-01 12:30:00"},
			Params:   []CommandParam{{Name: "value", Label: "时间输入", Placeholder: "now / 1710000000 / 2026-03-01 12:30:00", Required: true}},
		}, executeTimestamp),
		manifest("base64", "Base64 编码/解码", "对文本进行 Base64 编码或解码", []string{"b", "base64", "b64"}, nil, CommandSchema{
			Usage:    "base64 <text>",
			Examples: []string{"base64 hello world", "b64 5L2g5aW9"},
			Params:   []CommandParam{{Name: "text", Label: "文本", Placeholder: "输入待编码或解码内容", Required: true}},
		}, executeBase64),
		manifest("hash", "Hash 生成器", "生成 MD5、SHA1、SHA256 哈希值", []string{"h", "hash"}, nil, CommandSchema{
			Usage:    "hash <text>",
			Examples: []string{"hash hello", "h 123456"},
			Params:   []CommandParam{{Name: "text", Label: "文本", Placeholder: "输入待计算哈希的内容", Required: true}},
		}, executeHash),
		manifest("json", "JSON 格式化", "格式化或压缩 JSON 字符串", []string{"j", "json"}, nil, CommandSchema{
			Usage:    "json <json-text>",
			Examples: []string{`json {"name":"ariadne","ok":true}`, `j {"items":[1,2,3]}`},
			Params:   []CommandParam{{Name: "json_text", Label: "JSON", Placeholder: "输入 JSON 字符串", Required: true}},
		}, executeJSON),
		manifest("json_compare", "JSON 对比", "打开 JSON 对比工具窗口", []string{"jd", "jsondiff", "jsoncompare", "json对比"}, nil, CommandSchema{
			Usage:    "jsondiff",
			Examples: []string{"jsondiff", "jd", "json对比"},
		}, executeJSONCompare),
		manifest("url", "URL 编码/解码", "对 URL 进行编码或解码", []string{"u", "url"}, nil, CommandSchema{
			Usage:    "url <text-or-url>",
			Examples: []string{"url https://a.com?q=中文", "u hello world"},
			Params:   []CommandParam{{Name: "value", Label: "文本或 URL", Placeholder: "输入待编码/解码内容", Required: true}},
		}, executeURL),
		manifest("uuid", "UUID 生成器", "生成随机 UUID v4", []string{"uuid", "guid"}, nil, CommandSchema{
			Usage:    "uuid [count]",
			Examples: []string{"uuid", "uuid 10", "guid 3"},
			Params:   []CommandParam{{Name: "count", Label: "数量", Placeholder: "默认 5，最大 50", Required: false}},
		}, executeUUID),
		manifest("custom_launch", "自定义启动项", "搜索并启动用户配置的程序、文件夹、文件或 URL", []string{"launch", "start", "启动项"}, []string{"custom_launchers"}, CommandSchema{
			Usage:    "launch [query]",
			Examples: []string{"launch", "launch code", "start docs"},
			Params:   []CommandParam{{Name: "query", Label: "关键词", Placeholder: "留空打开启动项设置；实际启动项会直接出现在主搜索结果中", Required: false}},
		}, executeCustomLaunch),
		manifest("qr", "二维码生成", "生成二维码并贴到屏幕", []string{"qr", "qrcode"}, []string{"pinned_image"}, CommandSchema{
			Usage:    "qr <text>",
			Examples: []string{"qr https://x-tools.app", "qrcode hello"},
			Params:   []CommandParam{{Name: "text", Label: "二维码内容", Placeholder: "输入文本或链接", Required: true}},
		}, executeQR),
		manifest("qr_scan", "二维码识别", "识别截图历史或当前屏幕中的二维码", []string{"qrscan", "scanqr", "识别二维码", "二维码识别"}, []string{"qr_recognition"}, CommandSchema{
			Usage:    "qrscan",
			Examples: []string{"qrscan", "二维码识别"},
		}, executeQRScan),
		manifest("system_commands", "系统命令", "锁屏、休眠、清空回收站等系统快捷控制", []string{"sys", "system"}, []string{"system_commands"}, CommandSchema{
			Usage:    "sys <command>",
			Examples: []string{"sys lock", "system sleep", "sys empty"},
			Params:   []CommandParam{{Name: "command", Label: "命令", Placeholder: "lock / sleep / empty / shutdown / restart", Required: true}},
		}, executeSystem),
		manifest("hosts", "Hosts 管理", "快速切换和编辑系统 Hosts", []string{"hosts", "host"}, []string{"hosts"}, CommandSchema{
			Usage:    "hosts",
			Examples: []string{"hosts", "host"},
		}, executeHosts),
		manifest("clipboard", "剪贴板历史", "查看、搜索、置顶并复用历史剪贴板内容", []string{"clip", "clipboard"}, []string{"clipboard"}, CommandSchema{
			Usage:    "clip [query|clear]",
			Examples: []string{"clip", "clip token", "clipboard clear"},
			Params:   []CommandParam{{Name: "query", Label: "关键词", Placeholder: "留空打开中心；输入 clear 清空未置顶", Required: false}},
		}, executeClipboard),
		manifest("capture_overlay", "区域截图", "拖拽选择屏幕区域并保存、贴图或识别二维码", []string{"shot", "screenshot", "screen", "region", "截图", "区域截图"}, []string{"screenshot_overlay"}, CommandSchema{
			Usage:    "shot",
			Examples: []string{"shot", "screenshot", "区域截图"},
		}, executeCaptureOverlay),
		manifest("capture_history", "捕获历史", "查看、搜索、复制并复用截图捕获历史", []string{"cap", "capture", "截图历史", "捕获历史"}, []string{"clipboard", "open_path", "pinned_image"}, CommandSchema{
			Usage:    "cap [query|clear]",
			Examples: []string{"cap", "cap 1920x1080", "capture clear"},
			Params:   []CommandParam{{Name: "query", Label: "关键词", Placeholder: "留空打开中心；输入 clear 清空未置顶", Required: false}},
		}, executeCaptureHistory),
		manifest("network_monitor", "网络监控", "查看本机实时上下行速率和网卡流量", []string{"net", "network", "traffic", "网速", "网络监控"}, []string{"network_monitor"}, CommandSchema{
			Usage:    "net",
			Examples: []string{"net", "network", "网速"},
		}, executeNetworkMonitor),
		manifest("workflow", "工作流宏", "执行和管理命令链工作流", []string{"wf", "workflow", "flow", "macro"}, []string{"clipboard"}, CommandSchema{
			Usage:    "wf <workflow-id>",
			Examples: []string{"wf clip-md5", "workflow now-timestamp", "flow"},
			Params:   []CommandParam{{Name: "workflow_id", Label: "工作流 ID", Placeholder: "输入工作流 ID 或留空打开管理", Required: false}},
		}, executeWorkflow),
		manifest("work_memory", "工作记忆", "检索、补记、日报、复盘和知识草稿", []string{"mem", "memory", "wm", "记忆"}, []string{"clipboard", "open_path", "screenshot"}, CommandSchema{
			Usage:    "mem <query|daily|capture|pause|resume>",
			Examples: []string{"mem gateway", "mem daily", "mem capture"},
			Params:   []CommandParam{{Name: "query", Label: "记忆查询或命令", Placeholder: "gateway / daily / capture", Required: false}},
		}, executeWorkMemory),
	}}
}

func manifest(id string, name string, description string, keywords []string, capabilities []string, schema CommandSchema, handle handler) registeredPlugin {
	return registeredPlugin{
		manifest: PluginManifest{
			ID:                   id,
			Name:                 name,
			Description:          description,
			Keywords:             keywords,
			SupportedPlatforms:   []string{"windows"},
			RequiredCapabilities: capabilities,
			CommandSchema:        schema,
		},
		handle: handle,
	}
}

func (s *Service) List() []PluginManifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	plugins := make([]PluginManifest, 0, len(s.plugins))
	for _, plugin := range s.plugins {
		plugins = append(plugins, plugin.manifest)
	}
	return plugins
}

func (s *Service) ApplyEnabled(enabled map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = make(map[string]bool, len(enabled))
	for id, value := range enabled {
		s.enabled[strings.ToLower(strings.TrimSpace(id))] = value
	}
}

func (s *Service) Search(query string) []contracts.SearchResult {
	trimmed := strings.TrimSpace(query)
	lower := strings.ToLower(trimmed)
	if lower == "" {
		return s.triggerResults("")
	}

	keyword, rest := splitCommand(trimmed)
	if plugin, ok, enabled := s.findByKeyword(keyword); ok {
		if !enabled {
			return []contracts.SearchResult{disabledPluginResult(plugin.manifest)}
		}
		return plugin.handle(rest)
	}

	results := s.triggerResults(lower)
	return results
}

func (s *Service) Execute(keyword string, query string) []contracts.SearchResult {
	if plugin, ok, enabled := s.findByKeyword(keyword); ok {
		if !enabled {
			return []contracts.SearchResult{disabledPluginResult(plugin.manifest)}
		}
		return plugin.handle(query)
	}
	return []contracts.SearchResult{messageResult("plugin-unknown", "未找到插件", keyword, "请确认插件关键词是否正确。", "error")}
}

func (s *Service) findByKeyword(keyword string) (registeredPlugin, bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lower := strings.ToLower(strings.TrimSpace(keyword))
	for _, plugin := range s.plugins {
		for _, item := range plugin.manifest.Keywords {
			if strings.ToLower(item) == lower {
				return plugin, true, s.pluginEnabledLocked(plugin.manifest.ID)
			}
		}
	}
	return registeredPlugin{}, false, false
}

func (s *Service) triggerResults(filter string) []contracts.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]contracts.SearchResult, 0, len(s.plugins))
	for _, plugin := range s.plugins {
		if !s.pluginEnabledLocked(plugin.manifest.ID) {
			continue
		}
		if filter != "" && !pluginMatches(plugin.manifest, filter) {
			continue
		}
		keyword := primaryCommandKeyword(plugin.manifest)
		results = append(results, contracts.SearchResult{
			ID:       "plugin-trigger-" + plugin.manifest.ID,
			Type:     contracts.ResultPluginTrigger,
			Title:    plugin.manifest.Name,
			Subtitle: "插件 · " + strings.Join(plugin.manifest.Keywords, " / "),
			Detail:   plugin.manifest.Description,
			Icon:     "plugin",
			Tags:     append([]string{"插件"}, plugin.manifest.Keywords...),
			Payload: map[string]interface{}{
				"pluginId":      plugin.manifest.ID,
				"keyword":       keyword,
				"commandSchema": plugin.manifest.CommandSchema,
			},
			Preview: contracts.PreviewDescriptor{
				Kind:     contracts.PreviewText,
				Title:    plugin.manifest.Name,
				Subtitle: plugin.manifest.CommandSchema.Usage,
				Text:     plugin.manifest.Description,
				Meta: []contracts.LabelValue{
					{Label: "关键词", Value: strings.Join(plugin.manifest.Keywords, ", ")},
					{Label: "能力", Value: strings.Join(plugin.manifest.RequiredCapabilities, ", ")},
				},
			},
			Actions: []contracts.PreviewAction{
				contracts.RunAction("prepare_command", "补全命令", keyword, "Enter"),
				contracts.CopyAction("copy_command", "复制用法", plugin.manifest.CommandSchema.Usage, ""),
			},
		})
	}
	return results
}

func (s *Service) pluginEnabledLocked(id string) bool {
	if s.enabled == nil {
		return true
	}
	enabled, ok := s.enabled[strings.ToLower(strings.TrimSpace(id))]
	return !ok || enabled
}

func primaryCommandKeyword(plugin PluginManifest) string {
	usage := strings.TrimSpace(plugin.CommandSchema.Usage)
	if usage != "" {
		if fields := strings.Fields(usage); len(fields) > 0 {
			keyword := strings.Trim(fields[0], "[]<>")
			if keyword != "" {
				return keyword
			}
		}
	}
	if len(plugin.Keywords) > 0 {
		return plugin.Keywords[0]
	}
	return plugin.ID
}

func pluginMatches(plugin PluginManifest, filter string) bool {
	parts := []string{plugin.ID, plugin.Name, plugin.Description, plugin.CommandSchema.Usage}
	parts = append(parts, plugin.Keywords...)
	parts = append(parts, plugin.CommandSchema.Examples...)
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), filter)
}

func splitCommand(query string) (string, string) {
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return "", ""
	}
	keyword := parts[0]
	rest := strings.TrimSpace(strings.TrimPrefix(query, keyword))
	return keyword, rest
}

func executeCalculator(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []contracts.SearchResult{messageResult("calc-help", "输入表达式以计算", "calc <expression>", "支持 +、-、*、/、%、^ 和括号。", "info")}
	}
	result, err := evalExpression(query)
	if err != nil {
		return []contracts.SearchResult{messageResult("calc-error", "计算错误", query, err.Error(), "error")}
	}
	text := trimFloat(result)
	return []contracts.SearchResult{copyResult("calc-result", "= "+text, "计算器", text, "表达式: "+query, []string{"计算器"})}
}

func executeTimestamp(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	now := time.Now()
	if query == "" || strings.EqualFold(query, "now") {
		unix := strconv.FormatInt(now.Unix(), 10)
		date := now.Format("2006-01-02 15:04:05")
		return []contracts.SearchResult{
			copyResult("timestamp-now", "当前时间戳: "+unix, "时间戳", unix, date, []string{"时间戳"}),
			copyResult("timestamp-date", "当前日期: "+date, "时间戳", date, unix, []string{"时间戳"}),
		}
	}
	if ts, err := strconv.ParseInt(query, 10, 64); err == nil {
		date := time.Unix(ts, 0).Format("2006-01-02 15:04:05")
		return []contracts.SearchResult{copyResult("timestamp-to-date", "日期: "+date, "时间戳转换", date, query, []string{"时间戳"})}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", "2006/01/02 15:04:05", "2006/01/02"} {
		if parsed, err := time.ParseInLocation(layout, query, time.Local); err == nil {
			unix := strconv.FormatInt(parsed.Unix(), 10)
			return []contracts.SearchResult{copyResult("timestamp-from-date", "时间戳: "+unix, "时间戳转换", unix, query, []string{"时间戳"})}
		}
	}
	return []contracts.SearchResult{messageResult("timestamp-error", "无效的日期格式", query, "支持 now、Unix 秒和常见日期格式。", "error")}
}

func executeBase64(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []contracts.SearchResult{messageResult("base64-help", "输入文本以编码或解码", "base64 <text>", "会尝试解码，并始终提供编码结果。", "info")}
	}
	results := []contracts.SearchResult{}
	if decoded, err := base64.StdEncoding.DecodeString(query); err == nil && utf8.Valid(decoded) {
		results = append(results, copyResult("base64-decode", "解码结果: "+clip(string(decoded), 96), "Base64", string(decoded), query, []string{"Base64"}))
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(query))
	results = append(results, copyResult("base64-encode", "编码结果: "+clip(encoded, 96), "Base64", encoded, query, []string{"Base64"}))
	return results
}

func executeHash(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []contracts.SearchResult{messageResult("hash-help", "输入文本以生成 Hash", "hash <text>", "返回 MD5、SHA1 和 SHA256。", "info")}
	}
	data := []byte(query)
	md5Sum := md5.Sum(data)
	sha1Sum := sha1.Sum(data)
	sha256Sum := sha256.Sum256(data)
	return []contracts.SearchResult{
		copyResult("hash-md5", "MD5: "+hex.EncodeToString(md5Sum[:]), "Hash", hex.EncodeToString(md5Sum[:]), query, []string{"Hash"}),
		copyResult("hash-sha1", "SHA1: "+hex.EncodeToString(sha1Sum[:]), "Hash", hex.EncodeToString(sha1Sum[:]), query, []string{"Hash"}),
		copyResult("hash-sha256", "SHA256: "+hex.EncodeToString(sha256Sum[:]), "Hash", hex.EncodeToString(sha256Sum[:]), query, []string{"Hash"}),
	}
}

func executeJSON(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []contracts.SearchResult{messageResult("json-help", "输入 JSON 以格式化", "json <json-text>", "返回格式化和压缩结果。", "info")}
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(query), &parsed); err != nil {
		return []contracts.SearchResult{messageResult("json-error", "JSON 解析错误", query, err.Error(), "error")}
	}
	formatted, _ := marshalJSON(parsed, "    ")
	minified, _ := marshalJSON(parsed, "")
	return []contracts.SearchResult{
		copyResult("json-format", "格式化结果: "+clip(string(formatted), 96), "JSON", string(formatted), "格式化 JSON", []string{"JSON"}),
		copyResult("json-minify", "压缩结果: "+clip(string(minified), 96), "JSON", string(minified), "压缩 JSON", []string{"JSON"}),
	}
}

func marshalJSON(value interface{}, indent string) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if indent != "" {
		encoder.SetIndent("", indent)
	}
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}

func executeJSONCompare(query string) []contracts.SearchResult {
	return []contracts.SearchResult{toolWindowResult("json-compare", "打开 JSON 对比工具", "JSON 对比", "对比两个 JSON 的语义差异和规范化行差异。", "open_json_compare")}
}

func executeURL(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []contracts.SearchResult{messageResult("url-help", "输入 URL 或文本", "url <text-or-url>", "返回编码和可能的解码结果。", "info")}
	}
	results := []contracts.SearchResult{}
	if decoded, err := url.PathUnescape(query); err == nil && decoded != query {
		results = append(results, copyResult("url-decode", "解码结果: "+clip(decoded, 96), "URL", decoded, query, []string{"URL"}))
	}
	encoded := legacyURLQuote(query)
	results = append(results, copyResult("url-encode", "编码结果: "+clip(encoded, 96), "URL", encoded, query, []string{"URL"}))
	return results
}

func legacyURLQuote(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "%2F", "/")
	return encoded
}

func executeUUID(query string) []contracts.SearchResult {
	count := 5
	if parsed, err := strconv.Atoi(strings.TrimSpace(query)); err == nil {
		count = parsed
	}
	if count < 1 {
		count = 1
	}
	if count > 50 {
		count = 50
	}
	results := make([]contracts.SearchResult, 0, count)
	for i := 0; i < count; i++ {
		id := newUUID()
		results = append(results, copyResult("uuid-"+strconv.Itoa(i+1), id, "UUID v4", id, "随机 UUID", []string{"UUID"}))
	}
	return results
}

func executeCustomLaunch(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	result := toolWindowResult("custom-launch-manager", "管理自定义启动项", "自定义启动项", "在设置中心添加、禁用或调整自定义启动项；已配置的启动项会直接进入主搜索结果。", "open_settings")
	result.Icon = "settings"
	result.Tags = append(result.Tags, "启动项", "自定义启动项")
	result.Preview.Meta = append(result.Preview.Meta,
		contracts.LabelValue{Label: "搜索", Value: "直接在主搜索输入启动项名称或关键词"},
		contracts.LabelValue{Label: "管理", Value: "在设置中心维护应用、文件、文件夹、URL 和命令类启动项"},
	)
	if query != "" {
		result.Title = "管理自定义启动项: " + query
		result.Detail = "如果主搜索没有命中该关键词，请在设置中心添加或调整自定义启动项。"
		result.Preview.Text = result.Detail
		result.Preview.Meta = append(result.Preview.Meta, contracts.LabelValue{Label: "查询", Value: query})
	}
	return []contracts.SearchResult{result}
}

func executeQR(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []contracts.SearchResult{messageResult("qr-help", "输入内容以生成二维码", "qr <text>", "二维码结果只提供贴图和复制动作，不显示文件动作。", "info")}
	}
	return []contracts.SearchResult{{
		ID:       "qr-generate",
		Type:     contracts.ResultPluginResult,
		Title:    "生成二维码",
		Subtitle: "二维码生成",
		Detail:   query,
		Icon:     "plugin",
		Tags:     []string{"二维码", "贴图"},
		Payload:  map[string]interface{}{"qrText": query},
		Preview: contracts.PreviewDescriptor{
			Kind:      contracts.PreviewImage,
			Title:     "生成二维码",
			Subtitle:  "内容会在前端预览，并可贴到屏幕",
			Text:      query,
			ImageHint: "QR 预览",
		},
		Actions: []contracts.PreviewAction{
			{ID: "pin_qr", Label: "贴到屏幕", Icon: "pin", Kind: contracts.ActionPin, Payload: map[string]interface{}{"text": query}},
			contracts.CopyAction("copy_qr_text", "复制内容", query, ""),
		},
	}}
}

func executeQRScan(query string) []contracts.SearchResult {
	return []contracts.SearchResult{toolWindowResult("qr-scan", "打开截图历史识别二维码", "二维码识别", "选择截图后点击“识别二维码”，或在截图历史中捕获当前屏幕后识别。", "open_capture_center")}
}

func executeSystem(query string) []contracts.SearchResult {
	commands := map[string]string{
		"lock":     "锁定工作站",
		"sleep":    "进入睡眠模式",
		"empty":    "清空回收站",
		"shutdown": "关闭系统",
		"restart":  "重启系统",
	}
	lower := strings.ToLower(strings.TrimSpace(query))
	results := []contracts.SearchResult{}
	for key, label := range commands {
		if lower == "" || strings.Contains(key, lower) || strings.Contains(label, query) {
			kind := contracts.ActionRun
			if key == "shutdown" || key == "restart" {
				kind = contracts.ActionDanger
			}
			results = append(results, contracts.SearchResult{
				ID:       "system-" + key,
				Type:     contracts.ResultCommand,
				Title:    label,
				Subtitle: "系统命令 · " + key,
				Detail:   "高风险命令需要二次确认。",
				Icon:     "command",
				Tags:     []string{"系统命令"},
				Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: label, Subtitle: "系统命令", Text: "执行前必须由用户确认，Ariadne 不会自动运行高风险系统命令。"},
				Actions: []contracts.PreviewAction{{
					ID:       "run_system",
					Label:    "执行",
					Icon:     "run",
					Kind:     kind,
					Payload:  map[string]interface{}{"command": key, "requiresConfirmation": true},
					Feedback: &contracts.ActionFeedback{SuccessLabel: "已请求" + label, DurationMS: 1400},
				}},
			})
		}
	}
	return results
}

func executeHosts(query string) []contracts.SearchResult {
	return []contracts.SearchResult{toolWindowResult("hosts", "打开 Hosts 管理", "Hosts 管理", "快速切换和编辑系统 Hosts。写入系统 Hosts 属于高风险动作，必须确认。", "open_hosts")}
}

func executeClipboard(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if strings.EqualFold(query, "clear") || query == "清空" {
		return []contracts.SearchResult{{
			ID:       "clipboard-clear",
			Type:     contracts.ResultClipboard,
			Title:    "清空未置顶剪贴板历史",
			Subtitle: "剪贴板历史",
			Detail:   "删除动作需要确认。",
			Icon:     "clipboard",
			Tags:     []string{"剪贴板"},
			Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: "清空未置顶剪贴板历史", Subtitle: "需要确认", Text: "只清理未置顶记录；置顶记录保留。"},
			Actions:  []contracts.PreviewAction{{ID: "clear_unpinned", Label: "确认清理", Icon: "run", Kind: contracts.ActionDanger, Payload: map[string]interface{}{"command": "clipboard:clear_unpinned"}}},
		}}
	}
	return []contracts.SearchResult{toolWindowResult("clipboard-center", "打开剪贴板历史中心", "剪贴板历史", "查看、搜索、置顶并复用历史剪贴板内容。", "open_clipboard_center")}
}

func executeCaptureOverlay(query string) []contracts.SearchResult {
	result := toolWindowResult("capture-overlay", "区域截图", "截图覆盖层", "拖拽选择屏幕区域，保存到截图历史或创建贴图。", "open_capture_overlay")
	result.Icon = "capture"
	return []contracts.SearchResult{result}
}

func executeCaptureHistory(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if strings.EqualFold(query, "clear") || query == "清空" {
		return []contracts.SearchResult{{
			ID:       "capture-clear",
			Type:     contracts.ResultCapture,
			Title:    "清空未置顶捕获历史",
			Subtitle: "捕获历史",
			Detail:   "删除动作需要确认。",
			Icon:     "capture",
			Tags:     []string{"截图", "捕获历史"},
			Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: "清空未置顶捕获历史", Subtitle: "需要确认", Text: "只清理未置顶截图记录；置顶记录保留。"},
			Actions:  []contracts.PreviewAction{{ID: "clear_unpinned", Label: "确认清理", Icon: "run", Kind: contracts.ActionDanger, Payload: map[string]interface{}{"command": "capture:clear_unpinned"}}},
		}}
	}
	overlay := toolWindowResult("capture-overlay", "区域截图", "截图覆盖层", "拖拽选择屏幕区域，保存到截图历史或创建贴图。", "open_capture_overlay")
	overlay.Icon = "capture"
	history := toolWindowResult("capture-center", "打开截图历史中心", "捕获历史", "查看、搜索、复制并复用截图捕获历史。", "open_capture_center")
	if isCaptureOverlayQuery(query) {
		return []contracts.SearchResult{overlay, history}
	}
	return []contracts.SearchResult{history, overlay}
}

func isCaptureOverlayQuery(query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	switch query {
	case "shot", "screenshot", "screen", "region", "select", "overlay", "截图", "区域", "区域截图":
		return true
	default:
		return false
	}
}

func executeNetworkMonitor(query string) []contracts.SearchResult {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "mini" || query == "float" || query == "小窗" || query == "贴边" {
		return []contracts.SearchResult{networkMonitorMiniResult()}
	}
	return []contracts.SearchResult{networkMonitorCenterResult()}
}

func executeWorkflow(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []contracts.SearchResult{toolWindowResult("workflow-center", "打开工作流宏管理", "工作流宏", "查看、编辑和运行命令链工作流。", "open_workflow_center")}
	}
	return nil
}

func executeWorkMemory(query string) []contracts.SearchResult {
	query = strings.ToLower(strings.TrimSpace(query))
	switch query {
	case "", "center":
		return []contracts.SearchResult{toolWindowResult("work-memory-center", "打开工作记忆中心", "工作记忆", "时间线、截图历史、剪贴板、日报、知识草稿和经验发现。", "open_work_memory_center")}
	case "daily", "day", "日报":
		return []contracts.SearchResult{{
			ID:       "work-memory-daily",
			Type:     contracts.ResultWorkflow,
			Title:    "生成今日工作日报",
			Subtitle: "工作记忆 · 日报草稿",
			Detail:   "基于本地工作记忆生成可编辑草稿，并保留证据引用。",
			Icon:     "workflow",
			Tags:     []string{"工作记忆", "日报"},
			Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewWorkflow, Title: "生成今日工作日报", Subtitle: "本地草稿", Text: "不会自动发布或同步；所有外发处理都需要配置和确认。"},
			Actions:  []contracts.PreviewAction{contracts.RunAction("generate_daily", "生成草稿", "mem daily", "Enter")},
		}}
	case "capture", "shot", "补记", "截图":
		return []contracts.SearchResult{{
			ID:       "work-memory-capture-now",
			Type:     contracts.ResultMemory,
			Title:    "手动补记当前屏幕",
			Subtitle: "屏幕时间机器",
			Detail:   "把当前屏幕写入工作记忆时间线。",
			Icon:     "memory",
			Tags:     []string{"工作记忆", "截图"},
			Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewMemory, Title: "手动补记当前屏幕", Subtitle: "用户主动触发", Text: "受隐私模式和排除规则约束；不会绕过敏感内容策略。"},
			Actions:  []contracts.PreviewAction{contracts.RunAction("capture_now", "立即补记", "mem capture", "Enter")},
		}}
	case "pause", "暂停":
		return []contracts.SearchResult{messageResult("work-memory-pause", "暂停工作记忆", "工作记忆", "会暂停屏幕时间机器和自动处理任务。", "memory")}
	case "resume", "恢复":
		return []contracts.SearchResult{messageResult("work-memory-resume", "恢复工作记忆", "工作记忆", "恢复前仍会检查隐私模式和排除规则。", "memory")}
	default:
		return []contracts.SearchResult{copyResult("work-memory-query", "搜索工作记忆: "+query, "工作记忆", query, "将查询发送到本地工作记忆索引。", []string{"工作记忆"})}
	}
}

func copyResult(id string, title string, subtitle string, text string, detail string, tags []string) contracts.SearchResult {
	return contracts.SearchResult{
		ID:       id,
		Type:     contracts.ResultPluginResult,
		Title:    title,
		Subtitle: subtitle,
		Detail:   detail,
		Icon:     "plugin",
		Score:    112,
		Tags:     tags,
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    title,
			Subtitle: subtitle,
			Text:     text,
			Meta: []contracts.LabelValue{
				{Label: "动作来源", Value: "插件显式 preview action"},
			},
		},
		Actions: []contracts.PreviewAction{
			contracts.CopyAction("copy_value", "复制结果", text, "Enter"),
			contracts.RememberAction("remember", "加入记忆", id),
		},
	}
}

func messageResult(id string, title string, subtitle string, text string, icon string) contracts.SearchResult {
	return contracts.SearchResult{
		ID:       id,
		Type:     contracts.ResultPluginResult,
		Title:    title,
		Subtitle: subtitle,
		Detail:   text,
		Icon:     icon,
		Score:    104,
		Tags:     []string{"插件"},
		Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: title, Subtitle: subtitle, Text: text},
		Actions:  []contracts.PreviewAction{contracts.CopyAction("copy_message", "复制说明", text, "")},
	}
}

func disabledPluginResult(plugin PluginManifest) contracts.SearchResult {
	usage := plugin.CommandSchema.Usage
	if usage == "" && len(plugin.Keywords) > 0 {
		usage = plugin.Keywords[0]
	}
	return messageResult(
		"plugin-disabled-"+plugin.ID,
		plugin.Name+" 已停用",
		usage,
		"可以在设置中心的插件区域重新启用。",
		"plugin",
	)
}

func toolWindowResult(id string, title string, subtitle string, text string, command string) contracts.SearchResult {
	return contracts.SearchResult{
		ID:       id,
		Type:     contracts.ResultCommand,
		Title:    title,
		Subtitle: subtitle,
		Detail:   text,
		Icon:     "command",
		Score:    120,
		Tags:     []string{"工具窗口"},
		Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: title, Subtitle: subtitle, Text: text},
		Actions:  []contracts.PreviewAction{contracts.PluginAction("open_tool", title, command)},
	}
}

func networkMonitorCenterResult() contracts.SearchResult {
	result := toolWindowResult("network-monitor", "打开网络监控", "网络监控", "查看实时上传、下载速率和各网卡累计流量。", "open_network_monitor")
	result.Preview.Meta = append(result.Preview.Meta, contracts.LabelValue{Label: "小窗", Value: "可打开右下角贴边网速小窗"})
	result.Actions = append(result.Actions, contracts.PluginAction("open_mini", "打开小窗", "open_network_mini"))
	return result
}

func networkMonitorMiniResult() contracts.SearchResult {
	result := toolWindowResult("network-mini", "打开网速小窗", "网络监控 · 贴边小窗", "打开置顶、锁定尺寸、贴近任务栏右侧的实时网速小窗。", "open_network_mini")
	result.Tags = append(result.Tags, "贴边", "小窗")
	result.Preview.Meta = append(result.Preview.Meta,
		contracts.LabelValue{Label: "位置", Value: "主屏工作区右下角"},
		contracts.LabelValue{Label: "窗口", Value: "置顶、固定尺寸、可回到完整网络监控中心"},
	)
	result.Actions = append(result.Actions, contracts.PluginAction("open_center", "打开完整中心", "open_network_monitor"))
	return result
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func clip(value string, max int) string {
	if len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max]) + "..."
}

func trimFloat(value float64) string {
	if math.Abs(value-math.Round(value)) < 1e-10 {
		return strconv.FormatInt(int64(math.Round(value)), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func evalExpression(input string) (float64, error) {
	parser := mathParser{input: strings.ReplaceAll(input, "x", "*")}
	value, err := parser.parseExpression()
	if err != nil {
		return 0, err
	}
	parser.skipSpaces()
	if parser.pos != len(parser.input) {
		return 0, fmt.Errorf("表达式包含无法解析的字符")
	}
	return value, nil
}

type mathParser struct {
	input string
	pos   int
}

func (p *mathParser) parseExpression() (float64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.match('+') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value += next
		} else if p.match('-') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= next
		} else {
			return value, nil
		}
	}
}

func (p *mathParser) parseTerm() (float64, error) {
	value, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.match('*') {
			next, err := p.parsePower()
			if err != nil {
				return 0, err
			}
			value *= next
		} else if p.match('/') {
			next, err := p.parsePower()
			if err != nil {
				return 0, err
			}
			if next == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			value /= next
		} else if p.match('%') {
			next, err := p.parsePower()
			if err != nil {
				return 0, err
			}
			if next == 0 {
				return 0, fmt.Errorf("取模除数不能为 0")
			}
			value = math.Mod(value, next)
		} else {
			return value, nil
		}
	}
}

func (p *mathParser) parsePower() (float64, error) {
	value, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	p.skipSpaces()
	if p.match('^') {
		next, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		value = math.Pow(value, next)
	}
	return value, nil
}

func (p *mathParser) parseFactor() (float64, error) {
	p.skipSpaces()
	if p.match('+') {
		return p.parseFactor()
	}
	if p.match('-') {
		value, err := p.parseFactor()
		return -value, err
	}
	if p.match('(') {
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if !p.match(')') {
			return 0, fmt.Errorf("缺少右括号")
		}
		return value, nil
	}
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return 0, fmt.Errorf("表达式包含非法字符")
	}
	value, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("数字格式无效")
	}
	return value, nil
}

func (p *mathParser) skipSpaces() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}

func (p *mathParser) match(ch byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}
