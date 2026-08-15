package tools

import (
	"encoding/json"
	"strconv"
)

// CoerceInt accepts any of the shapes a tool argument can take on the way in
// and returns it as an int.
//
// LLM tool-call payloads round-trip through encoding/json, which without
// UseNumber materializes every JSON number as float64 — never int. Mangle
// sourced arguments arrive as int64, and a few call sites hand over decimal
// strings. A bare args[key].(int) therefore fails silently in production: the
// caller's limit is dropped and the tool's default is used instead, which is
// how grep stayed pinned at 50 matches while the model asked for 500.
//
// This is the single copy. Four near-identical private versions had already
// accumulated (core/search.go argInt, research/numeric_args.go argInt,
// shell/execute.go coerceInt, and inline float64 fallbacks in codedom/lines.go),
// each accepting a slightly different set of types — so the same argument was
// honored by one tool and ignored by the next.
func CoerceInt(v any) (int, bool) {
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
		if f, err := n.Float64(); err == nil {
			return int(f), true
		}
	case string:
		if n == "" {
			return 0, false
		}
		if i, err := strconv.Atoi(n); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return int(f), true
		}
	}
	return 0, false
}

// ArgInt reads args[key] through CoerceInt. Returns (0, false) when the key is
// absent or the value cannot be read as a number.
func ArgInt(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	return CoerceInt(v)
}
