package apitesting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const gitCollectionFileName = "ariadne-api-collection.json"

type GitConfig struct {
	Path   string `json:"path,omitempty"`
	Remote string `json:"remote,omitempty"`
	Branch string `json:"branch,omitempty"`
}

type GitStatus struct {
	OK           bool     `json:"ok"`
	Message      string   `json:"message"`
	CollectionID string   `json:"collectionId,omitempty"`
	Path         string   `json:"path,omitempty"`
	Remote       string   `json:"remote,omitempty"`
	Branch       string   `json:"branch,omitempty"`
	Dirty        bool     `json:"dirty"`
	Files        []string `json:"files,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func (s *Service) ConfigureGit(collectionID string, repoPath string, remote string) GitStatus {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return GitStatus{OK: false, Message: "请选择 Git 目录"}
	}
	absolutePath, err := filepath.Abs(repoPath)
	if err != nil {
		return GitStatus{OK: false, Message: "Git 目录无效", Error: err.Error()}
	}
	if err := ensureGitRepository(absolutePath, strings.TrimSpace(remote)); err != nil {
		return GitStatus{OK: false, Message: "Git 仓库初始化失败", Path: absolutePath, Error: err.Error()}
	}

	s.mu.Lock()
	s.ensureActiveLocked()
	index := s.collectionIndexLocked(collectionID)
	if index < 0 {
		s.mu.Unlock()
		return GitStatus{OK: false, Message: "集合不存在", Path: absolutePath}
	}
	s.collections[index].Git = GitConfig{
		Path:   absolutePath,
		Remote: strings.TrimSpace(remote),
		Branch: currentGitBranch(absolutePath),
	}
	s.collections[index].UpdatedAt = time.Now().Unix()
	collection := cloneCollection(s.collections[index])
	s.saveLockedWithStatus()
	saveErr := s.lastSaveError
	s.mu.Unlock()
	if saveErr != "" {
		return GitStatus{OK: false, Message: "Git 配置保存失败", Path: absolutePath, Error: saveErr}
	}
	if err := writeCollectionToGit(collection); err != nil {
		return GitStatus{OK: false, Message: "集合写入 Git 目录失败", Path: absolutePath, Error: err.Error()}
	}
	status := s.GitStatus(collection.ID)
	if status.OK {
		status.Message = "Git 已绑定"
	}
	return status
}

func (s *Service) GitStatus(collectionID string) GitStatus {
	collection, ok := s.collectionForGit(collectionID)
	if !ok {
		return GitStatus{OK: false, Message: "集合不存在"}
	}
	if strings.TrimSpace(collection.Git.Path) == "" {
		return GitStatus{OK: false, Message: "未绑定 Git 目录", CollectionID: collection.ID}
	}
	return gitStatusForCollection(collection, "Git 状态已更新")
}

func (s *Service) GitPull(collectionID string) GitStatus {
	collection, ok := s.collectionForGit(collectionID)
	if !ok {
		return GitStatus{OK: false, Message: "集合不存在"}
	}
	if strings.TrimSpace(collection.Git.Path) == "" {
		return GitStatus{OK: false, Message: "未绑定 Git 目录", CollectionID: collection.ID}
	}
	if _, err := runGit(collection.Git.Path, "pull", "--ff-only"); err != nil {
		return gitStatusWithError(collection, "拉取失败", err)
	}
	if err := s.loadCollectionFromGit(collection.ID); err != nil {
		return gitStatusWithError(collection, "拉取完成，合集导入失败", err)
	}
	return gitStatusForCollection(collection, "拉取完成")
}

func (s *Service) GitCommitPush(collectionID string, message string) GitStatus {
	collection, ok := s.collectionForGit(collectionID)
	if !ok {
		return GitStatus{OK: false, Message: "集合不存在"}
	}
	if strings.TrimSpace(collection.Git.Path) == "" {
		return GitStatus{OK: false, Message: "未绑定 Git 目录", CollectionID: collection.ID}
	}
	if err := writeCollectionToGit(collection); err != nil {
		return gitStatusWithError(collection, "集合写入 Git 目录失败", err)
	}
	if _, err := runGit(collection.Git.Path, "add", gitCollectionFileName); err != nil {
		return gitStatusWithError(collection, "暂存失败", err)
	}
	status := gitStatusForCollection(collection, "")
	if status.OK && !status.Dirty {
		if strings.TrimSpace(collection.Git.Remote) == "" {
			status.Message = "没有本地修改"
			return status
		}
		if _, err := runGit(collection.Git.Path, "push"); err != nil {
			return gitStatusWithError(collection, "没有本地修改，推送失败", err)
		}
		status.Message = "没有本地修改"
		return status
	}
	message = firstNonEmpty(message, fmt.Sprintf("Update %s", collection.Name))
	if _, err := runGit(collection.Git.Path, "commit", "-m", message); err != nil {
		return gitStatusWithError(collection, "提交失败", err)
	}
	if strings.TrimSpace(collection.Git.Remote) == "" {
		return gitStatusForCollection(collection, "提交完成")
	}
	if _, err := runGit(collection.Git.Path, "push", "-u", "origin", currentGitBranch(collection.Git.Path)); err != nil {
		return gitStatusWithError(collection, "提交完成，推送失败", err)
	}
	return gitStatusForCollection(collection, "提交并推送完成")
}

func (s *Service) syncCollectionToGit(collectionID string) error {
	collection, ok := s.collectionForGit(collectionID)
	if !ok || strings.TrimSpace(collection.Git.Path) == "" {
		return nil
	}
	return writeCollectionToGit(collection)
}

func (s *Service) loadCollectionFromGit(collectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.collectionIndexLocked(collectionID)
	if index < 0 {
		return errors.New("集合不存在")
	}
	path := gitCollectionFilePath(s.collections[index])
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var loaded Collection
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return err
	}
	loaded.ID = s.collections[index].ID
	loaded.Git = s.collections[index].Git
	loaded = normalizeCollection(loaded)
	s.collections[index] = loaded
	s.activeID = loaded.ID
	s.saveLockedWithStatus()
	if s.lastSaveError != "" {
		return errors.New(s.lastSaveError)
	}
	return nil
}

func (s *Service) collectionForGit(collectionID string) (Collection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	index := s.collectionIndexLocked(collectionID)
	if index < 0 {
		return Collection{}, false
	}
	return cloneCollection(s.collections[index]), true
}

func (s *Service) collectionIndexLocked(collectionID string) int {
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		collectionID = s.activeID
	}
	for index := range s.collections {
		if s.collections[index].ID == collectionID {
			return index
		}
	}
	return -1
}

func ensureGitRepository(path string, remote string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if _, err := runGit(path, "init"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(remote) == "" {
		return nil
	}
	if _, err := runGit(path, "remote", "get-url", "origin"); err == nil {
		_, err = runGit(path, "remote", "set-url", "origin", remote)
		return err
	}
	_, err := runGit(path, "remote", "add", "origin", remote)
	return err
}

func writeCollectionToGit(collection Collection) error {
	if strings.TrimSpace(collection.Git.Path) == "" {
		return nil
	}
	if err := os.MkdirAll(collection.Git.Path, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(gitFileCollection(normalizeCollection(collection)), "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(gitCollectionFilePath(collection), raw, 0o644)
}

type gitFileCollectionPayload struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	Variables           []Variable    `json:"variables"`
	Environments        []Environment `json:"environments"`
	Requests            []Request     `json:"requests"`
	ActiveEnvironmentID string        `json:"activeEnvironmentId"`
	ActiveRequestID     string        `json:"activeRequestId"`
	UpdatedAt           int64         `json:"updatedAt"`
}

func gitFileCollection(collection Collection) gitFileCollectionPayload {
	return gitFileCollectionPayload{
		ID:                  collection.ID,
		Name:                collection.Name,
		Variables:           collection.Variables,
		Environments:        collection.Environments,
		Requests:            collection.Requests,
		ActiveEnvironmentID: collection.ActiveEnvironmentID,
		ActiveRequestID:     collection.ActiveRequestID,
		UpdatedAt:           collection.UpdatedAt,
	}
}

func gitCollectionFilePath(collection Collection) string {
	return filepath.Join(collection.Git.Path, gitCollectionFileName)
}

func gitStatusForCollection(collection Collection, message string) GitStatus {
	output, err := runGit(collection.Git.Path, "status", "--porcelain=v1", "--branch")
	if err != nil {
		return gitStatusWithError(collection, "Git 状态读取失败", err)
	}
	status := GitStatus{
		OK:           true,
		Message:      firstNonEmpty(message, "Git 状态已更新"),
		CollectionID: collection.ID,
		Path:         collection.Git.Path,
		Remote:       collection.Git.Remote,
		Branch:       currentGitBranch(collection.Git.Path),
		Files:        []string{},
	}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "## ") {
			continue
		}
		status.Dirty = true
		status.Files = append(status.Files, line)
	}
	return status
}

func gitStatusWithError(collection Collection, message string, err error) GitStatus {
	return GitStatus{
		OK:           false,
		Message:      message,
		CollectionID: collection.ID,
		Path:         collection.Git.Path,
		Remote:       collection.Git.Remote,
		Branch:       currentGitBranch(collection.Git.Path),
		Error:        err.Error(),
	}
}

func currentGitBranch(path string) string {
	output, err := runGit(path, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func runGit(path string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", path}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return strings.TrimSpace(stdout.String()), errors.New(detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func cloneCollection(collection Collection) Collection {
	cloned := cloneCollections([]Collection{collection})
	if len(cloned) == 0 {
		return Collection{}
	}
	return cloned[0]
}
