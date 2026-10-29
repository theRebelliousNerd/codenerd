package prompt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// mangleUpdatesExample is a "mangle_updates": [...] array in an atom's content:
// the writes an atom shows the model how to make.
var mangleUpdatesExample = regexp.MustCompile(`(?s)"mangle_updates"\s*:\s*\[(.*?)\]`)

// jsonStringLiteral is one JSON string inside such an array.
var jsonStringLiteral = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// Every mangle_updates example the corpus teaches survives the gate the runtime
// applies to a model's writes (core.FilterMangleUpdates with the model
// observation policy, on a kernel loaded with the policy corpus: allowlist,
// declaration, arity, prose_only). campaign/taxonomist/output_protocol taught
// task_phase, phase_order and dependency_edge -- no Decl, not on the
// allowlist -- so every write it asked for was refused, and a model that
// followed the atom was told nothing (2026-09-23). The gate is probed, not
// restated, so the two cannot drift.
func TestEmbeddedCorpus_MangleUpdateExamplesPassTheModelGate(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("load the embedded corpus: %v", err)
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	policy := core.ModelObservationPolicy()

	var offences []string
	examples := 0
	for _, a := range corpus.All() {
		if a == nil {
			continue
		}
		for _, block := range mangleUpdatesExample.FindAllStringSubmatch(a.Content, -1) {
			for _, literal := range jsonStringLiteral.FindAllString(block[1], -1) {
				var update string
				if err := json.Unmarshal([]byte(literal), &update); err != nil {
					offences = append(offences, fmt.Sprintf("atom %q: example %s is not a JSON string: %v", a.ID, literal, err))
					continue
				}
				update = strings.TrimSpace(update)
				if !strings.HasSuffix(update, ".") {
					update += "."
				}
				examples++
				if _, blocked := core.FilterMangleUpdates(kernel, []string{update}, policy); len(blocked) > 0 {
					offences = append(offences, fmt.Sprintf("atom %q teaches %s -- refused: %s", a.ID, update, blocked[0].Reason))
				}
			}
		}
	}
	if examples == 0 {
		t.Fatal("no mangle_updates example was found in the embedded corpus; the probe measures nothing")
	}
	sort.Strings(offences)
	if len(offences) > 0 {
		t.Errorf("%d mangle_updates example(s) in the embedded corpus teach a write the gate refuses:\n  %s",
			len(offences), strings.Join(offences, "\n  "))
	}
}

// A structured-output shard's compile drops every output-contract atom before
// dependencies resolve (selector.go, filterAtomsForStructuredOutput), and the
// resolver then prunes, transitively, every atom whose dependency is missing.
// So an atom that serves such a shard and depends on an output-contract atom
// never reaches it, mandatory or not: campaign/taxonomist/execution_strategy
// depended on a protocol atom and was pruned from every planner compile
// (2026-09-23).
func TestEmbeddedCorpus_StructuredOutputAtomsDoNotDependOnOutputContracts(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("load the embedded corpus: %v", err)
	}
	byID := make(map[string]*PromptAtom)
	for _, a := range corpus.All() {
		if a != nil {
			byID[a.ID] = a
		}
	}

	var offences []string
	checked := 0
	for _, a := range corpus.All() {
		if a == nil || imposesOutputContract(a) {
			continue
		}
		for _, shard := range a.ShardTypes {
			if !IsStructuredOutputOnly(shard) {
				continue
			}
			checked++
			for _, dep := range a.DependsOn {
				if d := byID[dep]; d != nil && imposesOutputContract(d) {
					offences = append(offences, fmt.Sprintf("atom %q serves %s and depends on output contract %q", a.ID, shard, dep))
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no atom serving a structured-output shard was found; the check measures nothing")
	}
	sort.Strings(offences)
	if len(offences) > 0 {
		t.Errorf("%d atom(s) can never reach a structured-output shard: their dependency is dropped first:\n  %s",
			len(offences), strings.Join(offences, "\n  "))
	}
}
