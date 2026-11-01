//go:build integration

package e2e_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

func contextFactKeys(t *testing.T, kernel *core.RealKernel) []string {
	t.Helper()
	facts, err := kernel.Query("compile_context")
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(facts))
	for _, fact := range facts {
		key := fact.Predicate
		for _, arg := range fact.Args {
			key += fmt.Sprintf("|%T:%v", arg, arg)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestE2E_PromptKernelAdapter_ContextFactParity(t *testing.T) {
	cc := prompt.NewCompilationContext()
	cc.IntentVerb, cc.Language = "/test", "go"
	for _, tc := range []struct {
		name            string
		facts           []any
		productionFacts []any
	}{
		{"generated context", cc.ToContextFacts(), nil},
		{"dotted", []any{`compile_context(/language, "go").`}, nil},
		{"bare", []any{`compile_context(/language, "go")`}, nil},
		{"whitespace", []any{" \n compile_context(/language, \"go\"). \t\n"}, []any{`compile_context(/language, "go").`}},
		{"quoted period", []any{`compile_context(/model, "literal.").`}, nil},
		{"name constant", []any{`compile_context(/language, /go).`}, nil},
		{"typed core", []any{core.Fact{Predicate: "compile_context", Args: []any{types.MangleAtom("/language"), "go"}}}, nil},
		{"typed prompt", []any{prompt.Fact{Predicate: "compile_context", Args: []any{types.MangleAtom("/language"), types.MangleAtom("/go")}}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixtureKernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			productionKernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			productionFacts := tc.productionFacts
			if productionFacts == nil {
				productionFacts = tc.facts
			}
			// The fixture also normalizes surrounding whitespace; the production
			// control here receives its canonical producer clause.
			if err := system.NewKernelAdapter(productionKernel).AssertBatch(productionFacts); err != nil {
				t.Fatalf("production control rejected context: %v", err)
			}
			if err := (&promptKernelAdapter{kernel: fixtureKernel}).AssertBatch(tc.facts); err != nil {
				t.Fatalf("fixture rejected producer/production-compatible context: %v", err)
			}
			got, want := contextFactKeys(t, fixtureKernel), contextFactKeys(t, productionKernel)
			if len(got) == 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("context fact values/types changed: got=%v want=%v", got, want)
			}
		})
	}
}

func TestE2E_PromptKernelAdapter_InvalidBatchRefusedBeforeMutation(t *testing.T) {
	for _, invalid := range []any{`compile_context(/language,`, struct{}{}, " . \n"} {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatal(err)
		}
		adapter := &promptKernelAdapter{kernel: kernel}
		prefix := prompt.Fact{Predicate: "compile_context", Args: []any{types.MangleAtom("/language"), "retained only on success"}}
		if err := adapter.AssertBatch([]any{prefix, invalid}); err == nil {
			t.Fatalf("accepted invalid fact type %T", invalid)
		}
		if got := contextFactKeys(t, kernel); len(got) != 0 {
			t.Fatalf("parse/type refusal mutated kernel: %v", got)
		}
	}
}
