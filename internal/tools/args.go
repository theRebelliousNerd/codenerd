package tools

import (
	"encoding/json"
	"math"
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
// This is the single copy. Nine near-identical private versions had already
// accumulated (core/search.go argInt, core/context_recall.go integer,
// research/numeric_args.go argInt, research/browser_progressive.go intArg,
// research/browser_reasoning.go int64Arg, mcpctl/mcpctl.go intArg,
// shell/execute.go coerceInt, codedom/apply_edits.go applyCoerceInt, and inline
// float64 fallbacks in codedom/lines.go), each accepting a slightly different
// set of types — so the same argument was honored by one tool and ignored by
// the next.
//
// Fractional values truncate toward zero: 4.7 reads as 4. That is the right
// call for soft quantities (timeouts, limits, counts), and it is pinned by
// shell's TestCoerceInt. It is the wrong call for exact addresses — a line
// number, an offset into retained output — where 1.5 silently becoming 1
// reads or edits a region the caller did not ask for. Those call sites must
// use CoerceIntStrict, which refuses the fractional value instead.
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
		return int64ToInt(n)
	case uint:
		if uint64(n) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		if uint64(n) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(n), true
	case uint64:
		if n > uint64(math.MaxInt) {
			return 0, false
		}
		return int(n), true
	case float32:
		return coerceFloat(float64(n), false)
	case float64:
		return coerceFloat(n, false)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int64ToInt(i)
		}
		if f, err := n.Float64(); err == nil {
			return coerceFloat(f, false)
		}
	case string:
		if n == "" {
			return 0, false
		}
		if i, err := strconv.Atoi(n); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return coerceFloat(f, false)
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

// CoerceIntStrict is CoerceInt for exact addresses: it accepts the same
// shapes but refuses fractional values (float64 1.5, json.Number "3.8",
// string "6.5") instead of truncating them. Line numbers and retained-output
// offsets must fail closed this way; soft quantities use CoerceInt.
func CoerceIntStrict(v any) (int, bool) {
	switch n := v.(type) {
	case float32:
		return coerceFloat(float64(n), true)
	case float64:
		return coerceFloat(n, true)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int64ToInt(i)
		}
		return 0, false
	case string:
		if n == "" {
			return 0, false
		}
		if i, err := strconv.Atoi(n); err == nil {
			return i, true
		}
		return 0, false
	default:
		return CoerceInt(v)
	}
}

// int64ToInt converts an int64 to int, refusing magnitudes outside int range
// (load-bearing on 32-bit platforms; a no-op check on 64-bit).
func int64ToInt(n int64) (int, bool) {
	if n > int64(math.MaxInt) || n < int64(math.MinInt) {
		return 0, false
	}
	return int(n), true
}

// coerceFloat converts f to int, refusing NaN, infinities, and magnitudes
// outside int range. Soft callers truncate in-range fractions; strict callers
// refuse them. The upper bound is exclusive because float64(math.MaxInt)
// rounds up to 2^63 on 64-bit platforms — checking <= that rounded value
// would admit an overflowing conversion.
func coerceFloat(f float64, strict bool) (int, bool) {
	maxExclusive := float64(uint64(math.MaxInt) + 1)
	if math.IsNaN(f) || math.IsInf(f, 0) || f < float64(math.MinInt) || f >= maxExclusive {
		return 0, false
	}
	if strict && f != math.Trunc(f) {
		return 0, false
	}
	return int(f), true
}

// CoerceInt64 is the 64-bit canonical coercion for millisecond timestamps and
// windows (since_ms, before_ms, time_window_ms), which exceed int32 and thread
// through int64 APIs. Soft semantics: in-range fractions truncate —
// sub-millisecond precision is meaningless to an evidence filter — while NaN,
// infinities, and out-of-range magnitudes are refused.
func CoerceInt64(v any) (int64, bool) {
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		if uint64(n) > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(n), true
	case float32:
		return coerceFloat64(float64(n))
	case float64:
		return coerceFloat64(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		if f, err := n.Float64(); err == nil {
			return coerceFloat64(f)
		}
	case string:
		if n == "" {
			return 0, false
		}
		if i, err := strconv.ParseInt(n, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return coerceFloat64(f)
		}
	}
	return 0, false
}

// coerceFloat64 converts f to int64, refusing NaN, infinities, and magnitudes
// outside int64 range. In-range fractions truncate. The upper bound is
// exclusive because float64(math.MaxInt64) rounds up to 2^63 — checking <=
// that rounded value would admit an overflowing conversion.
func coerceFloat64(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) ||
		f < float64(math.MinInt64) || f >= float64(uint64(math.MaxInt64)+1) {
		return 0, false
	}
	return int64(f), true
}

// ArgIntStrict reads args[key] through CoerceIntStrict. Returns (0, false)
// when the key is absent or the value is not integral.
func ArgIntStrict(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	return CoerceIntStrict(v)
}
