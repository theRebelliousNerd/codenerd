package store

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

const (
	traceDescriptorVersion  = 1
	learningHandleVersion   = 1
	defaultDescriptorMaxLen = 600
)

var (
	secretPatterns = []struct {
		replacement string
		pattern     *regexp.Regexp
	}{
		{replacement: "${1}[redacted]", pattern: regexp.MustCompile(`(?i)(api[_-]?key\\s*[:=]\\s*)([^\\s,]+)`)},
		{replacement: "${1}[redacted]", pattern: regexp.MustCompile(`(?i)(secret\\s*[:=]\\s*)([^\\s,]+)`)},
		{replacement: "${1}[redacted]", pattern: regexp.MustCompile(`(?i)(token\\s*[:=]\\s*)([^\\s,]+)`)},
		{replacement: "${1}[redacted]", pattern: regexp.MustCompile(`(?i)(password\\s*[:=]\\s*)([^\\s,]+)`)},
		{replacement: "bearer [redacted]", pattern: regexp.MustCompile(`(?i)bearer\\s+[A-Za-z0-9._-]+`)},
		{replacement: "[redacted]", pattern: regexp.MustCompile(`AIza[0-9A-Za-z_-]{10,}`)},
		{replacement: "[redacted]", pattern: regexp.MustCompile(`sk-[A-Za-z0-9]{10,}`)},
		{replacement: "[redacted]", pattern: regexp.MustCompile(`ctx7sk-[0-9a-f-]{8,}`)},
	}
	fileHintPattern = regexp.MustCompile(`\\b[\\w./-]+\\.(go|mg|mangle|yaml|yml|json|md|ts|tsx|js|jsx|py|rs|java|sh|ps1|txt)\\b`)
)

func sanitizeDescriptor(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	for _, pattern := range secretPatterns {
		text = pattern.pattern.ReplaceAllString(text, pattern.replacement)
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > defaultDescriptorMaxLen {
		text = text[:defaultDescriptorMaxLen]
	}
	return text
}

func computeDescriptorHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func extractFileHints(text string, max int) []string {
	if max <= 0 {
		max = 5
	}
	matches := fileHintPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var unique []string
	for _, m := range matches {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		unique = append(unique, m)
		if len(unique) >= max {
			break
		}
	}
	return unique
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}
