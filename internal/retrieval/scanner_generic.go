//go:build !amd64 || !simd

package retrieval

import (
	"bytes"
)

// ScanBuffer searches for occurrences of a keyword in a buffer.
// Returns a slice of byte offsets.
func ScanBuffer(buf []byte, keyword []byte) []int {
	var offsets []int
	if len(keyword) == 0 {
		return offsets
	}

	var pos int
	for {
		idx := bytes.Index(buf[pos:], keyword)
		if idx == -1 {
			break
		}

		matchPos := pos + idx
		offsets = append(offsets, matchPos)
		pos = matchPos + len(keyword)
	}
	return offsets
}
