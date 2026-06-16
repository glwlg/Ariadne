package jsoncompare

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompareIgnoresObjectKeyOrder(t *testing.T) {
	result := CompareJSONText(CompareRequest{
		LeftText:       `{"b":2,"a":1}`,
		RightText:      `{"a":1,"b":2}`,
		SortKeys:       true,
		MaxReportItems: 500,
	})

	if !result.OK {
		t.Fatalf("expected ok result, got error %q", result.Error)
	}
	if len(result.Differences) != 0 {
		t.Fatalf("expected no differences, got %#v", result.Differences)
	}
	if result.Summary != "两个 JSON 语义一致" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if !strings.Contains(result.UnifiedDiff, "没有行差异") {
		t.Fatalf("expected no line diff marker, got %q", result.UnifiedDiff)
	}
}

func TestCompareReportsAddedRemovedAndChangedPaths(t *testing.T) {
	left := `{"name":"same","meta":{"enabled":true,"drop":1},"items":[1,2]}`
	right := `{"name":"same","meta":{"enabled":true,"add":2},"items":[1,3,4]}`

	result := CompareJSONText(CompareRequest{LeftText: left, RightText: right, SortKeys: true})

	if !result.OK {
		t.Fatalf("expected ok result, got error %q", result.Error)
	}
	if result.Summary != "发现 4 处差异：新增 2，删除 1，变更 1" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	for _, expected := range []string{
		"- $.meta.drop: 1",
		"+ $.meta.add: 2",
		"~ $.items[1]: 2 -> 3",
		"+ $.items[2]: 4",
	} {
		if !strings.Contains(result.Report, expected) {
			t.Fatalf("expected report to contain %q, got:\n%s", expected, result.Report)
		}
	}
	if result.Added != 2 || result.Removed != 1 || result.Changed != 1 {
		t.Fatalf("unexpected counters: added=%d removed=%d changed=%d", result.Added, result.Removed, result.Changed)
	}
}

func TestCompareReturnsParseErrorWithLocation(t *testing.T) {
	result := CompareJSONText(CompareRequest{
		LeftText:  `{"ok": true}`,
		RightText: `{"bad": }`,
		SortKeys:  true,
	})

	if result.OK {
		t.Fatalf("expected parse failure")
	}
	if result.Summary != "解析失败" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if !strings.Contains(result.Error, "右侧 JSON 解析失败") {
		t.Fatalf("expected right-side parse error, got %q", result.Error)
	}
	if !strings.Contains(result.Error, "第 1 行") {
		t.Fatalf("expected line number in error, got %q", result.Error)
	}
}

func TestCompareFormatsNonIdentifierObjectPath(t *testing.T) {
	result := CompareJSONText(CompareRequest{
		LeftText:  `{"bad-key":1}`,
		RightText: `{"bad-key":2}`,
		SortKeys:  true,
	})

	if !result.OK {
		t.Fatalf("expected ok result, got error %q", result.Error)
	}
	if !strings.Contains(result.Report, `~ $["bad-key"]: 1 -> 2`) {
		t.Fatalf("expected quoted path, got:\n%s", result.Report)
	}
}

func TestFormatPreservesInputOrderWhenSortKeysDisabled(t *testing.T) {
	result := NewService().Format(FormatRequest{
		Text:     `{"b":2,"a":1}`,
		SortKeys: false,
		Label:    "左侧 JSON",
	})

	if !result.OK {
		t.Fatalf("expected ok format, got error %q", result.Error)
	}
	if !strings.Contains(result.Text, `"b": 2`) || !strings.Contains(result.Text, `"a": 1`) {
		t.Fatalf("unexpected formatted text: %s", result.Text)
	}
	if strings.Index(result.Text, `"b": 2`) > strings.Index(result.Text, `"a": 1`) {
		t.Fatalf("expected original key order when sortKeys is false, got:\n%s", result.Text)
	}
}

func TestCompareSkipsLargeUnifiedDiffButKeepsSemanticReport(t *testing.T) {
	leftItems := make([]string, 1100)
	rightItems := make([]string, 1100)
	for index := range leftItems {
		leftItems[index] = fmt.Sprintf("%d", index)
		rightItems[index] = fmt.Sprintf("%d", index)
	}
	rightItems[len(rightItems)-1] = "999999"

	result := CompareJSONText(CompareRequest{
		LeftText:  "[" + strings.Join(leftItems, ",") + "]",
		RightText: "[" + strings.Join(rightItems, ",") + "]",
		SortKeys:  true,
	})

	if !result.OK {
		t.Fatalf("expected ok result, got %q", result.Error)
	}
	if !result.DiffTruncated {
		t.Fatalf("expected large unified diff to be skipped")
	}
	if !strings.Contains(result.UnifiedDiff, "行级 diff 已跳过") {
		t.Fatalf("expected skip message, got %q", result.UnifiedDiff)
	}
	if result.Changed != 1 || !strings.Contains(result.Report, "~ $[1099]: 1099 -> 999999") {
		t.Fatalf("expected semantic report to remain useful, changed=%d report=%q", result.Changed, result.Report)
	}
	if !strings.Contains(result.PerformanceNote, "行级 diff") {
		t.Fatalf("expected performance note, got %q", result.PerformanceNote)
	}
}

func TestCompareTruncatesHugeFormattedPreview(t *testing.T) {
	largeValue := strings.Repeat("x", maxFormattedPreviewBytes+4096)
	payload := fmt.Sprintf(`{"payload":%q}`, largeValue)

	result := CompareJSONText(CompareRequest{
		LeftText:  payload,
		RightText: payload,
		SortKeys:  true,
	})

	if !result.OK {
		t.Fatalf("expected ok result, got %q", result.Error)
	}
	if !result.FormattedTruncated {
		t.Fatalf("expected formatted preview to be truncated")
	}
	if !strings.Contains(result.LeftFormatted, "已截断格式化预览") || !strings.Contains(result.RightFormatted, "已截断格式化预览") {
		t.Fatalf("expected truncation marker in previews")
	}
	if result.DiffTruncated {
		t.Fatalf("identical truncated previews should not need diff truncation")
	}
	if result.Summary != "两个 JSON 语义一致" {
		t.Fatalf("unexpected semantic summary: %s", result.Summary)
	}
}

func TestCompareTruncatesReturnedDifferencesButKeepsCounters(t *testing.T) {
	leftItems := make([]string, maxReturnedDifferences+250)
	rightItems := make([]string, maxReturnedDifferences+250)
	for index := range leftItems {
		leftItems[index] = fmt.Sprintf("%d", index)
		rightItems[index] = fmt.Sprintf("%d", index+100000)
	}

	result := CompareJSONText(CompareRequest{
		LeftText:       "[" + strings.Join(leftItems, ",") + "]",
		RightText:      "[" + strings.Join(rightItems, ",") + "]",
		SortKeys:       true,
		MaxReportItems: 10,
	})

	if !result.OK {
		t.Fatalf("expected ok result, got %q", result.Error)
	}
	if !result.DifferencesTruncated {
		t.Fatalf("expected returned differences to be truncated")
	}
	if len(result.Differences) != maxReturnedDifferences {
		t.Fatalf("expected %d returned differences, got %d", maxReturnedDifferences, len(result.Differences))
	}
	if result.Changed != maxReturnedDifferences+250 {
		t.Fatalf("expected full changed counter, got %d", result.Changed)
	}
	if !strings.Contains(result.Summary, fmt.Sprintf("发现 %d 处差异", maxReturnedDifferences+250)) {
		t.Fatalf("expected full count summary, got %q", result.Summary)
	}
	if !strings.Contains(result.PerformanceNote, "差异明细") {
		t.Fatalf("expected differences truncation note, got %q", result.PerformanceNote)
	}
}
