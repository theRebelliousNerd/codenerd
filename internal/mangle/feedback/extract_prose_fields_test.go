package feedback

import "testing"

// The 2026-10-28 dogfood session sent "CONFIDENCE: ..." and the rationale
// after it to the sandbox compiler ("mismatched input 'CONFIDENCE:'"): the
// structured-format cut ran only on a bare reply, not on a reply wrapped in a
// JSON field or a code block. Whatever path finds the candidate, only the rule
// reaches the compiler.
func TestExtractRuleFromResponse_ProseFieldsNeverReachTheRule(t *testing.T) {
	const rule = "next_action(/pause) :- current_intent(/current_intent), !executive_processed_intent(/current_intent)."
	cases := map[string]string{
		"json surface_response": `{"surface_response": "RULE: ` + rule + `\nCONFIDENCE: 0.8\nRATIONALE: avoid the loop; re-open only after -o review", "control_packet": {}}`,
		"json rule field":       `{"rule": "` + rule + `\nCONFIDENCE: 0.8"}`,
		"json string":           `"` + rule + `\nRATIONALE: because"`,
		"code block":            "```mangle\n" + rule + "\nCONFIDENCE: 0.8\nRATIONALE: because\n```",
		"bold labels":           "**RULE:** " + rule + "\n**CONFIDENCE:** 0.8\n**RATIONALE:** because",
		"mixed-case label":      "```\n" + rule + "\nConfidence: high\n```",
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ExtractRuleFromResponse(response); got != rule {
				t.Fatalf("extracted %q\nwant      %q", got, rule)
			}
		})
	}
}

// A multi-line rule keeps every clause line; only the labelled field after it
// is cut.
func TestExtractRuleFromResponse_MultiLineRuleSurvivesTheCut(t *testing.T) {
	response := "```mangle\nnext_action(/run) :-\n    test(/fail),\n    !blocked(/test).\nCONFIDENCE: 0.85\n```"
	want := "next_action(/run) :-\n    test(/fail),\n    !blocked(/test)."
	if got := ExtractRuleFromResponse(response); got != want {
		t.Fatalf("extracted %q\nwant      %q", got, want)
	}
}
