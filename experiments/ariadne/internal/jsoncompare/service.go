package jsoncompare

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const (
	defaultMaxReportItems     = 500
	maxReturnedDifferences    = 2000
	maxFormattedPreviewBytes  = 240 * 1024
	maxUnifiedDiffLineProduct = 900000
)

type Difference struct {
	Kind  string      `json:"kind"`
	Path  string      `json:"path"`
	Left  interface{} `json:"left,omitempty"`
	Right interface{} `json:"right,omitempty"`
}

type CompareRequest struct {
	LeftText       string `json:"leftText"`
	RightText      string `json:"rightText"`
	SortKeys       bool   `json:"sortKeys"`
	MaxReportItems int    `json:"maxReportItems"`
}

type CompareResult struct {
	OK                   bool         `json:"ok"`
	Summary              string       `json:"summary"`
	Differences          []Difference `json:"differences"`
	Report               string       `json:"report"`
	UnifiedDiff          string       `json:"unifiedDiff"`
	LeftFormatted        string       `json:"leftFormatted"`
	RightFormatted       string       `json:"rightFormatted"`
	DiffTruncated        bool         `json:"diffTruncated"`
	DifferencesTruncated bool         `json:"differencesTruncated"`
	FormattedTruncated   bool         `json:"formattedTruncated"`
	PerformanceNote      string       `json:"performanceNote,omitempty"`
	Error                string       `json:"error,omitempty"`
	Added                int          `json:"added"`
	Removed              int          `json:"removed"`
	Changed              int          `json:"changed"`
}

type FormatRequest struct {
	Text     string `json:"text"`
	SortKeys bool   `json:"sortKeys"`
	Label    string `json:"label"`
}

type FormatResult struct {
	OK    bool   `json:"ok"`
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Compare(request CompareRequest) CompareResult {
	return CompareJSONText(request)
}

func (s *Service) Format(request FormatRequest) FormatResult {
	label := strings.TrimSpace(request.Label)
	if label == "" {
		label = "JSON"
	}
	parsed, err := parseJSONText(request.Text, label)
	if err != nil {
		message := err.Error()
		return FormatResult{OK: false, Error: message}
	}
	formatted, err := formatJSONText(request.Text, parsed, request.SortKeys)
	if err != nil {
		message := err.Error()
		return FormatResult{OK: false, Error: message}
	}
	return FormatResult{OK: true, Text: formatted}
}

func CompareJSONText(request CompareRequest) CompareResult {
	maxItems := request.MaxReportItems
	if maxItems <= 0 {
		maxItems = defaultMaxReportItems
	}

	leftValue, err := parseJSONText(request.LeftText, "左侧 JSON")
	if err != nil {
		return parseErrorResult(err)
	}
	rightValue, err := parseJSONText(request.RightText, "右侧 JSON")
	if err != nil {
		return parseErrorResult(err)
	}

	differences := compareValues(leftValue, rightValue, "$")
	leftFormatted, leftPreviewTruncated := formatJSONPreview(request.LeftText, leftValue, request.SortKeys)
	rightFormatted, rightPreviewTruncated := formatJSONPreview(request.RightText, rightValue, request.SortKeys)
	unifiedDiff, diffTruncated := buildUnifiedDiff(leftFormatted, rightFormatted)
	formattedTruncated := leftPreviewTruncated || rightPreviewTruncated
	added, removed, changed := countDifferences(differences)
	returnedDifferences, differencesTruncated := truncateDifferences(differences)
	performanceNote := buildPerformanceNote(formattedTruncated, diffTruncated, differencesTruncated)

	return CompareResult{
		OK:                   true,
		Summary:              buildSummary(differences),
		Differences:          returnedDifferences,
		Report:               buildDifferenceReport(differences, maxItems),
		UnifiedDiff:          unifiedDiff,
		LeftFormatted:        leftFormatted,
		RightFormatted:       rightFormatted,
		DiffTruncated:        diffTruncated,
		DifferencesTruncated: differencesTruncated,
		FormattedTruncated:   formattedTruncated,
		PerformanceNote:      performanceNote,
		Added:                added,
		Removed:              removed,
		Changed:              changed,
	}
}

func parseErrorResult(err error) CompareResult {
	message := err.Error()
	return CompareResult{
		OK:          false,
		Summary:     "解析失败",
		Differences: []Difference{},
		Report:      message,
		Error:       message,
	}
}

func parseJSONText(text string, label string) (interface{}, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return nil, fmt.Errorf("%s 不能为空", label)
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, jsonParseError(raw, label, err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%s 解析失败: 包含多个 JSON 值", label)
		}
		return nil, jsonParseError(raw, label, err)
	}
	return value, nil
}

func jsonParseError(raw string, label string, err error) error {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		line, column := lineColumn(raw, syntaxErr.Offset)
		return fmt.Errorf("%s 解析失败: 第 %d 行，第 %d 列，%s", label, line, column, err.Error())
	}
	return fmt.Errorf("%s 解析失败: %s", label, err.Error())
}

func lineColumn(raw string, offset int64) (int, int) {
	if offset < 1 {
		return 1, 1
	}
	line := 1
	column := 1
	for index, r := range raw {
		if int64(index) >= offset-1 {
			break
		}
		if r == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func formatJSONText(raw string, value interface{}, sortKeys bool) (string, error) {
	if !sortKeys {
		var buffer bytes.Buffer
		if err := json.Indent(&buffer, []byte(strings.TrimSpace(raw)), "", "  "); err == nil {
			return buffer.String(), nil
		}
	}
	return marshalIndented(value)
}

func formatJSONPreview(raw string, value interface{}, sortKeys bool) (string, bool) {
	formatted, err := formatJSONText(raw, value, sortKeys)
	if err != nil {
		formatted = strings.TrimSpace(raw)
	}
	return truncatePreview(formatted, maxFormattedPreviewBytes)
}

func truncatePreview(text string, maxBytes int) (string, bool) {
	if len(text) <= maxBytes {
		return text, false
	}
	cut := maxBytes
	for cut > 0 && (text[cut]&0xC0) == 0x80 {
		cut--
	}
	if cut <= 0 {
		cut = maxBytes
	}
	return text[:cut] + fmt.Sprintf("\n\n... 已截断格式化预览，完整输入为 %d bytes", len(text)), true
}

func marshalIndented(value interface{}) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

func buildSummary(differences []Difference) string {
	if len(differences) == 0 {
		return "两个 JSON 语义一致"
	}
	added, removed, changed := countDifferences(differences)
	return fmt.Sprintf("发现 %d 处差异：新增 %d，删除 %d，变更 %d", len(differences), added, removed, changed)
}

func countDifferences(differences []Difference) (int, int, int) {
	added := 0
	removed := 0
	changed := 0
	for _, difference := range differences {
		switch difference.Kind {
		case "added":
			added++
		case "removed":
			removed++
		case "changed":
			changed++
		}
	}
	return added, removed, changed
}

func truncateDifferences(differences []Difference) ([]Difference, bool) {
	if len(differences) <= maxReturnedDifferences {
		return differences, false
	}
	return differences[:maxReturnedDifferences], true
}

func buildDifferenceReport(differences []Difference, maxItems int) string {
	if len(differences) == 0 {
		return "两个 JSON 语义一致。对象字段顺序不会被判定为差异。"
	}
	lines := []string{buildSummary(differences), ""}
	limit := maxItems
	if limit > len(differences) {
		limit = len(differences)
	}
	for _, difference := range differences[:limit] {
		switch difference.Kind {
		case "added":
			lines = append(lines, fmt.Sprintf("+ %s: %s", difference.Path, summarizeValue(difference.Right, 160)))
		case "removed":
			lines = append(lines, fmt.Sprintf("- %s: %s", difference.Path, summarizeValue(difference.Left, 160)))
		default:
			lines = append(lines, fmt.Sprintf("~ %s: %s -> %s", difference.Path, summarizeValue(difference.Left, 160), summarizeValue(difference.Right, 160)))
		}
	}
	if remaining := len(differences) - limit; remaining > 0 {
		lines = append(lines, "", fmt.Sprintf("... 还有 %d 处差异未展示", remaining))
	}
	return strings.Join(lines, "\n")
}

func summarizeValue(value interface{}, limit int) string {
	text := marshalCompact(value)
	if len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit-3]) + "..."
}

func marshalCompact(value interface{}) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Sprint(value)
	}
	return strings.TrimSpace(buffer.String())
}

func buildUnifiedDiff(leftFormatted string, rightFormatted string) (string, bool) {
	if leftFormatted == rightFormatted {
		return "(规范化格式后没有行差异)", false
	}
	leftLines := strings.Split(leftFormatted, "\n")
	rightLines := strings.Split(rightFormatted, "\n")
	if shouldSkipUnifiedDiff(leftLines, rightLines) {
		return fmt.Sprintf("行级 diff 已跳过：左侧 %d 行，右侧 %d 行，超过当前性能预算。语义差异统计和报告仍可用。", len(leftLines), len(rightLines)), true
	}
	lines := []string{"--- left.json", "+++ right.json", "@@"}
	for _, op := range diffLines(leftLines, rightLines) {
		lines = append(lines, op)
	}
	return strings.Join(lines, "\n"), false
}

func shouldSkipUnifiedDiff(leftLines []string, rightLines []string) bool {
	if len(leftLines) == 0 || len(rightLines) == 0 {
		return false
	}
	return len(leftLines) > maxUnifiedDiffLineProduct/len(rightLines)
}

func buildPerformanceNote(formattedTruncated bool, diffTruncated bool, differencesTruncated bool) string {
	notes := []string{}
	if formattedTruncated {
		notes = append(notes, fmt.Sprintf("格式化预览已按 %d KiB 截断", maxFormattedPreviewBytes/1024))
	}
	if diffTruncated {
		notes = append(notes, "行级 diff 超过预算已跳过")
	}
	if differencesTruncated {
		notes = append(notes, fmt.Sprintf("差异明细已按 %d 条截断", maxReturnedDifferences))
	}
	if len(notes) == 0 {
		return ""
	}
	return strings.Join(notes, "；")
}

func diffLines(left []string, right []string) []string {
	table := make([][]int, len(left)+1)
	for index := range table {
		table[index] = make([]int, len(right)+1)
	}
	for i := len(left) - 1; i >= 0; i-- {
		for j := len(right) - 1; j >= 0; j-- {
			if left[i] == right[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}

	lines := []string{}
	i := 0
	j := 0
	for i < len(left) && j < len(right) {
		if left[i] == right[j] {
			lines = append(lines, " "+left[i])
			i++
			j++
		} else if table[i+1][j] >= table[i][j+1] {
			lines = append(lines, "-"+left[i])
			i++
		} else {
			lines = append(lines, "+"+right[j])
			j++
		}
	}
	for i < len(left) {
		lines = append(lines, "-"+left[i])
		i++
	}
	for j < len(right) {
		lines = append(lines, "+"+right[j])
		j++
	}
	return lines
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func compareValues(left interface{}, right interface{}, path string) []Difference {
	if reflect.TypeOf(left) != reflect.TypeOf(right) {
		return []Difference{{Kind: "changed", Path: path, Left: left, Right: right}}
	}

	switch leftValue := left.(type) {
	case map[string]interface{}:
		rightValue := right.(map[string]interface{})
		return compareObjects(leftValue, rightValue, path)
	case []interface{}:
		rightValue := right.([]interface{})
		return compareArrays(leftValue, rightValue, path)
	default:
		if !reflect.DeepEqual(left, right) {
			return []Difference{{Kind: "changed", Path: path, Left: left, Right: right}}
		}
		return nil
	}
}

func compareObjects(left map[string]interface{}, right map[string]interface{}, path string) []Difference {
	differences := []Difference{}
	leftKeys := sortedKeys(left)
	rightKeys := sortedKeys(right)
	rightLookup := map[string]bool{}
	leftLookup := map[string]bool{}
	for _, key := range rightKeys {
		rightLookup[key] = true
	}
	for _, key := range leftKeys {
		leftLookup[key] = true
		if !rightLookup[key] {
			differences = append(differences, Difference{Kind: "removed", Path: joinObjectPath(path, key), Left: left[key]})
		}
	}
	for _, key := range rightKeys {
		if !leftLookup[key] {
			differences = append(differences, Difference{Kind: "added", Path: joinObjectPath(path, key), Right: right[key]})
		}
	}
	for _, key := range leftKeys {
		if rightLookup[key] {
			differences = append(differences, compareValues(left[key], right[key], joinObjectPath(path, key))...)
		}
	}
	return differences
}

func sortedKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compareArrays(left []interface{}, right []interface{}, path string) []Difference {
	differences := []Difference{}
	commonLength := len(left)
	if len(right) < commonLength {
		commonLength = len(right)
	}
	for index := 0; index < commonLength; index++ {
		differences = append(differences, compareValues(left[index], right[index], fmt.Sprintf("%s[%d]", path, index))...)
	}
	for index := commonLength; index < len(left); index++ {
		differences = append(differences, Difference{Kind: "removed", Path: fmt.Sprintf("%s[%d]", path, index), Left: left[index]})
	}
	for index := commonLength; index < len(right); index++ {
		differences = append(differences, Difference{Kind: "added", Path: fmt.Sprintf("%s[%d]", path, index), Right: right[index]})
	}
	return differences
}

func joinObjectPath(path string, key string) string {
	if identifierPattern.MatchString(key) {
		return path + "." + key
	}
	return path + "[" + marshalCompact(key) + "]"
}
