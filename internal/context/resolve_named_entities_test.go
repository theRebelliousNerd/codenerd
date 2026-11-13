package context

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"codenerd/internal/core"
)

// referenceLookups is the implementation resolveNamedEntities replaced: both
// lookups built for every fact, then read for the names.
func referenceLookups(allFacts []core.Fact, names []string) (map[string]core.Fact, map[string][]core.Fact) {
	factLookup := make(map[string]core.Fact, len(allFacts))
	byEntity := make(map[string][]core.Fact, len(allFacts))
	for _, f := range allFacts {
		factLookup[f.String()] = f
		for _, arg := range f.Args {
			s, ok := arg.(string)
			if !ok || s == "" {
				continue
			}
			byEntity[s] = append(byEntity[s], f)
		}
	}
	exact := make(map[string]core.Fact)
	entity := make(map[string][]core.Fact)
	for _, n := range names {
		if f, ok := factLookup[n]; ok {
			exact[n] = f
		}
		if fs, ok := byEntity[n]; ok && n != "" {
			entity[n] = fs
		}
	}
	return exact, entity
}

func namedEntityFacts(n int) []core.Fact {
	facts := make([]core.Fact, 0, n+4)
	for i := range n {
		file := fmt.Sprintf("internal/pkg%03d/file%05d.go", i%300, i%7000)
		switch i % 4 {
		case 0:
			facts = append(facts, core.Fact{Predicate: "dependency_link", Args: []any{file, fmt.Sprintf("pkg:mod/p%04d", i%900), "mod"}})
		case 1:
			facts = append(facts, core.Fact{Predicate: "code_defines", Args: []any{file, fmt.Sprintf("pkg.Sym%d", i), "/function", int64(i), int64(i + 3)}})
		case 2:
			facts = append(facts, core.Fact{Predicate: "modified", Args: []any{file}})
		default:
			facts = append(facts, core.Fact{Predicate: "cyclomatic_complexity", Args: []any{file, fmt.Sprintf("pkg.Sym%d", i-2), int64(i % 40)}})
		}
	}
	return append(facts,
		core.Fact{Predicate: "session_active"},
		core.Fact{Predicate: "modified", Args: []any{`odd "quoted" name (with parens).go`}},
		core.Fact{Predicate: "note", Args: []any{"", int64(3)}},
		core.Fact{Predicate: "modified", Args: []any{"internal/pkg001/file00001.go"}},
	)
}

// The two lookups, built for the named strings only, hold exactly what the
// lookups built for every fact held for those strings: the same exact fact,
// the same facts per entity, in the same order.
func TestResolveNamedEntities_MatchesTheLookupsBuiltForEveryFact(t *testing.T) {
	facts := namedEntityFacts(20000)
	names := []string{
		"internal/pkg001/file00001.go", // an entity many facts name
		"pkg.Sym5",                     // a symbol
		"pkg:mod/p0008",                // an import target
		facts[1].String(),              // a fact's own spelling
		facts[len(facts)-4].String(),   // a zero-arity fact's spelling
		facts[len(facts)-3].String(),   // quotes and parentheses in an argument
		`odd "quoted" name (with parens).go`,
		"/function",        // a name constant carried as a string argument
		"no/such/thing.go", // named by the kernel, held by no fact
		"modified(",        // a predicate prefix that is no fact's spelling
		"",
	}
	wantExact, wantEntity := referenceLookups(facts, names)
	gotExact, gotEntity := resolveNamedEntities(facts, slices.Values(names))
	if !reflect.DeepEqual(gotExact, wantExact) {
		t.Fatalf("exact lookups differ:\n got %v\nwant %v", gotExact, wantExact)
	}
	if len(gotEntity) != len(wantEntity) {
		t.Fatalf("entity lookups name %d strings, want %d", len(gotEntity), len(wantEntity))
	}
	for name, want := range wantEntity {
		if !reflect.DeepEqual(gotEntity[name], want) {
			t.Fatalf("facts naming %q differ: got %d, want %d (or their order)", name, len(gotEntity[name]), len(want))
		}
	}
	if len(wantExact) < 3 || len(wantEntity) < 5 {
		t.Fatalf("the fixture resolved too little to prove anything: %d exact, %d entities", len(wantExact), len(wantEntity))
	}
}

func BenchmarkResolveNamedEntities(b *testing.B) {
	facts := namedEntityFacts(300000)
	names := []string{"internal/pkg001/file00001.go", "pkg.Sym5", "pkg:mod/p0008", facts[1].String(), "no/such/thing.go"}
	b.Run("every-fact", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			referenceLookups(facts, names)
		}
	})
	b.Run("named-only", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			resolveNamedEntities(facts, slices.Values(names))
		}
	})
}
