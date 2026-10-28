package perception

import (
	"encoding/json"
	"strings"
)

// visibleTaskOutput projects envelope responses onto their displayed answer for
// interaction audits and vocabulary learning. Native thinking blocks are already
// separate at the provider boundary. This also keeps retired control fields in
// historical records out of the critic's input.
func visibleTaskOutput(response string) string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(response), &fields); err != nil {
		// A partial control envelope is not a visible answer. Do not recover its
		// control prose as fallback learning material.
		for _, marker := range []string{`"control_packet"`, `"reasoning_trace"`, `"chain_of_thought"`} {
			if strings.Contains(response, marker) {
				return ""
			}
		}
		return response
	}
	if _, envelope := fields["control_packet"]; envelope {
		var surface string
		if err := json.Unmarshal(fields["surface_response"], &surface); err != nil {
			return ""
		}
		return surface
	}
	changed := false
	for _, key := range []string{"reasoning_trace", "chain_of_thought"} {
		if _, found := fields[key]; found {
			delete(fields, key)
			changed = true
		}
	}
	if changed {
		data, err := json.Marshal(fields)
		if err != nil {
			return ""
		}
		return string(data)
	}
	return response
}
