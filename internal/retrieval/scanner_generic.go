package retrieval

import (
	"bytes"
)

// ScanBuffer searches for occurrences of a keyword in a buffer.
// Returns a slice of byte offsets.
//
// The previous amd64/simd variant was removed because it performed no vector
// operations — it constructed archsimd.Int8x16 values and then compared them
// element-by-element with sixteen scalar chunk[N]==vFirst[N] checks — and it
// never compiled against Go 1.26's archsimd (implicit assignment to unexported
// field). This generic implementation is now the single unconditional
// implementation for all architectures and build tags.

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
