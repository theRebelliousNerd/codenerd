package prompt

import (
	"strings"
	"testing"
)

// envelopeMarkers are keys that exist only in the Piggyback envelope. Each is
// matched as a JSON key declaration (`"control_packet":`), not as a bare word,
// which is the difference between an atom SHOWING the envelope and one
// FORBIDDING it. "Do not wrap your reply in a control_packet envelope" is a
// legitimate and necessary instruction; `"control_packet": {` is the example
// that makes the model emit one.
var envelopeMarkers = []string{
	`"control_packet":`,
	`"surface_response":`,
	`"tool_requests":`,
	`"reasoning_trace":`,
	`"knowledge_requests":`,
	`"mangle_updates":`,
}

// filterAtomsForStructuredOutput drops atoms that impose a competing output
// contract, but it recognizes them by identity — CategoryProtocol, or an ID
// under protocol/piggyback/ or protocol/reasoning/. That is not the same thing
// as "contains an envelope".
//
// Live failure this test exists to prevent: campaign/planner/validation declared
// category "campaign", so the filter kept it, and its section VII showed the
// plan wrapped in a control_packet with "phases": [...] written as a literal
// ellipsis. The model copied the example exactly — envelope emitted, phases
// elided — and three consecutive campaigns decomposed to nothing and silently
// ran a generic three-task placeholder instead. campaign/librarian/protocol and
// campaign/extractor/output carried the same contradiction.
//
// So the invariant is not "protocol atoms are filtered" but: an atom that can
// reach a structured-output-only shard AND survives the filter must not contain
// envelope text at all.
func TestEmbeddedCorpus_NoEnvelopeAtomsReachStructuredOutputShards(t *testing.T) {
	corpus, err := getEmbeddedCorpusCached()
	if err != nil {
		t.Fatalf("load embedded corpus: %v", err)
	}

	atoms := corpus.All()
	if len(atoms) == 0 {
		t.Fatal("embedded corpus is empty — the go:embed of atoms/ is broken")
	}

	checked := 0
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		// Atoms the filter already removes are fine: they never reach the model.
		if imposesOutputContract(atom) {
			continue
		}
		for _, shardType := range atom.ShardTypes {
			if !IsStructuredOutputOnly(shardType) {
				continue
			}
			checked++
			for _, marker := range envelopeMarkers {
				if strings.Contains(atom.Content, marker) {
					t.Errorf("atom %q (category %q) is selectable by structured-output-only shard %q "+
						"and survives filterAtomsForStructuredOutput, but its content contains %q. "+
						"That shard's reply is parsed directly by Go; an envelope instruction makes it "+
						"emit the wrong shape and the parse silently yields nothing. "+
						"Remove the envelope from the atom, or give the atom CategoryProtocol so the "+
						"filter drops it.",
						atom.ID, atom.Category, shardType, marker)
					break
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("no atom was gated to a structured-output-only shard type — either the corpus " +
			"stopped shipping campaign role atoms or IsStructuredOutputOnly stopped matching them; " +
			"either way this test is no longer guarding anything")
	}
	t.Logf("checked %d atom/shard-type pairs against %d envelope markers", checked, len(envelopeMarkers))
}

// The planner is the role that actually broke, so assert its contract directly:
// whatever it is told, it must be told to produce phases and never an envelope.
func TestEmbeddedCorpus_PlannerAtomsDemandPopulatedPhases(t *testing.T) {
	corpus, err := getEmbeddedCorpusCached()
	if err != nil {
		t.Fatalf("load embedded corpus: %v", err)
	}

	var plannerContent strings.Builder
	for _, atom := range corpus.All() {
		if atom == nil || imposesOutputContract(atom) {
			continue
		}
		for _, shardType := range atom.ShardTypes {
			if strings.EqualFold(strings.TrimPrefix(shardType, "/"), "planner") {
				plannerContent.WriteString(atom.Content)
				plannerContent.WriteString("\n")
				break
			}
		}
	}

	content := plannerContent.String()
	if content == "" {
		t.Fatal("no planner atoms survived the filter — the planner would compile with no plan schema")
	}
	if !strings.Contains(content, `"phases"`) {
		t.Error("planner atoms never mention a \"phases\" key; the decomposer parses that field and " +
			"an absent one falls through to the degraded three-task scaffold")
	}
	// The exact string that taught the model to elide the array.
	if strings.Contains(content, `"phases": [...]`) {
		t.Error(`planner atoms contain "phases": [...] — a literal ellipsis. Models copy it verbatim ` +
			`and emit no phases. Show a populated example instead.`)
	}
}
