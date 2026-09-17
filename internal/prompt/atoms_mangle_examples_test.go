package prompt

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// mangleExampleExemptFiles lists atom files whose mangle_updates examples are
// NOT filtered by core.ModelObservationPolicy, with the consumer that filters
// them instead. An entry here needs a real alternative consumer, not a wish to
// keep an example the kernel refuses.
var mangleExampleExemptFiles = map[string]string{
	"campaign/taxonomist/rules.yaml": "planner/taxonomist envelopes are filtered by the planner shard's own policy (internal/shards/system/planner.go)",
}

var mangleUpdatesArrayRe = regexp.MustCompile(`(?s)"mangle_updates"\s*:\s*(\[.*?\])`)

// TestAtomCorpus_MangleUpdateExamplesAreAccepted runs every mangle_updates
// example the built-in atoms show a model through the same filter, policy and
// kernel the chat and session surfaces apply to the model's reply.
//
// A worked example is the most imitated context in a prompt. The envelope atom
// used to show "predicate(arg1, arg2)." and the exemplars showed file_modified,
// test_generated, coverage_target(..., 0.90) and a four-argument
// review_finding — none of which the kernel accepts — so a model that copied
// its instructions faithfully had its facts dropped and the user was shown
// "[Kernel] Mangle update dropped" for it. The examples and the allowlist are
// separate files with no compiler between them; this test is that compiler.
func TestAtomCorpus_MangleUpdateExamplesAreAccepted(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	policy := core.ModelObservationPolicy()

	checked := 0
	filesWithExamples := 0
	walkErr := filepath.WalkDir("atoms", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(path), "atoms/"))
		if _, exempt := mangleExampleExemptFiles[rel]; exempt {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		matches := mangleUpdatesArrayRe.FindAllStringSubmatch(string(raw), -1)
		sawExample := false
		for _, m := range matches {
			var updates []string
			if err := json.Unmarshal([]byte(m[1]), &updates); err != nil {
				t.Errorf("%s: mangle_updates example is not a JSON string array: %v\n%s", rel, err, m[1])
				continue
			}
			if len(updates) == 0 {
				continue
			}
			sawExample = true
			facts, blocked := core.FilterMangleUpdates(kernel, updates, policy)
			for _, b := range blocked {
				t.Errorf("%s: example the kernel would drop: %q: %s", rel, b.Update, b.Reason)
			}
			for _, f := range facts {
				if err := kernel.Assert(f); err != nil {
					t.Errorf("%s: example the kernel would reject on assert: %s: %v", rel, f.Predicate, err)
				}
			}
			checked += len(updates)
		}
		if sawExample {
			filesWithExamples++
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk atoms: %v", walkErr)
	}

	// Guard against the extraction silently matching nothing: a regexp that
	// stops finding examples would otherwise turn this into a test that passes
	// by checking zero facts.
	if checked < 10 || filesWithExamples < 5 {
		t.Fatalf("checked %d example facts in %d files; expected at least 10 in 5 — the extraction no longer finds the corpus examples", checked, filesWithExamples)
	}
	for rel := range mangleExampleExemptFiles {
		if _, err := os.Stat(filepath.Join("atoms", filepath.FromSlash(rel))); err != nil {
			t.Errorf("exempt file %s no longer exists; remove the exemption: %v", rel, err)
		}
	}
}

// TestAtomCorpus_MangleUpdateRulesTravelWithTheEnvelope pins the delivery
// contract: the atom that teaches what mangle_updates accepts must be selected
// whenever the envelope that introduces the field is. While it was optional it
// reached 0 of 150 recorded compiles.
func TestAtomCorpus_MangleUpdateRulesTravelWithTheEnvelope(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("atoms", "protocol", "piggyback.yaml"))
	if err != nil {
		t.Fatalf("read piggyback.yaml: %v", err)
	}
	parsed, _, err := ParsePromptAtomYAML(raw, "protocol/piggyback.yaml", nil)
	if err != nil {
		t.Fatalf("parse piggyback.yaml: %v", err)
	}
	byID := make(map[string]*PromptAtom, len(parsed))
	for i := range parsed {
		byID[parsed[i].Atom.ID] = parsed[i].Atom
	}
	envelope, rules := byID["protocol/piggyback/envelope"], byID["protocol/piggyback/mangle_updates"]
	if envelope == nil || rules == nil {
		t.Fatalf("envelope=%v rules=%v: both atoms must exist", envelope != nil, rules != nil)
	}
	if envelope.IsMandatory && !rules.IsMandatory {
		t.Errorf("envelope is mandatory but the mangle_updates rules atom is not: the field is delivered without its rules")
	}
	if len(rules.ShardTypes)+len(rules.IntentVerbs)+len(rules.OperationalModes) > 0 && len(envelope.ShardTypes)+len(envelope.IntentVerbs)+len(envelope.OperationalModes) == 0 {
		t.Errorf("rules atom is gated by selectors the envelope is not; it would be blocked in contexts where the envelope is still sent")
	}
	if strings.Contains(envelope.Content, "predicate(arg1") {
		t.Errorf("envelope shows a bare-word fact example again; bare words do not parse")
	}
	for pred := range core.ModelObservationPolicy().AllowedPredicates {
		if pred == "checkpoint_verdict" {
			continue // taught by the campaign checkpoint atoms, to the reviewer that emits it
		}
		if !strings.Contains(rules.Content, pred+"(") {
			t.Errorf("rules atom does not teach accepted predicate %q", pred)
		}
	}
}
