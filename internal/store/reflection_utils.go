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
	// Combined regex for key-value secrets (e.g., "api_key: secret")
	// Matches: key followed by separator OR "bearer" followed by space.
	// Group 1: The key+separator (e.g., "api_key: " or "bearer ")
	// Group 2: The value (e.g., "secret")
	//
	// Optimization: Combined 5 separate patterns into one to reduce passes over the string.
	// Also fixed double backslash bug (\\s -> \s) in raw strings.
	combinedKeyPattern = regexp.MustCompile(`(?i)((?:api[_-]?key|secret|token|password)\s*[:=]\s*|bearer\s+)([^\s,]+)`)

	// Combined regex for standalone secrets (e.g., "AIza...", "sk-...")
	// Matches known secret formats directly.
	//
	// Optimization: Combined 3 separate patterns into one.
	combinedSecretPattern = regexp.MustCompile(`(AIza[0-9A-Za-z_-]{10,}|sk-[A-Za-z0-9]{10,}|ctx7sk-[0-9a-f-]{8,})`)

	// File hint pattern
	// Fixed double backslash bug (\\b -> \b) in raw strings.
	fileHintPattern = regexp.MustCompile(`\b[\w./-]+\.(go|mg|mangle|yaml|yml|json|md|ts|tsx|js|jsx|py|rs|java|sh|ps1|txt)\b`)
)

func sanitizeDescriptor(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}

	// Redact key-value pairs (single pass for 5 patterns)
	text = combinedKeyPattern.ReplaceAllString(text, "${1}[redacted]")

	// Redact standalone secrets (single pass for 3 patterns)
	text = combinedSecretPattern.ReplaceAllString(text, "[redacted]")

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
