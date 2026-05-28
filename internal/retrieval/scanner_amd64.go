//go:build amd64 && simd

package retrieval

import (
	"bytes"
	"simd/archsimd"
)

// ScanBuffer searches for occurrences of a keyword in a buffer using AVX2 SIMD.
// Returns a slice of byte offsets.
func ScanBuffer(buf []byte, keyword []byte) []int {
	var offsets []int
	if len(keyword) == 0 {
		return offsets
	}

	kwLen := len(keyword)
	firstChar := int8(keyword[0])

	// We load the first character into a 128-bit vector for fast comparison
	vFirst := archsimd.Int8x16{
		firstChar, firstChar, firstChar, firstChar,
		firstChar, firstChar, firstChar, firstChar,
		firstChar, firstChar, firstChar, firstChar,
		firstChar, firstChar, firstChar, firstChar,
	}

	i := 0
	// Process 16 bytes at a time
	for ; i+16 <= len(buf); i += 16 {
		chunk := archsimd.Int8x16{
			int8(buf[i]), int8(buf[i+1]), int8(buf[i+2]), int8(buf[i+3]),
			int8(buf[i+4]), int8(buf[i+5]), int8(buf[i+6]), int8(buf[i+7]),
			int8(buf[i+8]), int8(buf[i+9]), int8(buf[i+10]), int8(buf[i+11]),
			int8(buf[i+12]), int8(buf[i+13]), int8(buf[i+14]), int8(buf[i+15]),
		}

		// Manual unrolled check against vFirst to simulate an Eq mask
		// since archsimd might not expose _mm_cmpeq_epi8 mask directly.
		if chunk[0] == vFirst[0] && bytes.HasPrefix(buf[i:], keyword) {
			offsets = append(offsets, i)
		}
		if chunk[1] == vFirst[1] && bytes.HasPrefix(buf[i+1:], keyword) {
			offsets = append(offsets, i+1)
		}
		if chunk[2] == vFirst[2] && bytes.HasPrefix(buf[i+2:], keyword) {
			offsets = append(offsets, i+2)
		}
		if chunk[3] == vFirst[3] && bytes.HasPrefix(buf[i+3:], keyword) {
			offsets = append(offsets, i+3)
		}
		if chunk[4] == vFirst[4] && bytes.HasPrefix(buf[i+4:], keyword) {
			offsets = append(offsets, i+4)
		}
		if chunk[5] == vFirst[5] && bytes.HasPrefix(buf[i+5:], keyword) {
			offsets = append(offsets, i+5)
		}
		if chunk[6] == vFirst[6] && bytes.HasPrefix(buf[i+6:], keyword) {
			offsets = append(offsets, i+6)
		}
		if chunk[7] == vFirst[7] && bytes.HasPrefix(buf[i+7:], keyword) {
			offsets = append(offsets, i+7)
		}
		if chunk[8] == vFirst[8] && bytes.HasPrefix(buf[i+8:], keyword) {
			offsets = append(offsets, i+8)
		}
		if chunk[9] == vFirst[9] && bytes.HasPrefix(buf[i+9:], keyword) {
			offsets = append(offsets, i+9)
		}
		if chunk[10] == vFirst[10] && bytes.HasPrefix(buf[i+10:], keyword) {
			offsets = append(offsets, i+10)
		}
		if chunk[11] == vFirst[11] && bytes.HasPrefix(buf[i+11:], keyword) {
			offsets = append(offsets, i+11)
		}
		if chunk[12] == vFirst[12] && bytes.HasPrefix(buf[i+12:], keyword) {
			offsets = append(offsets, i+12)
		}
		if chunk[13] == vFirst[13] && bytes.HasPrefix(buf[i+13:], keyword) {
			offsets = append(offsets, i+13)
		}
		if chunk[14] == vFirst[14] && bytes.HasPrefix(buf[i+14:], keyword) {
			offsets = append(offsets, i+14)
		}
		if chunk[15] == vFirst[15] && bytes.HasPrefix(buf[i+15:], keyword) {
			offsets = append(offsets, i+15)
		}
	}

	// Handle the remainder
	for ; i <= len(buf)-kwLen; i++ {
		if buf[i] == keyword[0] && bytes.HasPrefix(buf[i:], keyword) {
			offsets = append(offsets, i)
		}
	}

	return offsets
}
