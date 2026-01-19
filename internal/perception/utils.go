package perception

import "strings"

// requiresJSONOutput checks if the prompt contains markers indicating a JSON output is expected.
// This is used to automatically enable JSON mode or set response MIME types for certain models.
func requiresJSONOutput(systemPrompt, userPrompt string) bool {
	markers := []string{
		"mangle_synth_v1",
		"MangleSynth",
		"Output ONLY a MangleSynth JSON object",
		"responseJsonSchema",
		"responseMimeType",
		"application/json",
	}
	combined := systemPrompt + "\n" + userPrompt
	for _, marker := range markers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}
