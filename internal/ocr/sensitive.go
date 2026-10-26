package ocr

import (
	"regexp"
	"strings"
)

var sensitiveCredentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:password|passwd|pwd|api[_-]?key|apikey|token|access[_-]?token|refresh[_-]?token|id[_-]?token|secret|client[_-]?secret|private[_-]?key)\b["']?\s*[:=]\s*["']?[a-z0-9._~+/=@#$%^&*!?-]{4,}`),
	regexp.MustCompile(`(?i)\bauthorization\b\s*:\s*(?:bearer|basic)\s+[a-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]{16,}`),
	regexp.MustCompile(`(?i)\b(?:cookie|set-cookie)\s*:\s*[a-z0-9._~-]{2,}=[a-z0-9._~+/=%-]{3,}`),
	regexp.MustCompile(`(?i)-----begin [a-z0-9 ]*private key-----`),
	regexp.MustCompile(`(?i)\bssh-(?:rsa|ed25519)\s+[a-z0-9+/=]{32,}`),
	regexp.MustCompile(`(?i)\beyj[a-z0-9_-]{10,}\.eyj[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\b`),
	regexp.MustCompile(`(?i)\b(?:sk|rk)-[a-z0-9_-]{20,}\b`),
	regexp.MustCompile(`(?i)\bgh[pousr]_[a-z0-9_]{20,}\b`),
	regexp.MustCompile(`(?i)(?:数据库密码|密码|密钥|令牌|验证码)\s*[:：=]\s*["']?[a-z0-9._~+/=@#$%^&*!?-]{4,}`),
}

func looksSensitiveText(text string) bool {
	text = strings.TrimSpace(text)
	for _, pattern := range sensitiveCredentialPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
