package research

import "encoding/json"

// argInt extracts an integer tool argument, tolerating the numeric types that
// actually arrive at runtime.
//
// LLM tool-call arguments are JSON-decoded (perception/client_tool_helpers.go),
// and encoding/json without UseNumber() materializes every JSON number as
// float64 — never int. Mangle-sourced args arrive as int64. A bare
// args[key].(int) therefore silently fails in production, so caller-supplied
// limits (max_docs / max_length / max_results) were discarded and the default
// was always used. This mirrors the int/int64/float64 coercion the rest of the
// repo already uses (internal/tools/codedom/lines.go, internal/tools/shell).
func argInt(args map[string]any, key string) (int, bool) {
	switch v := args[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}
