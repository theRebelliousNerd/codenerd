package core

import "codenerd/internal/tactile"

// detectLineEnding returns the dominant line ending in data by counting
// byte sequences. It counts occurrences of '\n' and occurrences of '\r'+'\n'
// in a single pass over the bytes.
// If the file contains no newline or is dominated by LF, it returns "\n";
// if CRLF occurrences dominate (more CRLFs than lone LFs), it returns "\r\n".
// When the file does not exist or is empty, callers should keep the current
// behaviour (LF) rather than inventing a new convention.
func detectLineEnding(data []byte) string {
	return tactile.DetectLineEnding(data)
}

// normalizeLineEnding converts content to use the given line ending.
// It is cheap and does not use regex. For CRLF, it first normalizes any
// existing CRLF to LF then converts LF to CRLF to avoid doubling.
// For LF, it converts any CRLF to LF.
func normalizeLineEnding(content string, ending string) string {
	return tactile.NormalizeLineEnding(content, ending)
}

// existingLineEnding reads the file at path and returns its dominant line
// ending. A missing file is not an error and signals the caller to keep current
// new-file behavior; other read failures are returned.
func existingLineEnding(path string) (string, bool, error) {
	return tactile.ExistingLineEnding(path)
}
