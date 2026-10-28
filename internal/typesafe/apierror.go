package typesafe

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// APIError is a non-200 reply. Detail names what the API objected to; for a
// 422 it lists every offending field path.
type APIError struct {
	Status     int
	Detail     string
	RetryAfter time.Duration // zero when the reply carried none
}

func (e *APIError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("typesafe: HTTP %d", e.Status)
	}
	return fmt.Sprintf("typesafe: HTTP %d: %s", e.Status, e.Detail)
}

// retryable is the API's own list: rate limited or overloaded.
func (e *APIError) retryable() bool { return e.Status == 429 || e.Status == 529 }

// errorDetail renders the body of an error reply. It handles the three shapes
// the spec gives (a string, an object, a list of located validation errors)
// and falls back to the raw text, never to nothing.
func errorDetail(body []byte) string {
	var env struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(body, &env) != nil || len(env.Detail) == 0 {
		return strings.TrimSpace(string(body))
	}
	var s string
	if json.Unmarshal(env.Detail, &s) == nil {
		return s
	}
	var fields []struct {
		Loc  []any  `json:"loc"`
		Msg  string `json:"msg"`
		Type string `json:"type"`
	}
	if json.Unmarshal(env.Detail, &fields) == nil && len(fields) > 0 {
		parts := make([]string, 0, len(fields))
		for _, f := range fields {
			loc := make([]string, len(f.Loc))
			for i, l := range f.Loc {
				loc[i] = fmt.Sprint(l)
			}
			parts = append(parts, fmt.Sprintf("%s: %s (%s)", strings.Join(loc, "."), f.Msg, f.Type))
		}
		return strings.Join(parts, "; ")
	}
	return string(env.Detail)
}
