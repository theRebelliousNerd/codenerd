package perception

import "testing"

// The interaction audit's Response projection read envelopes more narrowly
// than articulation admits them: a fenced envelope failed its whole-input
// decode and came out empty, an uppercase surface key came out empty, and
// uppercase outer keys left the whole control envelope in the record (audit
// interaction-audit-surface-projection-parity). The projection follows the
// admission grammar: one JSON object, optionally in one Markdown fence, keys
// by decoded spelling matched case-insensitively, a repeated key ambiguous.
func TestVisibleTaskOutput_FollowsTheAdmissionGrammar(t *testing.T) {
	const control = `"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[],"memory_operations":[]}`
	const want = "The comparison is complete."
	for name, response := range map[string]string{
		"canonical":            `{` + control + `,"surface_response":"` + want + `"}`,
		"fenced":               "```json\n{" + control + `,"surface_response":"` + want + "\"}\n```",
		"bare fence":           "```\n{" + control + `,"surface_response":"` + want + "\"}\n```",
		"uppercase surface":    `{` + control + `,"SURFACE_RESPONSE":"` + want + `"}`,
		"uppercase outer keys": `{"CONTROL_PACKET":{},"SURFACE_RESPONSE":"` + want + `"}`,
		"escaped key":          `{"control_packet":{},"surface_response":"` + want + `"}`,
	} {
		if got := visibleTaskOutput(response); got != want {
			t.Errorf("%s: projection = %q, want %q", name, got, want)
		}
	}

	// A key that appears twice has two readings: nothing is selected.
	if got := visibleTaskOutput(`{"control_packet":{},"surface_response":"a","Surface_Response":"b"}`); got != "" {
		t.Errorf("ambiguous envelope projected %q, want nothing", got)
	}
	// Reasoning fields go whatever their case; plain text stays.
	if got := visibleTaskOutput(`{"Chain_Of_Thought":["x"],"result":"Observed"}`); got != `{"result":"Observed"}` {
		t.Errorf("reasoning projection = %q", got)
	}
	if got := visibleTaskOutput("plain answer"); got != "plain answer" {
		t.Errorf("plain text = %q", got)
	}
}
