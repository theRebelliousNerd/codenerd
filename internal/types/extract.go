package types

import (
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// FACT ARGUMENT EXTRACTION UTILITIES
// =============================================================================
//
// These functions provide safe, type-aware extraction from Fact.Args values.
// They replace the pervasive fmt.Sprintf("%v", fact.Args[N]) anti-pattern and
// bare type assertions that panic on type mismatch.
//
// Fact.Args values can be any of these Go types (after Mangle constant→Go
// conversion in engine.go's constantToInterface):
//   - string:        Plain text values
//   - MangleAtom:    Mangle name constants (e.g., "/active", "/read_file")
//   - int64:         Integer values (Mangle NumberType)
//   - int:           Go integers (less common, from manual fact construction)
//   - float64:       Float values (Mangle Float64Type)
//   - float32:       Go floats (less common)
//   - time.Time:     Timestamps (Mangle TimeType)
//   - time.Duration: Durations (Mangle DurationType)
//   - bool:          Boolean values

// ExtractString extracts a string representation from a fact argument.
// Handles string, MangleAtom, and falls back to fmt.Sprintf for other types.
// This is the safe replacement for fmt.Sprintf("%v", fact.Args[N]).
func ExtractString(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case MangleAtom:
		return string(v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case float32:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "/true"
		}
		return "/false"
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case time.Duration:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ExtractName extracts a Mangle name constant string from a fact argument.
// Returns the raw string value (with "/" prefix for atoms).
// For non-atom strings, returns the string as-is.
// This is specifically for extracting predicate-like identifiers such as
// "/read_file", "/active", "/coder".
func ExtractName(arg interface{}) string {
	switch v := arg.(type) {
	case MangleAtom:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ExtractInt64 extracts an int64 value from a fact argument.
// Returns (value, true) on success, (0, false) if the type is incompatible.
func ExtractInt64(arg interface{}) (int64, bool) {
	switch v := arg.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	default:
		return 0, false
	}
}

// ExtractFloat64 extracts a float64 value from a fact argument.
// Returns (value, true) on success, (0, false) if the type is incompatible.
func ExtractFloat64(arg interface{}) (float64, bool) {
	switch v := arg.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

// ExtractBool extracts a boolean value from a fact argument.
// Handles bool type directly and MangleAtom/string "/true"/"/false" conventions.
// Returns (value, true) on success, (false, false) if the type is incompatible.
func ExtractBool(arg interface{}) (bool, bool) {
	switch v := arg.(type) {
	case bool:
		return v, true
	case MangleAtom:
		s := string(v)
		if s == "/true" {
			return true, true
		}
		if s == "/false" {
			return false, true
		}
		return false, false
	case string:
		if v == "/true" || v == "true" {
			return true, true
		}
		if v == "/false" || v == "false" {
			return false, true
		}
		return false, false
	default:
		return false, false
	}
}

// ExtractTime extracts a time.Time value from a fact argument.
// Returns (value, true) on success, (zero, false) if the type is incompatible.
func ExtractTime(arg interface{}) (time.Time, bool) {
	switch v := arg.(type) {
	case time.Time:
		return v, true
	case int64:
		// Interpret as Unix nanoseconds (Mangle TimeType convention)
		return time.Unix(0, v).UTC(), true
	default:
		return time.Time{}, false
	}
}

// ExtractDuration extracts a time.Duration value from a fact argument.
// Returns (value, true) on success, (0, false) if the type is incompatible.
func ExtractDuration(arg interface{}) (time.Duration, bool) {
	switch v := arg.(type) {
	case time.Duration:
		return v, true
	case int64:
		// Interpret as nanoseconds (Mangle DurationType convention)
		return time.Duration(v), true
	default:
		return 0, false
	}
}

// ArgString is a convenience wrapper that extracts a string from fact.Args[i]
// with bounds checking. Returns "" if index is out of range.
func ArgString(f Fact, i int) string {
	if i < 0 || i >= len(f.Args) {
		return ""
	}
	return ExtractString(f.Args[i])
}

// ArgName is a convenience wrapper that extracts a name from fact.Args[i]
// with bounds checking. Returns "" if index is out of range.
func ArgName(f Fact, i int) string {
	if i < 0 || i >= len(f.Args) {
		return ""
	}
	return ExtractName(f.Args[i])
}

// ArgInt64 is a convenience wrapper that extracts an int64 from fact.Args[i]
// with bounds checking. Returns (0, false) if index is out of range.
func ArgInt64(f Fact, i int) (int64, bool) {
	if i < 0 || i >= len(f.Args) {
		return 0, false
	}
	return ExtractInt64(f.Args[i])
}

// ArgFloat64 is a convenience wrapper that extracts a float64 from fact.Args[i]
// with bounds checking. Returns (0, false) if index is out of range.
func ArgFloat64(f Fact, i int) (float64, bool) {
	if i < 0 || i >= len(f.Args) {
		return 0, false
	}
	return ExtractFloat64(f.Args[i])
}

// StripAtomPrefix strips the leading "/" from a Mangle atom name.
// This is useful when converting atom values like "/read_file" to plain
// identifiers like "read_file" for use in Go logic.
func StripAtomPrefix(s string) string {
	return strings.TrimPrefix(s, "/")
}
