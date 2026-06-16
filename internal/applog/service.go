package applog

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Status struct {
	Path            string `json:"path"`
	Directory       string `json:"directory"`
	DirectoryExists bool   `json:"directoryExists"`
	Exists          bool   `json:"exists"`
	Bytes           int64  `json:"bytes"`
	LastModifiedAt  int64  `json:"lastModifiedAt,omitempty"`
	LastError       string `json:"lastError,omitempty"`
}

type Service struct {
	mu        sync.Mutex
	path      string
	file      *os.File
	lastError string
}

func NewService() *Service {
	return NewServiceWithPath(defaultLogPath())
}

func NewServiceWithPath(path string) *Service {
	return &Service{path: path}
}

func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		s.lastError = err.Error()
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		s.lastError = err.Error()
		return err
	}
	s.file = file
	s.lastError = ""
	_, _ = s.file.WriteString(time.Now().Format(time.RFC3339) + " ariadne log started\n")
	return nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	if err != nil {
		s.lastError = err.Error()
	}
	return err
}

func (s *Service) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return len(data), nil
	}
	written, err := s.file.Write(data)
	if err != nil {
		s.lastError = err.Error()
	}
	return written, err
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return statusForPath(s.path, s.lastError)
}

func defaultLogPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		workingDir, err := os.Getwd()
		if err == nil {
			base = workingDir
		} else {
			base = "."
		}
	}
	return filepath.Join(base, "Ariadne", "logs", "ariadne.log")
}

func statusForPath(path string, lastError string) Status {
	status := Status{
		Path:      path,
		Directory: filepath.Dir(path),
		LastError: lastError,
	}
	if info, err := os.Stat(status.Directory); err == nil && info.IsDir() {
		status.DirectoryExists = true
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		status.Exists = true
		status.Bytes = info.Size()
		status.LastModifiedAt = info.ModTime().Unix()
	}
	return status
}
