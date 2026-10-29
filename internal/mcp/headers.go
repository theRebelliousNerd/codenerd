package mcp

import (
	"os"
	"strings"
)

// ExpandHeaderValues resolves ${VAR} / $VAR references in configured header
// values against the process environment.
//
// The point is that an auth header belongs in the environment, not in a
// workspace config file that gets committed. A reference to an unset variable
// expands to the empty string and the header is dropped rather than sent as a
// literal "${MY_TOKEN}", which would otherwise reach the remote server as a
// bogus credential and produce a confusing 401.
func ExpandHeaderValues(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for name, value := range headers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		expanded := strings.TrimSpace(os.ExpandEnv(value))
		if expanded == "" {
			continue
		}
		out[name] = expanded
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
