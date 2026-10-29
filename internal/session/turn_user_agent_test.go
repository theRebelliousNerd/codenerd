package session

import (
	"testing"

	"codenerd/internal/perception"
)

// A taxonomy verb whose shard type is /none (/explain, /help, /configure ...)
// runs as no agent: before, the user-agent fallback read "/explain" as an
// agent named "explain", so the turn asserted compile_context(/shard,
// /explain), looked up a knowledge DB and a shard profile named "explain",
// scoped its working set under that phantom agent, and was a "speaker" to the
// expert selection. A real user agent's verb still selects the agent.
func TestTaxonomyVerbsRunAsNoUserAgent(t *testing.T) {
	t.Parallel()
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	for _, verb := range []string{"/explain", "/help", "/configure", "/greet"} {
		cc := e.buildCompilationContext(t.Context(), perception.Intent{Verb: verb})
		if cc.ShardID != "" || cc.ShardType != "" {
			t.Errorf("%s compiled as agent %q (%q); a taxonomy verb names no agent", verb, cc.ShardID, cc.ShardType)
		}
	}
	for verb, want := range map[string]string{"/consult/goexpert": "goexpert", "/goexpert": "goexpert"} {
		cc := e.buildCompilationContext(t.Context(), perception.Intent{Verb: verb})
		if cc.ShardID != want || cc.ShardType != "/"+want {
			t.Errorf("%s compiled as %q (%q), want the user agent %q", verb, cc.ShardID, cc.ShardType, want)
		}
	}
}
