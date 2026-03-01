package store

import (
	"errors"
	"strconv"
	"unsafe"
)

// fastParseVectorJSON parses a JSON array of floats into []float32.
// It appends to the provided dest slice (resetting it first).
func fastParseVectorJSON(data []byte, dest []float32) ([]float32, error) {
	dest = dest[:0] // Reuse capacity

	if len(data) == 0 {
		return dest, nil
	}

	// Skip leading whitespace
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i == len(data) {
		return dest, nil // Empty or just whitespace
	}

	if data[i] != '[' {
		return nil, errors.New("expected '[' at start")
	}
	i++ // skip '['

	for i < len(data) {
		// Skip whitespace before number
		for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}
		if i == len(data) {
			break
		}

		if data[i] == ']' {
			return dest, nil
		}

		// Find end of number
		start := i
		for i < len(data) && (data[i] != ',' && data[i] != ']' && data[i] != ' ' && data[i] != '\t' && data[i] != '\n' && data[i] != '\r') {
			i++
		}

		// Parse number
		numBytes := data[start:i]
		if len(numBytes) > 0 {
			// Unsafe string conversion to avoid allocation
			s := *(*string)(unsafe.Pointer(&numBytes))

			f, err := strconv.ParseFloat(s, 32)
			if err != nil {
				return nil, err
			}
			dest = append(dest, float32(f))
		}

		// Skip whitespace after number
		for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}

		if i < len(data) && data[i] == ',' {
			i++
		} else if i < len(data) && data[i] == ']' {
			return dest, nil
		}
	}

	return dest, nil
}
