package core

import "testing"

// simulated_effect carried its argument list rendered with %v, which turns
// ["a b", "c"] into "[a b c]" -- two arguments or three, the fact could not
// say. It carries JSON now, the encoding ToAtom uses for the same container.
func TestEffectArgsJSON_ShouldKeepArgumentBoundaries(t *testing.T) {
	t.Parallel()
	if got := effectArgsJSON([]any{"a b", "c", int64(3)}); got != `["a b","c",3]` {
		t.Fatalf("effectArgsJSON = %s", got)
	}
	if got := effectArgsJSON(nil); got != "null" {
		t.Fatalf("effectArgsJSON(nil) = %s", got)
	}
}
