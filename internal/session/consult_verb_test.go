package session

import "testing"

// A consultation's routing verb names the agent as the roster does, hyphens
// included; anything that is not one bare segment is still refused, and none
// of them is a Mangle atom the tool-call query may interpolate.
func TestIsConsultIntentVerb_AcceptsRosterAgentNames(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"/consult/mangle-logic-architect", "/consult/codenerd-dogfood", "/consult/goexpert", "/consult/arch_propose_auditor"} {
		if !isConsultIntentVerb(verb) {
			t.Errorf("isConsultIntentVerb(%q) = false; a roster agent's consultation reads as malformed", verb)
		}
		if validMangleVerb(verb) {
			t.Errorf("validMangleVerb(%q) = true; a consultation verb must never reach the policy query", verb)
		}
	}
	for _, verb := range []string{"/consult/", "/consult/a b", "/consult/a/b", "/consult/x)", "/fix"} {
		if isConsultIntentVerb(verb) {
			t.Errorf("isConsultIntentVerb(%q) = true", verb)
		}
	}
}
