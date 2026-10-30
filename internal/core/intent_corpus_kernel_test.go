package core

import (
	"sort"
	"strings"
	"testing"
)

// The intent corpus (defaults/schema/intent_*.mg) reaches the sharded kernel
// one way only: loadEmbeddedIntentFacts reads its FACT clauses of the predicates
// in defaultIntentFactPredicates, and every shard loads them at init. Its rules
// and its other fact tables (multistep_*, pattern_*) are not in the kernel: they
// run in the perception taxonomy engine, one unsharded store. So a kernel rule
// reading one of those would never fire, in any shard, and nothing would say so.

// intentCorpusClauses returns the corpus's fact predicates and rule heads.
func intentCorpusClauses(t *testing.T) (facts, ruleHeads map[string]struct{}) {
	t.Helper()
	facts, ruleHeads = map[string]struct{}{}, map[string]struct{}{}
	for _, f := range DefaultIntentSchemaFiles() {
		data, err := coreLogic.ReadFile("defaults/" + f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		unit, err := parseUnit(strings.NewReader(string(data)))
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, c := range unit.Clauses {
			if len(c.Premises) == 0 {
				facts[c.Head.Predicate.Symbol] = struct{}{}
			} else {
				ruleHeads[c.Head.Predicate.Symbol] = struct{}{}
			}
		}
	}
	return facts, ruleHeads
}

// kernelReadersOf returns, for each predicate in names, the kernel rules (file
// and head) that read it in a body.
func kernelReadersOf(t *testing.T, corpus string, names map[string]struct{}) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, chunk := range splitPolicyByFile(corpus) {
		unit, err := parseUnit(strings.NewReader(chunk.text))
		if err != nil {
			t.Fatalf("parse %s: %v", chunk.file, err)
		}
		for _, c := range unit.Clauses {
			for _, p := range c.Premises {
				if sym, _, ok := bodyTermPred(p); ok {
					if _, wanted := names[sym]; wanted {
						out[sym] = append(out[sym], chunk.file+":"+c.Head.Predicate.Symbol)
					}
				}
			}
		}
	}
	return out
}

func notLoaded(facts, ruleHeads, loaded map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for p := range facts {
		if _, ok := loaded[p]; !ok {
			out[p] = struct{}{}
		}
	}
	for p := range ruleHeads {
		out[p] = struct{}{}
	}
	return out
}

func TestIntentCorpus_NoKernelRuleReadsWhatTheKernelDoesNotLoad(t *testing.T) {
	facts, ruleHeads := intentCorpusClauses(t)
	unloaded := notLoaded(facts, ruleHeads, IntentCorpusPredicates())
	if _, ok := unloaded["multistep_pattern"]; !ok {
		t.Fatalf("control: multistep_pattern is corpus data the kernel does not load; unloaded = %v", unloaded)
	}
	schemas, policy, err := DefaultCorpusText()
	if err != nil {
		t.Fatal(err)
	}
	readers := kernelReadersOf(t, schemas+"\n"+policy, unloaded)
	var names []string
	for p := range readers {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		t.Errorf("%s is intent-corpus data or a rule head the kernel never loads (the perception taxonomy engine does), yet kernel rules read it and can never fire: %v", p, readers[p])
	}
}

// The detector proves itself: a kernel rule reading an unloaded table is found.
func TestIntentCorpus_ReaderCheckFindsARuleOverAnUnloadedTable(t *testing.T) {
	policy := "# Policy Module: p.mg\nDecl multistep_pattern(A, B, C, D).\nDecl h(X).\nh(X) :- multistep_pattern(X, _, _, _).\nh(X) :- other(X), !multistep_pattern(X, _, _, _).\n"
	got := kernelReadersOf(t, policy, map[string]struct{}{"multistep_pattern": {}})
	if len(got["multistep_pattern"]) != 2 {
		t.Fatalf("want the positive and the negated reader, got %v", got)
	}
}

// An allowed predicate with no fact in the corpus is a dead allowance: nothing
// loads, and a kernel rule reading it would be starved.
func TestIntentCorpus_EveryAllowedPredicateHasFacts(t *testing.T) {
	facts, _ := intentCorpusClauses(t)
	for p := range IntentCorpusPredicates() {
		if _, ok := facts[p]; !ok {
			t.Errorf("%s is allowed in defaultIntentFactPredicates but no intent_*.mg file has a fact of it", p)
		}
	}
	// And every fact the corpus defines is either allowed (loaded) or known to
	// be perception-only: the multistep tables.
	allowed := IntentCorpusPredicates()
	for p := range facts {
		if _, ok := allowed[p]; ok {
			continue
		}
		if !strings.HasPrefix(p, "multistep_") {
			t.Errorf("intent corpus facts of %s are neither loaded into the kernel nor a multistep table: they are dropped", p)
		}
	}
}
