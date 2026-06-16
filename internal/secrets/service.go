package secrets

import (
	"fmt"
	"os"
	"strings"

	"ariadne/internal/securestore"
)

type SecretRecordStatus struct {
	Kind         string   `json:"kind"`
	Label        string   `json:"label"`
	TargetName   string   `json:"targetName"`
	Stored       bool     `json:"stored"`
	EnvNames     []string `json:"envNames"`
	EnvPresent   bool     `json:"envPresent"`
	ActiveSource string   `json:"activeSource"`
	LastError    string   `json:"lastError,omitempty"`
}

type SecretStatus struct {
	Available bool                 `json:"available"`
	Backend   string               `json:"backend"`
	Records   []SecretRecordStatus `json:"records"`
	LastError string               `json:"lastError,omitempty"`
}

type SaveSecretRequest struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type ClearSecretRequest struct {
	Kind    string `json:"kind"`
	Confirm bool   `json:"confirm"`
}

type SecretActionResult struct {
	OK                   bool         `json:"ok"`
	Message              string       `json:"message"`
	RequiresConfirmation bool         `json:"requiresConfirmation,omitempty"`
	Status               SecretStatus `json:"status"`
}

type credentialSpec struct {
	Kind   string
	Label  string
	Target string
	Envs   []string
}

type credentialStore interface {
	Available() bool
	Backend() string
	Read(target string) (string, bool, error)
	Write(target string, secret string) error
	Delete(target string) error
}

type Service struct {
	store credentialStore
}

func NewService() *Service {
	return NewServiceWithStore(securestore.DefaultStore{})
}

func NewServiceWithStore(store credentialStore) *Service {
	if store == nil {
		store = securestore.DefaultStore{}
	}
	return &Service{store: store}
}

func (s *Service) Status() SecretStatus {
	return s.status()
}

func (s *Service) SaveSecret(request SaveSecretRequest) SecretActionResult {
	spec, ok := findSpec(request.Kind)
	if !ok {
		status := s.status()
		return SecretActionResult{OK: false, Message: "未知密钥类型", Status: status}
	}
	if s.store == nil || !s.store.Available() {
		status := s.status()
		return SecretActionResult{OK: false, Message: "当前平台未提供安全凭据存储", Status: status}
	}
	value := strings.TrimSpace(request.Value)
	if value == "" {
		status := s.status()
		return SecretActionResult{OK: false, Message: "密钥不能为空", Status: status}
	}
	if err := s.store.Write(spec.Target, value); err != nil {
		status := s.status()
		return SecretActionResult{OK: false, Message: "密钥保存失败: " + shortError(err.Error()), Status: status}
	}
	status := s.status()
	return SecretActionResult{OK: true, Message: fmt.Sprintf("%s 已保存到 Windows Credential Manager", spec.Label), Status: status}
}

func (s *Service) ClearSecret(request ClearSecretRequest) SecretActionResult {
	spec, ok := findSpec(request.Kind)
	status := s.status()
	if !ok {
		return SecretActionResult{OK: false, Message: "未知密钥类型", Status: status}
	}
	if !request.Confirm {
		return SecretActionResult{
			OK:                   false,
			Message:              "再次点击确认清除 " + spec.Label,
			RequiresConfirmation: true,
			Status:               status,
		}
	}
	if s.store == nil || !s.store.Available() {
		return SecretActionResult{OK: false, Message: "当前平台未提供安全凭据存储", Status: status}
	}
	if err := s.store.Delete(spec.Target); err != nil {
		status = s.status()
		return SecretActionResult{OK: false, Message: "密钥清除失败: " + shortError(err.Error()), Status: status}
	}
	status = s.status()
	return SecretActionResult{OK: true, Message: spec.Label + " 已从安全存储清除", Status: status}
}

func (s *Service) status() SecretStatus {
	status := SecretStatus{
		Available: s.store != nil && s.store.Available(),
		Backend:   "unsupported",
		Records:   make([]SecretRecordStatus, 0, len(specs())),
	}
	if s.store != nil {
		status.Backend = s.store.Backend()
	}
	for _, spec := range specs() {
		record := SecretRecordStatus{
			Kind:       spec.Kind,
			Label:      spec.Label,
			TargetName: spec.Target,
			EnvNames:   append([]string(nil), spec.Envs...),
		}
		for _, env := range spec.Envs {
			if strings.TrimSpace(os.Getenv(env)) != "" {
				record.EnvPresent = true
				break
			}
		}
		if s.store != nil && s.store.Available() {
			_, stored, err := s.store.Read(spec.Target)
			if err != nil {
				record.LastError = err.Error()
				if status.LastError == "" {
					status.LastError = err.Error()
				}
			}
			record.Stored = stored
		}
		switch {
		case record.EnvPresent:
			record.ActiveSource = "environment"
		case record.Stored:
			record.ActiveSource = "credential_manager"
		default:
			record.ActiveSource = "missing"
		}
		status.Records = append(status.Records, record)
	}
	return status
}

func specs() []credentialSpec {
	return []credentialSpec{
		{
			Kind:   "ai_api_key",
			Label:  "AI API key",
			Target: securestore.TargetOpenAIAPIKey,
			Envs:   []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		},
		{
			Kind:   "embedding_api_key",
			Label:  "Embedding API key",
			Target: securestore.TargetEmbeddingAPIKey,
			Envs:   []string{"ARIADNE_EMBED_API_KEY", "EMBED__API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		},
		{
			Kind:   "milvus_token",
			Label:  "Milvus token",
			Target: securestore.TargetMilvusToken,
			Envs:   []string{"ARIADNE_MILVUS_TOKEN", "MILVUS__TOKEN", "MILVUS_TOKEN"},
		},
	}
}

func findSpec(kind string) (credentialSpec, bool) {
	normalized := strings.TrimSpace(strings.ToLower(kind))
	for _, spec := range specs() {
		if spec.Kind == normalized {
			return spec, true
		}
	}
	return credentialSpec{}, false
}

func shortError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 120 {
		return message
	}
	return message[:117] + "..."
}
