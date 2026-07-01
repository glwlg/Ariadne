package apitesting

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeoutSeconds   = 30
	maxTimeoutSeconds       = 120
	maxResponseBodyBytes    = 512 * 1024
	maxEventStreamBodyBytes = 64 * 1024
)

var (
	templateTokenPattern  = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_$.-]+)\s*\}\}`)
	eventStreamURLPattern = regexp.MustCompile(`(?i)(^|[/?&#._-])(sse|stream)([/?&#._-]|$)|event-stream`)
)

type Header struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type Param struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type Variable struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
	Secret  bool   `json:"secret,omitempty"`
}

type Assertion struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Operator string `json:"operator"`
	Expected string `json:"expected"`
	Enabled  bool   `json:"enabled"`
}

type Request struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Folder     string      `json:"folder"`
	Method     string      `json:"method"`
	URL        string      `json:"url"`
	BodyType   string      `json:"bodyType"`
	Body       string      `json:"body"`
	Params     []Param     `json:"params"`
	Headers    []Header    `json:"headers"`
	Assertions []Assertion `json:"assertions"`
	UpdatedAt  int64       `json:"updatedAt"`
}

type Environment struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Variables []Variable `json:"variables"`
	UpdatedAt int64      `json:"updatedAt"`
}

type Collection struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	Variables           []Variable    `json:"variables"`
	Environments        []Environment `json:"environments"`
	Requests            []Request     `json:"requests"`
	Git                 GitConfig     `json:"git,omitempty"`
	ActiveEnvironmentID string        `json:"activeEnvironmentId"`
	ActiveRequestID     string        `json:"activeRequestId"`
	UpdatedAt           int64         `json:"updatedAt"`
}

type Status struct {
	Path               string       `json:"path"`
	DatabasePath       string       `json:"databasePath"`
	Collections        []Collection `json:"collections"`
	ActiveCollectionID string       `json:"activeCollectionId"`
	CollectionCount    int          `json:"collectionCount"`
	RequestCount       int          `json:"requestCount"`
	LastSaveError      string       `json:"lastSaveError,omitempty"`
	LastLoadError      string       `json:"lastLoadError,omitempty"`
}

type RunRequest struct {
	CollectionID   string  `json:"collectionId"`
	EnvironmentID  string  `json:"environmentId"`
	Request        Request `json:"request"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
	RunID          string  `json:"runId,omitempty"`
	Stream         bool    `json:"stream,omitempty"`
}

type RunStopResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type RunSnapshot struct {
	OK        bool      `json:"ok"`
	Running   bool      `json:"running"`
	Message   string    `json:"message"`
	UpdatedAt int64     `json:"updatedAt,omitempty"`
	Result    RunResult `json:"result"`
}

type AssertionResult struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Operator string `json:"operator"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
}

type RunResult struct {
	OK               bool              `json:"ok"`
	Message          string            `json:"message"`
	Method           string            `json:"method"`
	RequestURL       string            `json:"requestUrl"`
	StatusCode       int               `json:"statusCode"`
	StatusText       string            `json:"statusText"`
	DurationMs       int64             `json:"durationMs"`
	Headers          []Header          `json:"headers"`
	Body             string            `json:"body"`
	BodySize         int               `json:"bodySize"`
	BodyTruncated    bool              `json:"bodyTruncated"`
	ContentType      string            `json:"contentType"`
	Streaming        bool              `json:"streaming,omitempty"`
	AssertionResults []AssertionResult `json:"assertionResults"`
	Passed           int               `json:"passed"`
	Failed           int               `json:"failed"`
	Error            string            `json:"error,omitempty"`
	MissingVariables []string          `json:"missingVariables,omitempty"`
}

type ImportResult struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	ImportedCount int    `json:"importedCount"`
	Error         string `json:"error,omitempty"`
	Status        Status `json:"status"`
}

type state struct {
	Version            int          `json:"version"`
	ActiveCollectionID string       `json:"activeCollectionId"`
	Collections        []Collection `json:"collections"`
}

type Service struct {
	mu             sync.RWMutex
	runningMu      sync.Mutex
	path           string
	collections    []Collection
	activeID       string
	lastSaveError  string
	lastLoadError  string
	client         *http.Client
	defaultTimeout time.Duration
	running        map[string]*runningRun
}

type runningRun struct {
	mu        sync.RWMutex
	cancel    context.CancelFunc
	stopped   bool
	result    RunResult
	updatedAt int64
}

func NewService() *Service {
	return NewServiceWithPath(defaultPath(), nil)
}

func NewServiceWithPath(path string, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{}
	}
	service := &Service{
		path:           path,
		client:         client,
		defaultTimeout: defaultTimeoutSeconds * time.Second,
		running:        map[string]*runningRun{},
	}
	service.load()
	return service
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *Service) UpsertCollection(next Collection) Status {
	s.mu.Lock()
	next = normalizeCollection(next)
	now := time.Now().Unix()
	next.UpdatedAt = now
	replaced := false
	for index := range s.collections {
		if s.collections[index].ID == next.ID {
			s.collections[index] = next
			replaced = true
			break
		}
	}
	if !replaced {
		s.collections = append(s.collections, next)
	}
	if s.activeID == "" {
		s.activeID = next.ID
	}
	s.ensureActiveLocked()
	s.saveLockedWithStatus()
	status := s.statusLocked()
	s.mu.Unlock()
	if status.LastSaveError == "" {
		if err := s.syncCollectionToGit(next.ID); err != nil {
			s.mu.Lock()
			s.lastSaveError = err.Error()
			status = s.statusLocked()
			s.mu.Unlock()
		}
	}
	return status
}

func (s *Service) NewCollection() Status {
	collection := normalizeCollection(Collection{
		Name: "新 API 集合",
		Variables: []Variable{
			{ID: newID("var"), Name: "baseUrl", Value: "https://httpbin.org", Enabled: true},
		},
		Environments: []Environment{
			{ID: newID("env"), Name: "默认环境", Variables: []Variable{}, UpdatedAt: time.Now().Unix()},
		},
		Requests: []Request{
			{
				ID:       newID("req"),
				Name:     "GET 示例",
				Folder:   "示例",
				Method:   "GET",
				URL:      "{{baseUrl}}/get",
				BodyType: "none",
				Params:   []Param{},
				Headers:  []Header{{ID: newID("hdr"), Name: "Accept", Value: "application/json", Enabled: true}},
				Assertions: []Assertion{
					{ID: newID("ast"), Kind: "status", Operator: "equals", Expected: "200", Enabled: true},
				},
				UpdatedAt: time.Now().Unix(),
			},
		},
	})
	collection.ActiveRequestID = collection.Requests[0].ID
	collection.ActiveEnvironmentID = collection.Environments[0].ID
	s.mu.Lock()
	defer s.mu.Unlock()
	s.collections = append(s.collections, collection)
	s.activeID = collection.ID
	s.ensureActiveLocked()
	s.saveLockedWithStatus()
	return s.statusLocked()
}

func (s *Service) RemoveCollection(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		return s.statusLocked()
	}
	next := make([]Collection, 0, len(s.collections))
	for _, collection := range s.collections {
		if collection.ID != id {
			next = append(next, collection)
		}
	}
	s.collections = next
	if s.activeID == id {
		s.activeID = ""
	}
	s.ensureActiveLocked()
	s.saveLockedWithStatus()
	return s.statusLocked()
}

func (s *Service) SetActiveCollection(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, collection := range s.collections {
		if collection.ID == id {
			s.activeID = id
			break
		}
	}
	s.ensureActiveLocked()
	s.saveLockedWithStatus()
	return s.statusLocked()
}

func (s *Service) ImportRequests(path string, collectionID string) ImportResult {
	path = strings.TrimSpace(path)
	if path == "" {
		return ImportResult{OK: false, Message: "请选择要导入的文件", Status: s.Status()}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ImportResult{OK: false, Message: "导入失败", Error: err.Error(), Status: s.Status()}
	}
	requests, err := decodeImportedRequests(raw)
	if err != nil {
		return ImportResult{OK: false, Message: "导入失败", Error: err.Error(), Status: s.Status()}
	}
	if len(requests) == 0 {
		return ImportResult{OK: false, Message: "没有可导入的请求", Status: s.Status()}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureActiveLocked()
	targetIndex := -1
	collectionID = strings.TrimSpace(collectionID)
	for index := range s.collections {
		if collectionID != "" && s.collections[index].ID == collectionID {
			targetIndex = index
			break
		}
		if targetIndex < 0 && s.collections[index].ID == s.activeID {
			targetIndex = index
		}
	}
	if targetIndex < 0 {
		targetIndex = 0
	}

	now := time.Now().Unix()
	for index := range requests {
		request := normalizeRequest(reidentifyRequest(requests[index], now))
		s.collections[targetIndex].Requests = append(s.collections[targetIndex].Requests, request)
		s.collections[targetIndex].ActiveRequestID = request.ID
	}
	s.collections[targetIndex].UpdatedAt = now
	s.activeID = s.collections[targetIndex].ID
	s.saveLockedWithStatus()
	status := s.statusLocked()
	return ImportResult{
		OK:            s.lastSaveError == "",
		Message:       fmt.Sprintf("已导入 %d 个请求", len(requests)),
		ImportedCount: len(requests),
		Error:         s.lastSaveError,
		Status:        status,
	}
}

func (s *Service) StopRun(runID string) RunStopResult {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RunStopResult{OK: false, Message: "没有正在运行的请求"}
	}
	s.runningMu.Lock()
	run := s.running[runID]
	s.runningMu.Unlock()
	if run == nil {
		return RunStopResult{OK: false, Message: "请求已结束"}
	}
	run.stop()
	return RunStopResult{OK: true, Message: "正在停止请求"}
}

func (s *Service) RunSnapshot(runID string) RunSnapshot {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RunSnapshot{OK: false, Running: false, Message: "没有正在运行的请求"}
	}
	s.runningMu.Lock()
	run := s.running[runID]
	s.runningMu.Unlock()
	if run == nil {
		return RunSnapshot{OK: false, Running: false, Message: "请求已结束"}
	}
	return run.snapshot()
}

func (s *Service) Run(request RunRequest) RunResult {
	normalized := normalizeRequest(request.Request)
	method := normalized.Method
	if method == "" {
		method = http.MethodGet
	}

	variables := s.variablesForRun(request.CollectionID, request.EnvironmentID)
	resolvedURL, missingURL := resolveRequestURL(normalized.URL, normalized.Params, variables)
	resolvedBody, missingBody := applyVariables(normalized.Body, variables)
	missing := append(missingURL, missingBody...)

	headers := make([]Header, 0, len(normalized.Headers))
	for _, header := range normalized.Headers {
		if !header.Enabled || strings.TrimSpace(header.Name) == "" {
			continue
		}
		resolvedName, missingName := applyVariables(header.Name, variables)
		resolvedValue, missingValue := applyVariables(header.Value, variables)
		missing = append(missing, missingName...)
		missing = append(missing, missingValue...)
		headers = append(headers, Header{ID: header.ID, Name: resolvedName, Value: resolvedValue, Enabled: true})
	}
	missing = uniqueStrings(missing)

	streamHint := request.Stream || requestLooksLikeEventStream(resolvedURL, headers)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runID := strings.TrimSpace(request.RunID)
	var running *runningRun
	if runID != "" {
		running = s.registerRun(runID, cancel)
		defer s.unregisterRun(runID, running)
	}
	var timeoutTimer *time.Timer
	if !streamHint {
		timeoutTimer = time.AfterFunc(timeoutDuration(request.TimeoutSeconds, s.defaultTimeout), cancel)
		defer timeoutTimer.Stop()
	}

	var bodyReader io.Reader
	if resolvedBody != "" && method != http.MethodGet && method != http.MethodHead {
		bodyReader = strings.NewReader(resolvedBody)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, resolvedURL, bodyReader)
	if err != nil {
		return RunResult{
			OK:               false,
			Message:          "请求创建失败",
			Method:           method,
			RequestURL:       resolvedURL,
			Error:            err.Error(),
			MissingVariables: missing,
		}
	}
	for _, header := range headers {
		httpRequest.Header.Set(header.Name, header.Value)
	}
	if resolvedBody != "" && normalized.BodyType == "json" && httpRequest.Header.Get("Content-Type") == "" {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	client := s.httpClientForRun()
	response, err := client.Do(httpRequest)
	duration := time.Since(start)
	if err != nil {
		message := "请求失败"
		if running != nil && running.wasStopped() {
			message = "请求已停止"
		}
		return RunResult{
			OK:               false,
			Message:          message,
			Method:           method,
			RequestURL:       resolvedURL,
			DurationMs:       duration.Milliseconds(),
			Error:            err.Error(),
			MissingVariables: missing,
		}
	}
	defer response.Body.Close()

	contentType := response.Header.Get("Content-Type")
	streaming := isEventStreamResponse(contentType)
	if streaming && timeoutTimer != nil {
		timeoutTimer.Stop()
	}
	result := RunResult{
		OK:               true,
		Message:          "正在接收响应",
		Method:           method,
		RequestURL:       resolvedURL,
		StatusCode:       response.StatusCode,
		StatusText:       response.Status,
		DurationMs:       duration.Milliseconds(),
		Headers:          responseHeaders(response.Header),
		Body:             "",
		BodySize:         0,
		BodyTruncated:    false,
		ContentType:      contentType,
		Streaming:        streaming,
		MissingVariables: missing,
	}
	if streaming && running != nil {
		running.updateResult(result)
	}
	var body []byte
	var truncated bool
	var readErr error
	if streaming {
		body, truncated, readErr = readEventStreamBody(ctx, response.Body, func(partial []byte, partialTruncated bool) {
			if running == nil {
				return
			}
			next := result
			next.DurationMs = time.Since(start).Milliseconds()
			next.Body = responseText(partial, contentType)
			next.BodySize = len(partial)
			next.BodyTruncated = partialTruncated
			next.Message = "正在接收响应"
			running.updateResult(next)
		})
	} else {
		body, truncated, readErr = readResponseBody(response.Body)
	}
	result.OK = readErr == nil
	result.DurationMs = time.Since(start).Milliseconds()
	result.Body = responseText(body, contentType)
	result.BodySize = len(body)
	result.BodyTruncated = truncated
	if readErr != nil {
		result.Message = "响应读取失败"
		result.Error = readErr.Error()
		return result
	}
	result.AssertionResults = evaluateAssertions(normalized.Assertions, result)
	result.Passed, result.Failed = countAssertionResults(result.AssertionResults)
	if streaming {
		if running != nil && running.wasStopped() {
			result.Message = "SSE 已停止"
		} else {
			result.Message = "SSE 已结束"
		}
	} else if result.Failed > 0 {
		result.Message = fmt.Sprintf("请求完成，断言 %d/%d 通过", result.Passed, len(result.AssertionResults))
	} else if len(result.AssertionResults) > 0 {
		result.Message = "请求完成，断言全部通过"
	} else {
		result.Message = "请求完成"
	}
	return result
}

func (s *Service) load() {
	loaded, ok, err := loadStateFromSQLite(s.path)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastLoadError = err.Error()
	}
	if ok {
		s.collections = normalizeCollections(loaded.Collections)
		s.activeID = strings.TrimSpace(loaded.ActiveCollectionID)
	} else {
		s.collections = defaultCollections()
		s.activeID = s.collections[0].ID
		if err := saveStateToSQLite(s.path, s.stateLocked()); err != nil {
			s.lastSaveError = err.Error()
		}
	}
	s.ensureActiveLocked()
}

func (s *Service) statusLocked() Status {
	collections := cloneCollections(s.collections)
	requestCount := 0
	for _, collection := range collections {
		requestCount += len(collection.Requests)
	}
	return Status{
		Path:               s.path,
		DatabasePath:       databasePath(s.path),
		Collections:        collections,
		ActiveCollectionID: s.activeID,
		CollectionCount:    len(collections),
		RequestCount:       requestCount,
		LastSaveError:      s.lastSaveError,
		LastLoadError:      s.lastLoadError,
	}
}

func (s *Service) saveLockedWithStatus() {
	s.lastSaveError = ""
	if err := saveStateToSQLite(s.path, s.stateLocked()); err != nil {
		s.lastSaveError = err.Error()
	}
}

func (s *Service) stateLocked() state {
	return state{
		Version:            1,
		ActiveCollectionID: s.activeID,
		Collections:        cloneCollections(s.collections),
	}
}

func (s *Service) ensureActiveLocked() {
	s.collections = normalizeCollections(s.collections)
	if len(s.collections) == 0 {
		s.collections = defaultCollections()
	}
	if s.activeID == "" || !collectionExists(s.collections, s.activeID) {
		s.activeID = s.collections[0].ID
	}
}

func (s *Service) variablesForRun(collectionID string, environmentID string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var target Collection
	for _, collection := range s.collections {
		if collection.ID == collectionID {
			target = collection
			break
		}
	}
	if target.ID == "" && len(s.collections) > 0 {
		target = s.collections[0]
	}
	variables := map[string]string{}
	for _, variable := range target.Variables {
		if variable.Enabled && strings.TrimSpace(variable.Name) != "" {
			variables[strings.TrimSpace(variable.Name)] = variable.Value
		}
	}
	if environmentID == "" {
		environmentID = target.ActiveEnvironmentID
	}
	for _, environment := range target.Environments {
		if environment.ID != environmentID {
			continue
		}
		for _, variable := range environment.Variables {
			if variable.Enabled && strings.TrimSpace(variable.Name) != "" {
				variables[strings.TrimSpace(variable.Name)] = variable.Value
			}
		}
		break
	}
	return variables
}

func (s *Service) registerRun(runID string, cancel context.CancelFunc) *runningRun {
	run := &runningRun{cancel: cancel}
	s.runningMu.Lock()
	if s.running == nil {
		s.running = map[string]*runningRun{}
	}
	previous := s.running[runID]
	s.running[runID] = run
	s.runningMu.Unlock()
	if previous != nil {
		previous.stop()
	}
	return run
}

func (s *Service) unregisterRun(runID string, run *runningRun) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	if s.running[runID] == run {
		delete(s.running, runID)
	}
}

func (run *runningRun) stop() {
	run.mu.Lock()
	run.stopped = true
	cancel := run.cancel
	run.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (run *runningRun) wasStopped() bool {
	run.mu.RLock()
	defer run.mu.RUnlock()
	return run.stopped
}

func (run *runningRun) updateResult(result RunResult) {
	run.mu.Lock()
	run.result = result
	run.updatedAt = time.Now().UnixMilli()
	run.mu.Unlock()
}

func (run *runningRun) snapshot() RunSnapshot {
	run.mu.RLock()
	defer run.mu.RUnlock()
	if run.result.RequestURL == "" && run.result.Method == "" {
		return RunSnapshot{OK: true, Running: true, Message: "请求中", UpdatedAt: run.updatedAt}
	}
	return RunSnapshot{
		OK:        true,
		Running:   true,
		Message:   run.result.Message,
		UpdatedAt: run.updatedAt,
		Result:    run.result,
	}
}

func (s *Service) httpClientForRun() *http.Client {
	client := *s.client
	client.Timeout = 0
	return &client
}

func timeoutDuration(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	if seconds > maxTimeoutSeconds {
		seconds = maxTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func defaultCollections() []Collection {
	now := time.Now().Unix()
	envID := newID("env")
	reqID := newID("req")
	collection := normalizeCollection(Collection{
		ID:   newID("col"),
		Name: "默认集合",
		Variables: []Variable{
			{ID: newID("var"), Name: "baseUrl", Value: "https://httpbin.org", Enabled: true},
		},
		Environments: []Environment{
			{
				ID:        envID,
				Name:      "默认环境",
				Variables: []Variable{{ID: newID("var"), Name: "traceId", Value: "ariadne-{{$timestamp}}", Enabled: false}},
				UpdatedAt: now,
			},
		},
		Requests: []Request{
			{
				ID:       reqID,
				Name:     "GET 示例",
				Folder:   "示例",
				Method:   http.MethodGet,
				URL:      "{{baseUrl}}/get",
				BodyType: "none",
				Params:   []Param{},
				Headers: []Header{
					{ID: newID("hdr"), Name: "Accept", Value: "application/json", Enabled: true},
				},
				Assertions: []Assertion{
					{ID: newID("ast"), Kind: "status", Operator: "equals", Expected: "200", Enabled: true},
					{ID: newID("ast"), Kind: "json", Target: "url", Operator: "contains", Expected: "/get", Enabled: true},
				},
				UpdatedAt: now,
			},
		},
		ActiveEnvironmentID: envID,
		ActiveRequestID:     reqID,
		UpdatedAt:           now,
	})
	return []Collection{collection}
}

type importRequestSet struct {
	Requests []Request `json:"requests"`
}

type postmanCollection struct {
	Info struct {
		Name string `json:"name"`
	} `json:"info"`
	Item []postmanItem `json:"item"`
}

type postmanItem struct {
	Name    string          `json:"name"`
	Request *postmanRequest `json:"request"`
	Item    []postmanItem   `json:"item"`
}

type postmanRequest struct {
	Method string          `json:"method"`
	Header []postmanHeader `json:"header"`
	URL    interface{}     `json:"url"`
	Body   *postmanBody    `json:"body"`
}

type postmanHeader struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

type postmanBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

func decodeImportedRequests(raw []byte) ([]Request, error) {
	var collection Collection
	if err := json.Unmarshal(raw, &collection); err == nil && len(collection.Requests) > 0 {
		return collection.Requests, nil
	}

	var state state
	if err := json.Unmarshal(raw, &state); err == nil && len(state.Collections) > 0 {
		requests := []Request{}
		for _, collection := range state.Collections {
			for _, request := range collection.Requests {
				if strings.TrimSpace(request.Folder) == "" {
					request.Folder = collection.Name
				}
				requests = append(requests, request)
			}
		}
		if len(requests) > 0 {
			return requests, nil
		}
	}

	var set importRequestSet
	if err := json.Unmarshal(raw, &set); err == nil && len(set.Requests) > 0 {
		return set.Requests, nil
	}

	var requests []Request
	if err := json.Unmarshal(raw, &requests); err == nil && len(requests) > 0 {
		return requests, nil
	}

	var postman postmanCollection
	if err := json.Unmarshal(raw, &postman); err == nil && len(postman.Item) > 0 {
		requests := postmanRequests(postman.Item, "")
		if len(requests) > 0 {
			return requests, nil
		}
	}

	return nil, errors.New("未识别到可导入的请求")
}

func postmanRequests(items []postmanItem, folder string) []Request {
	requests := []Request{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if item.Request != nil {
			requests = append(requests, postmanToRequest(name, folder, *item.Request))
			continue
		}
		childFolder := folder
		if name != "" {
			if childFolder != "" {
				childFolder += "/"
			}
			childFolder += name
		}
		requests = append(requests, postmanRequests(item.Item, childFolder)...)
	}
	return requests
}

func postmanToRequest(name string, folder string, request postmanRequest) Request {
	headers := make([]Header, 0, len(request.Header))
	for _, header := range request.Header {
		if strings.TrimSpace(header.Key) == "" {
			continue
		}
		headers = append(headers, Header{
			ID:      newID("hdr"),
			Name:    header.Key,
			Value:   header.Value,
			Enabled: !header.Disabled,
		})
	}

	bodyType := "none"
	body := ""
	if request.Body != nil {
		body = request.Body.Raw
		switch strings.ToLower(strings.TrimSpace(request.Body.Mode)) {
		case "raw":
			bodyType = "text"
			if looksLikeJSON(body) {
				bodyType = "json"
			}
		case "urlencoded", "formdata":
			bodyType = "form"
		}
	}

	return Request{
		ID:       newID("req"),
		Name:     firstNonEmpty(name, "导入请求"),
		Folder:   folder,
		Method:   request.Method,
		URL:      postmanURLString(request.URL),
		BodyType: bodyType,
		Body:     body,
		Params:   []Param{},
		Headers:  headers,
		Assertions: []Assertion{
			{ID: newID("ast"), Kind: "status", Operator: "equals", Expected: "200", Enabled: true},
		},
		UpdatedAt: time.Now().Unix(),
	}
}

func postmanURLString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]interface{}:
		if raw, ok := typed["raw"].(string); ok && strings.TrimSpace(raw) != "" {
			return raw
		}
		protocol, _ := typed["protocol"].(string)
		host := postmanStringSlice(typed["host"])
		path := postmanStringSlice(typed["path"])
		if len(host) == 0 {
			return ""
		}
		result := strings.Join(host, ".")
		if protocol != "" {
			result = protocol + "://" + result
		}
		if len(path) > 0 {
			result += "/" + strings.Join(path, "/")
		}
		return result
	default:
		return ""
	}
}

func postmanStringSlice(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return []string{text}
		}
		return nil
	}
	result := []string{}
	for _, item := range items {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, text)
		}
	}
	return result
}

func looksLikeJSON(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[")
}

func reidentifyRequest(request Request, now int64) Request {
	request.ID = newID("req")
	if request.UpdatedAt == 0 {
		request.UpdatedAt = now
	}
	for index := range request.Headers {
		request.Headers[index].ID = newID("hdr")
	}
	for index := range request.Params {
		request.Params[index].ID = newID("param")
	}
	for index := range request.Assertions {
		request.Assertions[index].ID = newID("ast")
	}
	return request
}

func normalizeCollections(collections []Collection) []Collection {
	next := make([]Collection, 0, len(collections))
	seen := map[string]bool{}
	for _, collection := range collections {
		collection = normalizeCollection(collection)
		if collection.ID == "" || seen[collection.ID] {
			continue
		}
		seen[collection.ID] = true
		next = append(next, collection)
	}
	return next
}

func normalizeCollection(collection Collection) Collection {
	if strings.TrimSpace(collection.ID) == "" {
		collection.ID = newID("col")
	}
	collection.ID = strings.TrimSpace(collection.ID)
	collection.Name = firstNonEmpty(collection.Name, "API 集合")
	collection.Variables = normalizeVariables(collection.Variables)
	collection.Environments = normalizeEnvironments(collection.Environments)
	collection.Requests = normalizeRequests(collection.Requests)
	if len(collection.Environments) == 0 {
		collection.Environments = []Environment{{ID: newID("env"), Name: "默认环境", Variables: []Variable{}, UpdatedAt: time.Now().Unix()}}
	}
	if len(collection.Requests) == 0 {
		collection.Requests = []Request{normalizeRequest(Request{Name: "新请求", Method: http.MethodGet, URL: "https://example.com", BodyType: "none"})}
	}
	if collection.ActiveEnvironmentID == "" || !environmentExists(collection.Environments, collection.ActiveEnvironmentID) {
		collection.ActiveEnvironmentID = collection.Environments[0].ID
	}
	if collection.ActiveRequestID == "" || !requestExists(collection.Requests, collection.ActiveRequestID) {
		collection.ActiveRequestID = collection.Requests[0].ID
	}
	if collection.UpdatedAt == 0 {
		collection.UpdatedAt = time.Now().Unix()
	}
	return collection
}

func normalizeRequests(requests []Request) []Request {
	next := make([]Request, 0, len(requests))
	seen := map[string]bool{}
	for _, request := range requests {
		request = normalizeRequest(request)
		if request.ID == "" || seen[request.ID] {
			continue
		}
		seen[request.ID] = true
		next = append(next, request)
	}
	return next
}

func normalizeRequest(request Request) Request {
	if strings.TrimSpace(request.ID) == "" {
		request.ID = newID("req")
	}
	request.ID = strings.TrimSpace(request.ID)
	request.Name = firstNonEmpty(request.Name, "新请求")
	request.Folder = strings.TrimSpace(request.Folder)
	request.Method = normalizeMethod(request.Method)
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		request.URL = "https://example.com"
	}
	request.BodyType = normalizeBodyType(request.BodyType)
	request.Params = normalizeParams(request.Params)
	request.Headers = normalizeHeaders(request.Headers)
	request.Assertions = normalizeAssertions(request.Assertions)
	if request.UpdatedAt == 0 {
		request.UpdatedAt = time.Now().Unix()
	}
	return request
}

func normalizeEnvironments(environments []Environment) []Environment {
	next := make([]Environment, 0, len(environments))
	seen := map[string]bool{}
	for _, environment := range environments {
		if strings.TrimSpace(environment.ID) == "" {
			environment.ID = newID("env")
		}
		environment.ID = strings.TrimSpace(environment.ID)
		if seen[environment.ID] {
			continue
		}
		seen[environment.ID] = true
		environment.Name = firstNonEmpty(environment.Name, "环境")
		environment.Variables = normalizeVariables(environment.Variables)
		if environment.UpdatedAt == 0 {
			environment.UpdatedAt = time.Now().Unix()
		}
		next = append(next, environment)
	}
	return next
}

func normalizeVariables(variables []Variable) []Variable {
	next := make([]Variable, 0, len(variables))
	for _, variable := range variables {
		if strings.TrimSpace(variable.ID) == "" {
			variable.ID = newID("var")
			variable.Enabled = true
		}
		variable.ID = strings.TrimSpace(variable.ID)
		variable.Name = strings.TrimSpace(variable.Name)
		next = append(next, variable)
	}
	return next
}

func normalizeHeaders(headers []Header) []Header {
	next := make([]Header, 0, len(headers))
	for _, header := range headers {
		if strings.TrimSpace(header.ID) == "" {
			header.ID = newID("hdr")
			header.Enabled = true
		}
		header.ID = strings.TrimSpace(header.ID)
		header.Name = strings.TrimSpace(header.Name)
		next = append(next, header)
	}
	return next
}

func normalizeParams(params []Param) []Param {
	next := make([]Param, 0, len(params))
	for _, param := range params {
		if strings.TrimSpace(param.ID) == "" {
			param.ID = newID("param")
			param.Enabled = true
		}
		param.ID = strings.TrimSpace(param.ID)
		param.Name = strings.TrimSpace(param.Name)
		param.Type = normalizeParamType(param.Type)
		next = append(next, param)
	}
	return next
}

func normalizeAssertions(assertions []Assertion) []Assertion {
	next := make([]Assertion, 0, len(assertions))
	for _, assertion := range assertions {
		if strings.TrimSpace(assertion.ID) == "" {
			assertion.ID = newID("ast")
			assertion.Enabled = true
		}
		assertion.ID = strings.TrimSpace(assertion.ID)
		assertion.Kind = normalizeAssertionKind(assertion.Kind)
		assertion.Operator = normalizeOperator(assertion.Operator)
		assertion.Target = strings.TrimSpace(assertion.Target)
		next = append(next, assertion)
	}
	return next
}

func normalizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return method
	default:
		if isHTTPToken(method) {
			return method
		}
		return http.MethodGet
	}
}

func isHTTPToken(method string) bool {
	if method == "" {
		return false
	}
	for _, r := range method {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", r) {
			continue
		}
		return false
	}
	return true
}

func normalizeBodyType(bodyType string) string {
	switch strings.ToLower(strings.TrimSpace(bodyType)) {
	case "json", "text", "form", "none":
		return strings.ToLower(strings.TrimSpace(bodyType))
	default:
		return "none"
	}
}

func normalizeParamType(paramType string) string {
	switch strings.ToLower(strings.TrimSpace(paramType)) {
	case "path":
		return "path"
	default:
		return "query"
	}
}

func normalizeAssertionKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "status", "header", "body", "json", "response_time":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return "status"
	}
}

func normalizeOperator(operator string) string {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "equals", "not_equals", "contains", "exists", "less_than", "greater_than":
		return strings.ToLower(strings.TrimSpace(operator))
	default:
		return "equals"
	}
}

func applyVariables(text string, variables map[string]string) (string, []string) {
	missing := []string{}
	result := templateTokenPattern.ReplaceAllStringFunc(text, func(match string) string {
		groups := templateTokenPattern.FindStringSubmatch(match)
		if len(groups) != 2 {
			return match
		}
		name := groups[1]
		if name == "$timestamp" {
			return strconv.FormatInt(time.Now().Unix(), 10)
		}
		if value, ok := variables[name]; ok {
			return value
		}
		missing = append(missing, name)
		return match
	})
	return result, missing
}

func resolveRequestURL(rawURL string, params []Param, variables map[string]string) (string, []string) {
	resolvedURL, missing := applyVariables(rawURL, variables)
	for _, param := range params {
		if !param.Enabled || param.Type != "path" || strings.TrimSpace(param.Name) == "" {
			continue
		}
		name, missingName := applyVariables(param.Name, variables)
		value, missingValue := applyVariables(param.Value, variables)
		missing = append(missing, missingName...)
		missing = append(missing, missingValue...)
		if strings.TrimSpace(name) == "" {
			continue
		}
		escapedValue := url.PathEscape(value)
		resolvedURL = strings.ReplaceAll(resolvedURL, ":"+name, escapedValue)
		resolvedURL = strings.ReplaceAll(resolvedURL, "{"+name+"}", escapedValue)
	}

	parsed, err := url.Parse(resolvedURL)
	if err != nil {
		for _, param := range params {
			if param.Enabled && param.Type == "query" && strings.TrimSpace(param.Name) != "" {
				_, missingName := applyVariables(param.Name, variables)
				_, missingValue := applyVariables(param.Value, variables)
				missing = append(missing, missingName...)
				missing = append(missing, missingValue...)
			}
		}
		return resolvedURL, missing
	}

	query := parsed.Query()
	for _, param := range params {
		if !param.Enabled || param.Type != "query" || strings.TrimSpace(param.Name) == "" {
			continue
		}
		name, missingName := applyVariables(param.Name, variables)
		value, missingValue := applyVariables(param.Value, variables)
		missing = append(missing, missingName...)
		missing = append(missing, missingValue...)
		if strings.TrimSpace(name) == "" {
			continue
		}
		query.Add(name, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), missing
}

func isEventStreamResponse(contentType string) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return mediaType == "text/event-stream" || strings.Contains(strings.ToLower(contentType), "event-stream")
}

func requestLooksLikeEventStream(resolvedURL string, headers []Header) bool {
	if eventStreamURLPattern.MatchString(resolvedURL) {
		return true
	}
	for _, header := range headers {
		name := strings.ToLower(strings.TrimSpace(header.Name))
		value := strings.ToLower(strings.TrimSpace(header.Value))
		if name == "accept" && strings.Contains(value, "event-stream") {
			return true
		}
	}
	return false
}

func readEventStreamBody(ctx context.Context, body io.ReadCloser, onUpdate func([]byte, bool)) ([]byte, bool, error) {
	var buffer bytes.Buffer
	reader := bufio.NewReader(body)
	truncated := false
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			changed := false
			if buffer.Len() < maxEventStreamBodyBytes {
				remaining := maxEventStreamBodyBytes - buffer.Len()
				if len(line) > remaining {
					buffer.WriteString(line[:remaining])
					truncated = true
					changed = true
				} else {
					buffer.WriteString(line)
					changed = true
				}
			} else {
				truncated = true
			}
			if changed && onUpdate != nil {
				onUpdate(buffer.Bytes(), truncated)
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return eventStreamBytes(buffer), truncated, nil
		}
		if ctx.Err() != nil {
			if buffer.Len() > 0 {
				return buffer.Bytes(), truncated, nil
			}
			return []byte("SSE 连接已建立，暂未收到事件"), false, nil
		}
		return eventStreamBytes(buffer), truncated, err
	}
}

func eventStreamBytes(buffer bytes.Buffer) []byte {
	if buffer.Len() == 0 {
		return []byte("SSE 连接已建立，暂未收到事件")
	}
	return buffer.Bytes()
}

func readResponseBody(reader io.Reader) ([]byte, bool, error) {
	limited := io.LimitReader(reader, maxResponseBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if len(body) > maxResponseBodyBytes {
		return body[:maxResponseBodyBytes], true, nil
	}
	return body, false, nil
}

func responseText(body []byte, contentType string) string {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(mediaType, "text/"),
		strings.Contains(mediaType, "json"),
		strings.Contains(mediaType, "xml"),
		mediaType == "":
		return strings.ToValidUTF8(string(body), "")
	default:
		return fmt.Sprintf("<%d bytes binary response>", len(body))
	}
}

func responseHeaders(headers http.Header) []Header {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]Header, 0, len(names))
	for _, name := range names {
		result = append(result, Header{
			ID:      newID("hdr"),
			Name:    name,
			Value:   strings.Join(headers.Values(name), ", "),
			Enabled: true,
		})
	}
	return result
}

func evaluateAssertions(assertions []Assertion, result RunResult) []AssertionResult {
	results := []AssertionResult{}
	for _, assertion := range assertions {
		if !assertion.Enabled {
			continue
		}
		results = append(results, evaluateAssertion(assertion, result))
	}
	return results
}

func evaluateAssertion(assertion Assertion, result RunResult) AssertionResult {
	assertion = normalizeAssertions([]Assertion{assertion})[0]
	actual, exists, err := assertionActual(assertion, result)
	passed := false
	message := ""
	if err != nil {
		message = err.Error()
	} else {
		passed, message = compareActual(assertion, actual, exists)
	}
	return AssertionResult{
		ID:       assertion.ID,
		Kind:     assertion.Kind,
		Target:   assertion.Target,
		Operator: assertion.Operator,
		Expected: assertion.Expected,
		Actual:   actual,
		Passed:   passed,
		Message:  message,
	}
}

func assertionActual(assertion Assertion, result RunResult) (string, bool, error) {
	switch assertion.Kind {
	case "status":
		return strconv.Itoa(result.StatusCode), true, nil
	case "response_time":
		return strconv.FormatInt(result.DurationMs, 10), true, nil
	case "header":
		name := strings.TrimSpace(assertion.Target)
		if name == "" {
			return "", false, errors.New("响应头断言需要名称")
		}
		for _, header := range result.Headers {
			if strings.EqualFold(header.Name, name) {
				return header.Value, true, nil
			}
		}
		return "", false, nil
	case "body":
		return result.Body, result.Body != "", nil
	case "json":
		if strings.TrimSpace(result.Body) == "" {
			return "", false, errors.New("响应体不是 JSON")
		}
		var parsed interface{}
		decoder := json.NewDecoder(strings.NewReader(result.Body))
		decoder.UseNumber()
		if err := decoder.Decode(&parsed); err != nil {
			return "", false, fmt.Errorf("响应体不是有效 JSON: %w", err)
		}
		value, ok := jsonPathValue(parsed, assertion.Target)
		if !ok {
			return "", false, nil
		}
		return stringifyValue(value), true, nil
	default:
		return "", false, errors.New("未知断言类型")
	}
}

func compareActual(assertion Assertion, actual string, exists bool) (bool, string) {
	switch assertion.Operator {
	case "exists":
		if exists {
			return true, "存在"
		}
		return false, "不存在"
	case "equals":
		if assertion.Kind == "json" && semanticJSONEqual(actual, assertion.Expected) {
			return true, "相等"
		}
		if actual == assertion.Expected {
			return true, "相等"
		}
		return false, "不相等"
	case "not_equals":
		if actual != assertion.Expected {
			return true, "不相等"
		}
		return false, "相等"
	case "contains":
		if strings.Contains(actual, assertion.Expected) {
			return true, "已包含"
		}
		return false, "未包含"
	case "less_than":
		return compareNumber(actual, assertion.Expected, func(left, right float64) bool { return left < right }, "小于")
	case "greater_than":
		return compareNumber(actual, assertion.Expected, func(left, right float64) bool { return left > right }, "大于")
	default:
		return false, "未知断言操作"
	}
}

func compareNumber(actual string, expected string, compare func(float64, float64) bool, label string) (bool, string) {
	left, leftErr := strconv.ParseFloat(strings.TrimSpace(actual), 64)
	right, rightErr := strconv.ParseFloat(strings.TrimSpace(expected), 64)
	if leftErr != nil || rightErr != nil {
		return false, "无法按数字比较"
	}
	if compare(left, right) {
		return true, label
	}
	return false, "数字比较未通过"
}

func semanticJSONEqual(actual string, expected string) bool {
	var left interface{}
	var right interface{}
	leftDecoder := json.NewDecoder(strings.NewReader(actual))
	leftDecoder.UseNumber()
	rightDecoder := json.NewDecoder(strings.NewReader(expected))
	rightDecoder.UseNumber()
	if leftDecoder.Decode(&left) != nil || rightDecoder.Decode(&right) != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}

func jsonPathValue(value interface{}, path string) (interface{}, bool) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return value, true
	}
	current := value
	for _, rawSegment := range strings.Split(path, ".") {
		if rawSegment == "" {
			return nil, false
		}
		name, indexes, ok := parsePathSegment(rawSegment)
		if !ok {
			return nil, false
		}
		if name != "" {
			object, ok := current.(map[string]interface{})
			if !ok {
				return nil, false
			}
			current, ok = object[name]
			if !ok {
				return nil, false
			}
		}
		for _, index := range indexes {
			array, ok := current.([]interface{})
			if !ok || index < 0 || index >= len(array) {
				return nil, false
			}
			current = array[index]
		}
	}
	return current, true
}

func parsePathSegment(segment string) (string, []int, bool) {
	nameEnd := strings.Index(segment, "[")
	name := segment
	if nameEnd >= 0 {
		name = segment[:nameEnd]
	}
	remaining := segment[len(name):]
	indexes := []int{}
	for remaining != "" {
		if !strings.HasPrefix(remaining, "[") {
			return "", nil, false
		}
		closeAt := strings.Index(remaining, "]")
		if closeAt <= 1 {
			return "", nil, false
		}
		index, err := strconv.Atoi(remaining[1:closeAt])
		if err != nil {
			return "", nil, false
		}
		indexes = append(indexes, index)
		remaining = remaining[closeAt+1:]
	}
	return name, indexes, true
}

func stringifyValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	default:
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			return fmt.Sprint(value)
		}
		return strings.TrimSpace(buffer.String())
	}
}

func countAssertionResults(results []AssertionResult) (int, int) {
	passed := 0
	failed := 0
	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
		}
	}
	return passed, failed
}

func cloneCollections(collections []Collection) []Collection {
	raw, err := json.Marshal(collections)
	if err != nil {
		return []Collection{}
	}
	var cloned []Collection
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return []Collection{}
	}
	return cloned
}

func collectionExists(collections []Collection, id string) bool {
	for _, collection := range collections {
		if collection.ID == id {
			return true
		}
	}
	return false
}

func environmentExists(environments []Environment, id string) bool {
	for _, environment := range environments {
		if environment.ID == id {
			return true
		}
	}
	return false
}

func requestExists(requests []Request, id string) bool {
	for _, request := range requests {
		if request.ID == id {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	next := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		next = append(next, value)
	}
	sort.Strings(next)
	return next
}

func newID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}

func defaultPath() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "Ariadne", "api_testing.json")
	}
	return filepath.Join(".", "api_testing.json")
}
