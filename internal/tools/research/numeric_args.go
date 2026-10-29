package research

import "codenerd/internal/tools"

// argInt extracts an integer tool argument, tolerating the numeric types that
// actually arrive at runtime.
//
// LLM tool-call arguments are JSON-decoded (perception/client_tool_helpers.go),
// and encoding/json without UseNumber() materializes every JSON number as
// float64 — never int. Mangle-sourced args arrive as int64. A bare
// args[key].(int) therefore silently fails in production, so caller-supplied
// limits (max_docs / max_length / max_results) were discarded and the default
// was always used. This delegates to the canonical tools.ArgInt: the old local
// copy rejected decimal strings, which recreated the same silent-drop bug one
// shape over (a model emitting max_docs "3" got the default 10 instead).
// Well-formed strings are unambiguous, so they are honored; garbage fails closed.
func argInt(args map[string]any, key string) (int, bool) {
	return tools.ArgInt(args, key)
}
