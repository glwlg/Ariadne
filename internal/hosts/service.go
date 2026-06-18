package hosts

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ariadne/internal/appdb"
)

const (
	systemProfileID = "system-hosts"
	localProfile    = "local"
	remoteProfile   = "remote"

	ariadneStartMarker = "# ================= ARIADNE HOSTS START ================="
	ariadneEndMarker   = "# ================= ARIADNE HOSTS END ================="
	legacyStartMarker  = "# ================= X-TOOLS HOSTS START ================="
	legacyEndMarker    = "# ================= X-TOOLS HOSTS END ================="
)

type Profile struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Enabled   bool   `json:"enabled"`
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	System    bool   `json:"system"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

type Conflict struct {
	Host string   `json:"host"`
	IPs  []string `json:"ips"`
}

type ApplyPreview struct {
	HostsPath        string     `json:"hostsPath"`
	LineCount        int        `json:"lineCount"`
	AddedLines       int        `json:"addedLines"`
	RemovedLines     int        `json:"removedLines"`
	Changed          bool       `json:"changed"`
	EnabledProfiles  []string   `json:"enabledProfiles"`
	Conflicts        []Conflict `json:"conflicts"`
	CurrentContent   string     `json:"currentContent"`
	FinalContent     string     `json:"finalContent"`
	DiffText         string     `json:"diffText"`
	RequiresConfirm  bool       `json:"requiresConfirm"`
	LastPreviewError string     `json:"lastPreviewError,omitempty"`
}

type ApplyResult struct {
	OK                  bool         `json:"ok"`
	Message             string       `json:"message"`
	RequiresConfirm     bool         `json:"requiresConfirm"`
	Preview             ApplyPreview `json:"preview"`
	LastApplyError      string       `json:"lastApplyError,omitempty"`
	ConfirmationCommand string       `json:"confirmationCommand,omitempty"`
}

type Status struct {
	ConfigPath        string    `json:"configPath"`
	HostsPath         string    `json:"hostsPath"`
	LegacyPath        string    `json:"legacyPath"`
	Count             int       `json:"count"`
	EnabledCount      int       `json:"enabledCount"`
	SystemReadable    bool      `json:"systemReadable"`
	SystemBytes       int64     `json:"systemBytes"`
	LastSaveError     string    `json:"lastSaveError,omitempty"`
	LastReadError     string    `json:"lastReadError,omitempty"`
	LastApplyError    string    `json:"lastApplyError,omitempty"`
	LastRemoteError   string    `json:"lastRemoteError,omitempty"`
	LegacyImported    bool      `json:"legacyImported"`
	Profiles          []Profile `json:"profiles"`
	VirtualizedPath   string    `json:"virtualizedPath,omitempty"`
	VirtualizedExists bool      `json:"virtualizedExists"`
	VirtualizedBytes  int64     `json:"virtualizedBytes"`
}

type Service struct {
	mu              sync.RWMutex
	configPath      string
	hostsPath       string
	legacyPath      string
	profiles        []Profile
	lastSaveError   string
	lastReadError   string
	lastApplyError  string
	lastRemoteError string
	legacyImported  bool
	httpClient      *http.Client
}

func NewService() *Service {
	return NewServiceWithPaths(defaultConfigPath(), defaultHostsPath(), defaultLegacyPath())
}

func NewServiceWithPaths(configPath string, hostsPath string, legacyPath string) *Service {
	service := &Service{
		configPath: configPath,
		hostsPath:  hostsPath,
		legacyPath: legacyPath,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	service.load()
	return service
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshSystemProfileLocked()
	return s.statusLocked()
}

func (s *Service) List() []Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshSystemProfileLocked()
	return append([]Profile{}, s.profiles...)
}

func (s *Service) Upsert(next Profile) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	next = normalizeProfile(next)
	if next.ID == "" {
		next.ID = "hosts-" + stableID(next.Title+"|"+strconvUnixNano())
	}
	if next.System || next.ID == systemProfileID || next.Title == "" {
		return s.statusLocked()
	}
	next.System = false
	next.UpdatedAt = time.Now().Unix()

	replaced := false
	for i := range s.profiles {
		if s.profiles[i].ID == next.ID && !s.profiles[i].System {
			s.profiles[i] = next
			replaced = true
			break
		}
	}
	if !replaced {
		s.profiles = append(s.profiles, next)
	}
	sortProfiles(s.profiles)
	s.saveLockedWithStatus()
	return s.statusLocked()
}

func (s *Service) NewProfile() Status {
	return s.Upsert(Profile{
		Title:   "New Hosts Profile",
		Content: "# Local Hosts\n127.0.0.1 example.local\n",
		Type:    localProfile,
		Enabled: false,
	})
}

func (s *Service) Remove(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" || id == systemProfileID {
		return s.statusLocked()
	}
	next := make([]Profile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		if profile.ID != id {
			next = append(next, profile)
		}
	}
	s.profiles = next
	s.saveLockedWithStatus()
	return s.statusLocked()
}

func (s *Service) SetEnabled(id string, enabled bool) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.profiles {
		if s.profiles[i].ID == id && !s.profiles[i].System {
			s.profiles[i].Enabled = enabled
			s.profiles[i].UpdatedAt = time.Now().Unix()
			break
		}
	}
	s.saveLockedWithStatus()
	return s.statusLocked()
}

func (s *Service) FetchRemote(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	var target Profile
	targetIndex := -1
	for i, profile := range s.profiles {
		if profile.ID == id && !profile.System {
			target = profile
			targetIndex = i
			break
		}
	}
	s.mu.Unlock()

	if targetIndex < 0 {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastRemoteError = "未找到 Hosts 方案"
		return s.statusLocked()
	}
	if target.Type != remoteProfile {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastRemoteError = "当前方案不是远程拉取模式"
		return s.statusLocked()
	}
	parsed, err := url.Parse(strings.TrimSpace(target.URL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastRemoteError = "远程 Hosts URL 只支持 http/https"
		return s.statusLocked()
	}
	content, err := s.fetchURL(parsed.String())
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastRemoteError = err.Error()
		return s.statusLocked()
	}
	for i := range s.profiles {
		if s.profiles[i].ID == id && !s.profiles[i].System {
			s.profiles[i].Content = strings.ToValidUTF8(content, "")
			s.profiles[i].UpdatedAt = time.Now().Unix()
			break
		}
	}
	s.lastRemoteError = ""
	s.saveLockedWithStatus()
	return s.statusLocked()
}

func (s *Service) PreviewApply() ApplyPreview {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshSystemProfileLocked()
	return s.applyPreviewLocked()
}

func (s *Service) ApplyEnabledProfiles(confirmed bool) ApplyResult {
	s.mu.Lock()
	s.refreshSystemProfileLocked()
	preview := s.applyPreviewLocked()
	if !confirmed {
		s.mu.Unlock()
		return ApplyResult{
			OK:              false,
			Message:         "需要确认后写入系统 Hosts",
			RequiresConfirm: true,
			Preview:         preview,
		}
	}
	finalContent := preview.FinalContent
	s.mu.Unlock()

	if err := writeHostsContent(s.hostsPath, finalContent); err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastApplyError = err.Error()
		return ApplyResult{OK: false, Message: "写入系统 Hosts 失败", Preview: preview, LastApplyError: s.lastApplyError}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastApplyError = ""
	s.refreshSystemProfileLocked()
	return ApplyResult{OK: true, Message: "已写入系统 Hosts", Preview: preview}
}

func (s *Service) fetchURL(rawURL string) (string, error) {
	resp, err := s.httpClient.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("远程 Hosts 返回状态 %s", resp.Status)
	}
	reader := io.LimitReader(resp.Body, 2*1024*1024)
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return strings.ToValidUTF8(string(data), ""), nil
}

func (s *Service) load() {
	loaded := []Profile{}
	if s.configPath != "" {
		if profiles, ok, err := loadProfilesFromSQLite(s.configPath); err == nil && ok {
			loaded = normalizeProfiles(profiles)
		}
	}
	if len(loaded) == 0 {
		imported, ok := s.loadLegacyProfiles()
		if ok {
			loaded = imported
			s.legacyImported = true
		}
	}
	s.profiles = ensureSystemProfile(loaded)
	s.refreshSystemProfileLocked()
	sortProfiles(s.profiles)
	if s.configPath != "" && len(loaded) > 0 {
		s.saveLockedWithStatus()
	}
}

func (s *Service) loadLegacyProfiles() ([]Profile, bool) {
	if s.legacyPath == "" {
		return nil, false
	}
	raw, err := os.ReadFile(s.legacyPath)
	if err != nil {
		return nil, false
	}
	var legacy map[string]interface{}
	if json.Unmarshal(raw, &legacy) != nil {
		return nil, false
	}
	profiles := []Profile{}
	for key, value := range legacy {
		if strings.TrimSpace(key) == "" || key == "系统 Hosts" {
			continue
		}
		profile := Profile{
			ID:      "legacy-" + stableID(key),
			Title:   key,
			Type:    localProfile,
			Enabled: false,
		}
		switch typed := value.(type) {
		case string:
			profile.Content = typed
		case map[string]interface{}:
			profile.Title = stringValue(typed["title"], key)
			profile.Content = stringValue(typed["content"], "")
			profile.Type = oneOf(stringValue(typed["type"], localProfile), localProfile, localProfile, remoteProfile)
			profile.URL = stringValue(typed["url"], "")
			profile.Enabled = boolValue(typed["enabled"], false)
		}
		profile = normalizeProfile(profile)
		if profile.Title != "" {
			profiles = append(profiles, profile)
		}
	}
	return profiles, len(profiles) > 0
}

func (s *Service) statusLocked() Status {
	systemReadable := false
	systemBytes := int64(0)
	if info, err := os.Stat(s.hostsPath); err == nil && !info.IsDir() {
		systemReadable = true
		systemBytes = info.Size()
	}
	storagePath := firstNonEmpty(appdb.DatabasePathForPath(s.configPath), s.configPath)
	virtualizedPath, virtualizedExists, virtualizedBytes := findVirtualizedFile(storagePath, os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"))
	status := Status{
		ConfigPath:        storagePath,
		HostsPath:         s.hostsPath,
		LegacyPath:        s.legacyPath,
		Count:             len(s.profiles),
		SystemReadable:    systemReadable,
		SystemBytes:       systemBytes,
		LastSaveError:     s.lastSaveError,
		LastReadError:     s.lastReadError,
		LastApplyError:    s.lastApplyError,
		LastRemoteError:   s.lastRemoteError,
		LegacyImported:    s.legacyImported,
		Profiles:          append([]Profile{}, s.profiles...),
		VirtualizedPath:   virtualizedPath,
		VirtualizedExists: virtualizedExists,
		VirtualizedBytes:  virtualizedBytes,
	}
	for _, profile := range s.profiles {
		if profile.Enabled && !profile.System {
			status.EnabledCount++
		}
	}
	return status
}

func (s *Service) saveLockedWithStatus() {
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
	}
}

func (s *Service) saveLocked() error {
	if s.configPath == "" {
		return nil
	}
	persisted := []Profile{}
	for _, profile := range s.profiles {
		if profile.System {
			continue
		}
		persisted = append(persisted, normalizeProfile(profile))
	}
	return saveProfilesToSQLite(s.configPath, persisted)
}

func (s *Service) refreshSystemProfileLocked() {
	content := ""
	s.lastReadError = ""
	if s.hostsPath != "" {
		raw, err := os.ReadFile(s.hostsPath)
		if err == nil {
			content = strings.ToValidUTF8(string(raw), "")
		} else {
			s.lastReadError = err.Error()
		}
	}
	system := Profile{
		ID:      systemProfileID,
		Title:   "系统 Hosts",
		Content: content,
		Type:    localProfile,
		System:  true,
	}
	found := false
	for i := range s.profiles {
		if s.profiles[i].System || s.profiles[i].ID == systemProfileID {
			s.profiles[i] = system
			found = true
			break
		}
	}
	if !found {
		s.profiles = append([]Profile{system}, s.profiles...)
	}
}

func (s *Service) applyPreviewLocked() ApplyPreview {
	current := systemContent(s.profiles)
	base := stripManagedBlocks(current)
	enabledProfiles := []Profile{}
	enabledTitles := []string{}
	for _, profile := range s.profiles {
		if profile.System || !profile.Enabled {
			continue
		}
		enabledProfiles = append(enabledProfiles, profile)
		enabledTitles = append(enabledTitles, profile.Title)
	}
	finalContent := buildFinalHosts(base, enabledProfiles)
	added, removed := lineDelta(current, finalContent)
	return ApplyPreview{
		HostsPath:       s.hostsPath,
		LineCount:       len(splitLines(finalContent)),
		AddedLines:      added,
		RemovedLines:    removed,
		Changed:         current != finalContent,
		EnabledProfiles: enabledTitles,
		Conflicts:       detectConflicts(finalContent),
		CurrentContent:  current,
		FinalContent:    finalContent,
		DiffText:        simpleDiff(current, finalContent, 240),
		RequiresConfirm: true,
	}
}

func buildFinalHosts(base string, enabled []Profile) string {
	finalContent := strings.TrimSpace(stripManagedBlocks(base))
	injected := []string{}
	for _, profile := range enabled {
		content := strings.TrimSpace(profile.Content)
		if content == "" {
			continue
		}
		injected = append(injected, "# --- Profile: "+profile.Title+" ---\n"+content)
	}
	if len(injected) == 0 {
		return normalizeHostsEnding(finalContent)
	}
	if finalContent != "" {
		finalContent += "\n\n"
	}
	finalContent += ariadneStartMarker + "\n" + strings.Join(injected, "\n\n") + "\n" + ariadneEndMarker
	return normalizeHostsEnding(finalContent)
}

func stripManagedBlocks(content string) string {
	content = stripMarkerBlock(content, ariadneStartMarker, ariadneEndMarker)
	content = stripMarkerBlock(content, legacyStartMarker, legacyEndMarker)
	return strings.TrimSpace(content)
}

func stripMarkerBlock(content string, start string, end string) string {
	for {
		startIdx := strings.Index(content, start)
		if startIdx < 0 {
			return content
		}
		endIdx := strings.Index(content[startIdx:], end)
		if endIdx < 0 {
			return content
		}
		endIdx = startIdx + endIdx + len(end)
		before := strings.TrimRight(content[:startIdx], "\r\n\t ")
		after := strings.TrimLeft(content[endIdx:], "\r\n\t ")
		if before != "" && after != "" {
			content = before + "\n\n" + after
		} else {
			content = before + after
		}
	}
}

func detectConflicts(content string) []Conflict {
	mapping := map[string]map[string]bool{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.SplitN(raw, "#", 2)[0])
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 || !looksLikeIP(parts[0]) {
			continue
		}
		ip := parts[0]
		for _, host := range parts[1:] {
			key := strings.ToLower(strings.TrimSpace(host))
			if key == "" {
				continue
			}
			if mapping[key] == nil {
				mapping[key] = map[string]bool{}
			}
			mapping[key][ip] = true
		}
	}
	conflicts := []Conflict{}
	for host, ipsMap := range mapping {
		if len(ipsMap) <= 1 {
			continue
		}
		ips := []string{}
		for ip := range ipsMap {
			ips = append(ips, ip)
		}
		sort.Strings(ips)
		conflicts = append(conflicts, Conflict{Host: host, IPs: ips})
	}
	sort.SliceStable(conflicts, func(i, j int) bool {
		return conflicts[i].Host < conflicts[j].Host
	})
	return conflicts
}

func simpleDiff(current string, final string, limit int) string {
	currentLines := splitLines(current)
	finalLines := splitLines(final)
	currentSet := countedLines(currentLines)
	finalSet := countedLines(finalLines)
	out := []string{}
	for _, line := range finalLines {
		if currentSet[line] > 0 {
			currentSet[line]--
			continue
		}
		out = append(out, "+ "+line)
		if len(out) >= limit {
			out = append(out, "... diff truncated")
			return strings.Join(out, "\n")
		}
	}
	for _, line := range currentLines {
		if finalSet[line] > 0 {
			finalSet[line]--
			continue
		}
		out = append(out, "- "+line)
		if len(out) >= limit {
			out = append(out, "... diff truncated")
			return strings.Join(out, "\n")
		}
	}
	if len(out) == 0 {
		return "(无差异)"
	}
	return strings.Join(out, "\n")
}

func lineDelta(current string, final string) (int, int) {
	currentLines := countedLines(splitLines(current))
	finalLines := countedLines(splitLines(final))
	added := 0
	removed := 0
	for line, count := range finalLines {
		if diff := count - currentLines[line]; diff > 0 {
			added += diff
		}
	}
	for line, count := range currentLines {
		if diff := count - finalLines[line]; diff > 0 {
			removed += diff
		}
	}
	return added, removed
}

func splitLines(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(strings.TrimRight(normalized, "\n"), "\n")
}

func countedLines(lines []string) map[string]int {
	counts := map[string]int{}
	for _, line := range lines {
		counts[line]++
	}
	return counts
}

func looksLikeIP(value string) bool {
	if regexp.MustCompile(`^\d{1,3}(?:\.\d{1,3}){3}$`).MatchString(value) {
		return true
	}
	return strings.Contains(value, ":") && regexp.MustCompile(`^[0-9a-fA-F:]+$`).MatchString(value)
}

func normalizeProfiles(profiles []Profile) []Profile {
	result := make([]Profile, 0, len(profiles))
	seen := map[string]bool{}
	for _, profile := range profiles {
		profile = normalizeProfile(profile)
		if profile.ID == "" || seen[profile.ID] {
			continue
		}
		seen[profile.ID] = true
		result = append(result, profile)
	}
	return ensureSystemProfile(result)
}

func normalizeProfile(profile Profile) Profile {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Title = strings.TrimSpace(profile.Title)
	profile.Type = oneOf(profile.Type, localProfile, localProfile, remoteProfile)
	profile.URL = strings.TrimSpace(profile.URL)
	profile.Content = strings.ReplaceAll(profile.Content, "\r\n", "\n")
	profile.Content = strings.ReplaceAll(profile.Content, "\r", "\n")
	if profile.ID == "" && profile.Title != "" {
		profile.ID = "hosts-" + stableID(profile.Title)
	}
	if profile.Title == "" && profile.ID != "" {
		profile.Title = profile.ID
	}
	return profile
}

func ensureSystemProfile(profiles []Profile) []Profile {
	hasSystem := false
	result := []Profile{}
	for _, profile := range profiles {
		if profile.System || profile.ID == systemProfileID {
			if hasSystem {
				continue
			}
			hasSystem = true
			profile.ID = systemProfileID
			profile.Title = "系统 Hosts"
			profile.System = true
			profile.Enabled = false
			profile.Type = localProfile
			result = append(result, profile)
			continue
		}
		result = append(result, profile)
	}
	if !hasSystem {
		result = append([]Profile{{ID: systemProfileID, Title: "系统 Hosts", Type: localProfile, System: true}}, result...)
	}
	return result
}

func sortProfiles(profiles []Profile) {
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].System != profiles[j].System {
			return profiles[i].System
		}
		return strings.ToLower(profiles[i].Title) < strings.ToLower(profiles[j].Title)
	})
}

func systemContent(profiles []Profile) string {
	for _, profile := range profiles {
		if profile.System || profile.ID == systemProfileID {
			return profile.Content
		}
	}
	return ""
}

func normalizeHostsEnding(content string) string {
	content = strings.TrimRight(strings.ReplaceAll(content, "\r\n", "\n"), "\n\t ")
	if content == "" {
		return ""
	}
	return content + "\n"
}

func oneOf(value string, fallback string, allowed ...string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, item := range allowed {
		if normalized == item {
			return normalized
		}
	}
	return fallback
}

func stringValue(value interface{}, fallback string) string {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

func boolValue(value interface{}, fallback bool) bool {
	if parsed, ok := value.(bool); ok {
		return parsed
	}
	return fallback
}

func stableID(value string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(sum[:])[:12]
}

func strconvUnixNano() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
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
	return filepath.Join(base, "Ariadne", "hosts_profiles.json")
}

func defaultLegacyPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".x-tools", "hosts_profiles.json")
	}
	return ""
}

func defaultHostsPath() string {
	if strings.EqualFold(os.Getenv("OS"), "Windows_NT") {
		return filepath.Join(os.Getenv("WINDIR"), "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeHostsContent(path string, content string) error {
	if path == "" {
		return fmt.Errorf("缺少 Hosts 路径")
	}
	if strings.EqualFold(os.Getenv("OS"), "Windows_NT") {
		return writeHostsContentElevated(path, content)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeHostsContentElevated(hostsPath string, content string) error {
	tempFile, err := os.CreateTemp("", "ariadne-hosts-*.txt")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	resultFile, err := os.CreateTemp("", "ariadne-hosts-*.res")
	if err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	resultPath := resultFile.Name()
	_ = resultFile.Close()

	scriptFile, err := os.CreateTemp("", "ariadne-hosts-*.ps1")
	if err != nil {
		_ = os.Remove(tempPath)
		_ = os.Remove(resultPath)
		return err
	}
	scriptPath := scriptFile.Name()
	script := "$ErrorActionPreference = 'Stop'\n" +
		"try {\n" +
		"  Copy-Item -LiteralPath '" + psEscape(tempPath) + "' -Destination '" + psEscape(hostsPath) + "' -Force\n" +
		"  Set-Content -LiteralPath '" + psEscape(resultPath) + "' -Value 'SUCCESS' -Encoding UTF8\n" +
		"  exit 0\n" +
		"} catch {\n" +
		"  Set-Content -LiteralPath '" + psEscape(resultPath) + "' -Value $_.Exception.Message -Encoding UTF8\n" +
		"  exit 1\n" +
		"}\n"
	if _, err := scriptFile.WriteString(script); err != nil {
		scriptFile.Close()
		return err
	}
	_ = scriptFile.Close()
	defer func() {
		_ = os.Remove(tempPath)
		_ = os.Remove(resultPath)
		_ = os.Remove(scriptPath)
	}()

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		"Start-Process powershell -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File \""+scriptPath+"\"' -Verb RunAs -Wait -WindowStyle Hidden")
	if err := cmd.Run(); err != nil {
		return err
	}
	raw, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Errorf("无法获取写入结果，可能是用户拒绝了 UAC 请求")
	}
	result := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
	if strings.Contains(result, "SUCCESS") {
		return nil
	}
	if result == "" {
		result = "未知执行错误"
	}
	return errors.New(result)
}

func psEscape(path string) string {
	return strings.ReplaceAll(path, "'", "''")
}

func findVirtualizedFile(path string, appData string, localAppData string) (string, bool, int64) {
	if path == "" || appData == "" || localAppData == "" {
		return "", false, 0
	}
	relative, ok := pathRelativeTo(path, appData)
	if !ok {
		return "", false, 0
	}
	matches, err := filepath.Glob(filepath.Join(localAppData, "Packages", "*", "LocalCache", "Roaming", relative))
	if err != nil || len(matches) == 0 {
		return "", false, 0
	}
	sort.Slice(matches, func(i, j int) bool {
		left, leftErr := os.Stat(matches[i])
		right, rightErr := os.Stat(matches[j])
		if leftErr != nil || rightErr != nil {
			return matches[i] < matches[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr == nil && !info.IsDir() {
			return match, true, info.Size()
		}
	}
	return "", false, 0
}

func pathRelativeTo(path string, base string) (string, bool) {
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(base)
	relative, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return relative, true
}
