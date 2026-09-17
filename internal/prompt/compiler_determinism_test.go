package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// The property a prefix cache actually rests on: the same compilation context
// over the same corpus must produce the same BYTES.
//
// A provider's prefix cache matches a prefix of the token stream. Anything that
// moves inside the assembled prompt -- a section order decided by map
// iteration, a set rendered in whatever order it came out of a hash table --
// changes the prefix, so the cache-write premium is paid on every request and
// no read ever earns it back. The break-even arithmetic in internal/broker
// assumes bytes that hold still; without that, the whole calculation is about a
// cache that can never hit.
//
// This is deliberately a behavioural test rather than a lint on map iteration.
// A lint cannot tell a map from a slice without type information -- the repo's
// own audit_tiebreak documents that limit -- and it would miss every other
// source of drift: a timestamp, a random id, a pointer address formatted into
// text. Compiling twice and comparing catches all of them, and cannot produce a
// false positive.
//
// Fifty iterations, not two. Go reseeds map iteration per range, so a single
// comparison of two runs passes by luck often enough to be worthless: with two
// sections there is an even chance of missing the defect entirely.
func TestCompiledPromptIsByteStable(t *testing.T) {
	atoms := []*PromptAtom{
		{
			ID:          "identity/coder",
			Category:    CategoryIdentity,
			Content:     "You are the Coder Shard.",
			Priority:    100,
			IsMandatory: true,
		},
		{
			ID:          "protocol/piggyback",
			Category:    CategoryProtocol,
			Content:     "Output your response in the Piggyback format.",
			Priority:    90,
			IsMandatory: true,
		},
		{
			ID:          "methodology/tdd",
			Category:    CategoryMethodology,
			Content:     "Write the failing test first.",
			Priority:    80,
			IsMandatory: true,
		},
		{
			ID:          "domain/go",
			Category:    CategoryDomain,
			Content:     "Prefer table-driven tests.",
			Priority:    70,
			IsMandatory: true,
		},
		{
			ID:          "safety/scope",
			Category:    CategorySafety,
			Content:     "Do not widen the change beyond what was asked.",
			Priority:    60,
			IsMandatory: true,
		},
	}
	for _, atom := range atoms {
		atom.TokenCount = EstimateTokens(atom.Content)
		atom.ContentHash = HashContent(atom.Content)
	}

	corpus := NewEmbeddedCorpus(atoms)

	// A FRESH compiler per compile, which is the whole trick to testing this.
	//
	// The compiler keeps an LRU cache keyed on the context hash, so compiling
	// the same context twice against one compiler returns the first result
	// verbatim -- and a determinism test written the obvious way passes without
	// ever assembling a prompt a second time. It measures the cache and reports
	// on the assembler. Verified by injecting a per-call counter into
	// FinalAssembler.Assemble: with a shared compiler the test stayed green.
	compileOnce := func() string {
		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(&mockKernel{facts: atomsToFacts(atoms)}),
		)
		require.NoError(t, err)

		cc := NewCompilationContext().WithTokenBudget(10000, 1000)
		result, err := compiler.Compile(context.Background(), cc)
		require.NoError(t, err)
		return result.Prompt
	}

	first := compileOnce()
	if first == "" {
		t.Fatal("compiled prompt is empty, so this test proves nothing about its stability")
	}

	for i := 1; i < 50; i++ {
		got := compileOnce()
		if got == first {
			continue
		}
		// Report the first divergence rather than two whole prompts: the
		// difference is usually one reordered block and the prompts are long.
		line := firstDifferingLine(first, got)
		t.Fatalf("the compiled prompt changed between identical compiles, at run %d.\n"+
			"A provider's prefix cache matches a prefix of the token stream, so bytes "+
			"that move are bytes that can never be cached.\n%s", i, line)
	}
}

// firstDifferingLine names where two prompts diverge, for a failure message
// that points at the block responsible instead of printing both prompts.
func firstDifferingLine(a, b string) string {
	al, bl := splitLines(a), splitLines(b)
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return "line " + itoa(i+1) + ":\n  first: " + al[i] + "\n  then:  " + bl[i]
		}
	}
	if len(al) != len(bl) {
		return "the two prompts have different line counts: " + itoa(len(al)) + " then " + itoa(len(bl))
	}
	return "(no differing line found, so the difference is in trailing content)"
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
