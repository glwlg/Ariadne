package applog

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const maxLogBytes = 5 * 1024 * 1024
const logBackupCount = 3

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
	bytes     int64
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
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		s.file = nil
		s.lastError = err.Error()
		return err
	}
	s.bytes = info.Size()
	s.lastError = ""
	_, err = s.writeLocked([]byte(time.Now().Format(time.RFC3339) + " ariadne log started\n"))
	return err
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
	return s.writeLocked(data)
}

func (s *Service) writeLocked(data []byte) (written int, err error) {
	defer func() {
		if err != nil {
			s.lastError = err.Error()
		}
	}()
	if s.file == nil {
		if s.lastError != "" {
			return 0, errors.New(s.lastError)
		}
		return len(data), nil
	}
	for len(data) > 0 {
		if s.bytes >= maxLogBytes {
			if err := s.rotateLocked(); err != nil {
				return written, err
			}
		}
		size := min(int64(len(data)), maxLogBytes-s.bytes)
		n, err := s.file.Write(data[:size])
		written += n
		s.bytes += int64(n)
		if err != nil {
			return written, err
		}
		data = data[n:]
	}
	return written, nil
}

func (s *Service) rotateLocked() error {
	err := s.file.Close()
	s.file = nil
	if err != nil {
		return err
	}
	oldest := s.path + "." + strconv.Itoa(logBackupCount)
	if err := os.Remove(oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := logBackupCount - 1; i >= 0; i-- {
		source := s.path
		if i > 0 {
			source += "." + strconv.Itoa(i)
		}
		if err := os.Rename(source, s.path+"."+strconv.Itoa(i+1)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	s.file, err = os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err == nil {
		s.bytes = 0
	}
	return err
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
