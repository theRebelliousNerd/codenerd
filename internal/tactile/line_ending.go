package tactile

import (
	"os"
	"strings"
)

// DetectLineEnding returns the dominant newline convention in data. CRLF must
// strictly outnumber lone LF sequences; ties use LF.
func DetectLineEnding(data []byte) string {
	lfCount := 0
	crlfCount := 0
	for i, b := range data {
		if b != '\n' {
			continue
		}
		lfCount++
		if i > 0 && data[i-1] == '\r' {
			crlfCount++
		}
	}
	if crlfCount > lfCount-crlfCount {
		return "\r\n"
	}
	return "\n"
}

// NormalizeLineEnding converts content to one newline convention without
// doubling existing CRLF sequences.
func NormalizeLineEnding(content, ending string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if ending == "\r\n" {
		return strings.ReplaceAll(content, "\n", "\r\n")
	}
	return content
}

// ExistingLineEnding reads path and returns its dominant newline convention.
// A false exists result means callers should retain new-file LF behavior. Read
// failures for existing paths are returned so callers cannot silently rewrite
// a file whose convention could not be determined.
func ExistingLineEnding(path string) (ending string, exists bool, err error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "\n", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return DetectLineEnding(data), true, nil
}
