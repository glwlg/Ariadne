package aiclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ariadne/internal/ocr"
	"ariadne/internal/securestore"
)

type OpenAICompatibleImageOCR struct {
	HTTPClient    *http.Client
	APIKeyEnv     []string
	SecretTargets []string
}

const ocrUploadMaxSide = 2600

func NewOpenAICompatibleImageOCR() *OpenAICompatibleImageOCR {
	return &OpenAICompatibleImageOCR{
		HTTPClient:    &http.Client{Timeout: 60 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_OCR_API_KEY", "ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetOpenAIAPIKey},
	}
}

func (o *OpenAICompatibleImageOCR) RecognizeImageOCR(ctx context.Context, job ocr.AIOCRJob) (ocr.AIResult, error) {
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if isOllamaGenerateProvider(provider) {
		return o.recognizeImageOllamaGenerate(ctx, job)
	}
	if !isOpenAICompatibleProvider(provider) {
		return ocr.AIResult{}, fmt.Errorf("不支持的大模型 OCR provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return ocr.AIResult{}, errors.New("大模型 OCR model 未配置")
	}
	imagePath := strings.TrimSpace(job.ImagePath)
	if imagePath == "" {
		return ocr.AIResult{}, errors.New("大模型 OCR 图片路径为空")
	}
	imagePayload, err := readOCRImagePayload(imagePath)
	if err != nil {
		return ocr.AIResult{}, err
	}
	apiKey := o.apiKey()
	endpoint := openAIChatCompletionsEndpoint(job.BaseURL)
	if apiKey == "" && strings.HasPrefix(endpoint, "https://api.openai.com/") {
		return ocr.AIResult{}, errors.New("未检测到 ARIADNE_OCR_API_KEY、ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
	}

	payload := visionChatCompletionRequest{
		Model:       model,
		Temperature: 0,
		Messages: []visionChatMessage{
			{
				Role:    "system",
				Content: "你是 Ariadne 的视觉 OCR 引擎。只识别图片中实际可见的文字，保持原文语言，按阅读顺序输出纯文本；不要解释、不要 Markdown、不要编造不可见内容，密钥和 token 只输出占位符 [REDACTED]。",
			},
			{
				Role: "user",
				Content: []visionContentPart{
					{Type: "text", Text: "ocr this image"},
					{
						Type: "image_url",
						ImageURL: &visionImageURL{
							URL:    "data:" + imagePayload.MimeType + ";base64," + base64.StdEncoding.EncodeToString(imagePayload.Data),
							Detail: "high",
						},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ocr.AIResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return ocr.AIResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := o.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return ocr.AIResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return ocr.AIResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ocr.AIResult{}, fmt.Errorf("大模型 OCR provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 240))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ocr.AIResult{}, err
	}
	if len(result.Choices) == 0 {
		return ocr.AIResult{}, errors.New("大模型 OCR provider 未返回 choices")
	}
	text, lines := parseImageOCRContent(result.Choices[0].Message.Content)
	if text == "" {
		return ocr.AIResult{}, errors.New("大模型 OCR provider 返回空内容")
	}
	return ocr.AIResult{
		Provider:  "vision:" + model,
		Text:      text,
		Lines:     lines,
		ElapsedMs: int(time.Since(started) / time.Millisecond),
	}, nil
}

func (o *OpenAICompatibleImageOCR) recognizeImageOllamaGenerate(ctx context.Context, job ocr.AIOCRJob) (ocr.AIResult, error) {
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return ocr.AIResult{}, errors.New("Ollama OCR model 未配置")
	}
	imagePath := strings.TrimSpace(job.ImagePath)
	if imagePath == "" {
		return ocr.AIResult{}, errors.New("Ollama OCR 图片路径为空")
	}
	imagePayload, err := readOCRImagePayload(imagePath)
	if err != nil {
		return ocr.AIResult{}, err
	}

	payload := ollamaGenerateRequest{
		Model:  model,
		Prompt: imageOCRPrompt(),
		Images: []string{base64.StdEncoding.EncodeToString(imagePayload.Data)},
		Stream: false,
		Options: map[string]any{
			"temperature": 0,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ocr.AIResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaGenerateEndpoint(job.BaseURL), bytes.NewReader(raw))
	if err != nil {
		return ocr.AIResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	client := o.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return ocr.AIResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return ocr.AIResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ocr.AIResult{}, fmt.Errorf("Ollama OCR 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 240))
	}

	var result ollamaGenerateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ocr.AIResult{}, err
	}
	if strings.TrimSpace(result.Error) != "" {
		return ocr.AIResult{}, errors.New("Ollama OCR 返回错误: " + strings.TrimSpace(result.Error))
	}
	text := strings.TrimSpace(result.Response)
	if text == "" {
		return ocr.AIResult{}, errors.New("Ollama OCR 返回空内容")
	}
	return ocr.AIResult{
		Provider:  "ollama-generate:" + model,
		Text:      text,
		Lines:     textToOCRLines(text),
		ElapsedMs: int(time.Since(started) / time.Millisecond),
	}, nil
}

func (o *OpenAICompatibleImageOCR) apiKey() string {
	envs := o.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_OCR_API_KEY", "ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	return apiKeyFromSources(envs, o.SecretTargets)
}

type credentialReadFunc func(target string) (string, bool, error)

func apiKeyFromSources(envs []string, targets []string) string {
	return apiKeyFromSourcesWithReader(envs, targets, securestore.Read)
}

func apiKeyFromSourcesWithReader(envs []string, targets []string, read credentialReadFunc) string {
	if token := apiKeyFromEnv(envs, true); token != "" {
		return token
	}
	if token := apiKeyFromCredentialManagerWithReader(targets, read); token != "" {
		return token
	}
	return apiKeyFromEnv(envs, false)
}

func apiKeyFromEnv(envs []string, ariadneOnly bool) string {
	for _, name := range envs {
		if isAriadneAPIKeyEnv(name) != ariadneOnly {
			continue
		}
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func apiKeyFromCredentialManagerWithReader(targets []string, read credentialReadFunc) string {
	if read == nil {
		return ""
	}
	for _, target := range targets {
		value, ok, err := read(target)
		if err == nil && ok {
			if token := cleanAPIKey(value); token != "" {
				return token
			}
		}
	}
	return ""
}

func isAriadneAPIKeyEnv(name string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(name)), "ARIADNE_")
}

func cleanAPIKey(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if key, candidate, ok := strings.Cut(line, "="); ok {
			normalized := strings.ToUpper(strings.TrimSpace(key))
			if strings.Contains(normalized, "API_KEY") || strings.Contains(normalized, "TOKEN") {
				value = candidate
				break
			}
		}
	}
	if hash := strings.Index(value, "#"); hash >= 0 {
		value = value[:hash]
	}
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\x00", "")
	return strings.TrimSpace(value)
}

func imageOCRPrompt() string {
	return `请识别图片中所有可见文字。

要求：
1. 按屏幕阅读顺序输出纯文本，尽量保持换行。
2. 中文、英文、路径、命令和数字都要保留。
3. 删除明显的装饰噪声，不要输出图像描述。
4. 不要输出 Markdown、JSON、前言或解释。
5. 如果看不清或没有文字，只输出“未识别到文字”。`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type visionChatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []visionChatMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
}

type visionChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type visionContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

type visionImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Images  []string       `json:"images,omitempty"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

type ocrImagePayload struct {
	Data     []byte
	MimeType string
	Width    int
	Height   int
	Resized  bool
}

func imageMimeType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/png"
	}
}

func readOCRImagePayload(imagePath string) (ocrImagePayload, error) {
	rawImage, err := os.ReadFile(imagePath)
	if err != nil {
		return ocrImagePayload{}, fmt.Errorf("读取 OCR 图片失败: %w", err)
	}
	if len(rawImage) == 0 {
		return ocrImagePayload{}, errors.New("OCR 图片为空")
	}
	payload := ocrImagePayload{
		Data:     rawImage,
		MimeType: imageMimeType(imagePath),
	}
	resized, width, height, ok, err := resizeOCRImageForUpload(rawImage, ocrUploadMaxSide)
	if err != nil {
		return payload, nil
	}
	payload.Width = width
	payload.Height = height
	if ok {
		payload.Data = resized
		payload.MimeType = "image/png"
		payload.Resized = true
	}
	return payload, nil
}

func resizeOCRImageForUpload(rawImage []byte, maxSide int) ([]byte, int, int, bool, error) {
	if maxSide <= 0 {
		maxSide = ocrUploadMaxSide
	}
	src, _, err := image.Decode(bytes.NewReader(rawImage))
	if err != nil {
		return nil, 0, 0, false, err
	}
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, 0, 0, false, fmt.Errorf("OCR 图片尺寸无效")
	}
	if width <= maxSide && height <= maxSide {
		return nil, width, height, false, nil
	}

	scale := float64(maxSide) / float64(max(width, height))
	resizedWidth := max(1, int(math.Round(float64(width)*scale)))
	resizedHeight := max(1, int(math.Round(float64(height)*scale)))
	dst := image.NewNRGBA(image.Rect(0, 0, resizedWidth, resizedHeight))
	for y := 0; y < resizedHeight; y++ {
		sourceY := bounds.Min.Y + min(height-1, int(float64(y)*float64(height)/float64(resizedHeight)))
		for x := 0; x < resizedWidth; x++ {
			sourceX := bounds.Min.X + min(width-1, int(float64(x)*float64(width)/float64(resizedWidth)))
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(sourceX, sourceY)).(color.NRGBA))
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, width, height, false, err
	}
	return out.Bytes(), resizedWidth, resizedHeight, true, nil
}

func openAIChatCompletionsEndpoint(baseURL string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint == "" {
		return "https://api.openai.com/v1/chat/completions"
	}
	if strings.HasSuffix(strings.ToLower(endpoint), "/chat/completions") {
		return endpoint
	}
	return endpoint + "/chat/completions"
}

func isOpenAICompatibleProvider(provider string) bool {
	provider = strings.TrimSpace(strings.ToLower(provider))
	return provider == "openai-compatible" || provider == "openai"
}

func isOllamaGenerateProvider(provider string) bool {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "ollama", "ollama-generate", "ollama_generate":
		return true
	default:
		return false
	}
}

func ollamaGenerateEndpoint(baseURL string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint == "" {
		return "http://localhost:11434/api/generate"
	}
	lower := strings.ToLower(endpoint)
	if strings.HasSuffix(lower, "/api/generate") {
		return endpoint
	}
	if strings.HasSuffix(lower, "/api") {
		return endpoint + "/generate"
	}
	return endpoint + "/api/generate"
}

func textToOCRLines(text string) []ocr.Line {
	lines := []ocr.Line{}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, ocr.Line{Text: line, Confidence: 1})
	}
	return lines
}

func parseImageOCRContent(content string) (string, []ocr.Line) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return content, textToOCRLines(content)
	}

	lines := extractStructuredOCRLines(payload)
	text := strings.TrimSpace(stringFromAny(payload["text"]))
	if text == "" {
		parts := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.TrimSpace(line.Text) != "" {
				parts = append(parts, strings.TrimSpace(line.Text))
			}
		}
		text = strings.Join(parts, "\n")
	}
	if len(lines) == 0 {
		lines = textToOCRLines(text)
	}
	return text, lines
}

func extractStructuredOCRLines(payload map[string]any) []ocr.Line {
	lines := []ocr.Line{}
	appendLineItems := func(value any) {
		for _, item := range listFromAny(value) {
			if line, ok := ocrLineFromAny(item); ok {
				lines = append(lines, line)
			}
		}
	}
	appendPageLines := func(page any) {
		if pageMap, ok := page.(map[string]any); ok {
			appendLineItems(pageMap["lines"])
		}
	}

	appendLineItems(payload["lines"])
	for _, page := range listFromAny(payload["pages"]) {
		appendPageLines(page)
	}
	for _, image := range listFromAny(payload["images"]) {
		imageMap, ok := image.(map[string]any)
		if !ok {
			continue
		}
		appendLineItems(imageMap["lines"])
		for _, page := range listFromAny(imageMap["pages"]) {
			appendPageLines(page)
		}
	}
	return lines
}

func ocrLineFromAny(value any) (ocr.Line, bool) {
	switch item := value.(type) {
	case string:
		text := strings.TrimSpace(item)
		return ocr.Line{Text: text, Confidence: 1}, text != ""
	case map[string]any:
		text := firstStringField(item, "text", "content", "transcription", "value")
		if text == "" {
			return ocr.Line{}, false
		}
		line := ocr.Line{
			Text:       text,
			Confidence: firstFloatField(item, "confidence", "score", "prob"),
			Rect:       rectFromOCRMap(item),
		}
		if line.Confidence == 0 {
			line.Confidence = 1
		}
		return line, true
	case []any:
		return ocrLineFromArray(item)
	default:
		return ocr.Line{}, false
	}
}

func ocrLineFromArray(items []any) (ocr.Line, bool) {
	if len(items) == 0 {
		return ocr.Line{}, false
	}
	if len(items) >= 2 {
		if text := strings.TrimSpace(stringFromAny(items[1])); text != "" {
			confidence := floatFromAny(nil)
			if len(items) >= 3 {
				confidence = floatFromAny(items[2])
			}
			if confidence == 0 {
				confidence = 1
			}
			return ocr.Line{Text: text, Confidence: confidence, Rect: rectFromAny(items[0])}, true
		}
	}
	if text := strings.TrimSpace(stringFromAny(items[0])); text != "" {
		confidence := 1.0
		if len(items) >= 2 {
			if value := floatFromAny(items[1]); value > 0 {
				confidence = value
			}
		}
		return ocr.Line{Text: text, Confidence: confidence}, true
	}
	return ocr.Line{}, false
}

func rectFromOCRMap(item map[string]any) ocr.Rect {
	for _, key := range []string{"rect", "bbox", "box", "points", "poly", "polygon"} {
		if rect := rectFromAny(item[key]); rect.Width > 0 || rect.Height > 0 {
			return rect
		}
	}
	return ocr.Rect{}
}

func rectFromAny(value any) ocr.Rect {
	if item, ok := value.(map[string]any); ok {
		return ocr.Rect{
			X:      int(firstFloatField(item, "x", "left")),
			Y:      int(firstFloatField(item, "y", "top")),
			Width:  int(firstFloatField(item, "width", "w")),
			Height: int(firstFloatField(item, "height", "h")),
		}
	}
	values := listFromAny(value)
	if len(values) >= 4 && allNumberLike(values[:4]) {
		x1 := floatFromAny(values[0])
		y1 := floatFromAny(values[1])
		x2 := floatFromAny(values[2])
		y2 := floatFromAny(values[3])
		return ocr.Rect{
			X:      int(x1),
			Y:      int(y1),
			Width:  maxInt(0, int(x2-x1)),
			Height: maxInt(0, int(y2-y1)),
		}
	}
	xs := []float64{}
	ys := []float64{}
	for _, point := range values {
		coords := listFromAny(point)
		if len(coords) < 2 {
			continue
		}
		xs = append(xs, floatFromAny(coords[0]))
		ys = append(ys, floatFromAny(coords[1]))
	}
	if len(xs) == 0 || len(ys) == 0 {
		return ocr.Rect{}
	}
	minX, maxX := minMax(xs)
	minY, maxY := minMax(ys)
	return ocr.Rect{
		X:      int(minX),
		Y:      int(minY),
		Width:  maxInt(0, int(maxX-minX)),
		Height: maxInt(0, int(maxY-minY)),
	}
}

func firstStringField(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(stringFromAny(item[key])); value != "" {
			return value
		}
	}
	return ""
}

func firstFloatField(item map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value := floatFromAny(item[key]); value > 0 {
			return value
		}
	}
	return 0
}

func listFromAny(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func floatFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		result, _ := typed.Float64()
		return result
	default:
		return 0
	}
}

func allNumberLike(values []any) bool {
	for _, value := range values {
		if floatFromAny(value) == 0 && fmt.Sprint(value) != "0" {
			return false
		}
	}
	return true
}

func minMax(values []float64) (float64, float64) {
	minValue := values[0]
	maxValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	return minValue, maxValue
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
