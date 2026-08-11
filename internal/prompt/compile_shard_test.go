package prompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildContextFacts_CompileShard(t *testing.T) {
	s := NewAtomSelector()

	cc := NewCompilationContext()
	cc.ShardID = "test-shard-123"
	cc.ShardType = "coder"
	facts, err := s.buildContextFacts(cc, nil, nil)
	require.NoError(t, err)

	found := false
	var foundFact string
	for _, f := range facts {
		str, ok := f.(string)
		if !ok {
			continue
		}
		if strings.Contains(str, "compile_shard") {
			found = true
			foundFact = str
			break
		}
	}
	require.True(t, found, "compile_shard fact should be present when shard type is set, got facts: %v", facts)
	// Both args must be plain Go strings (quoted), not atoms like /coder
	assert.Contains(t, foundFact, `"test-shard-123"`, "ShardID should be emitted as quoted string, got %q", foundFact)
	assert.Contains(t, foundFact, `"coder"`, "ShardType should be emitted as quoted string, got %q", foundFact)
	assert.Contains(t, foundFact, `compile_shard("test-shard-123", "coder")`, "compile_shard fact should have both args as quoted strings, got %q", foundFact)

	// Verify current_context fact is still present (must not have been removed)
	hasCurrentContext := false
	for _, f := range facts {
		if str, ok := f.(string); ok && strings.Contains(str, "current_context") {
			hasCurrentContext = true
			break
		}
	}
	assert.True(t, hasCurrentContext, "current_context fact must still be present")

	// Empty shard type should emit no compile_shard fact
	ccEmpty := NewCompilationContext()
	ccEmpty.ShardID = "test-shard-123"
	ccEmpty.ShardType = ""
	factsEmpty, err := s.buildContextFacts(ccEmpty, nil, nil)
	require.NoError(t, err)
	for _, f := range factsEmpty {
		if str, ok := f.(string); ok {
			assert.NotContains(t, str, "compile_shard", "should not emit compile_shard when ShardType is empty")
		}
	}

	// Whitespace-only shard type should also emit no fact
	ccWS := NewCompilationContext()
	ccWS.ShardID = "test-shard-123"
	ccWS.ShardType = "   "
	factsWS, err := s.buildContextFacts(ccWS, nil, nil)
	require.NoError(t, err)
	for _, f := range factsWS {
		if str, ok := f.(string); ok {
			assert.NotContains(t, str, "compile_shard", "should not emit compile_shard when ShardType is whitespace")
		}
	}

	// Slash-prefixed ShardType "/coder" should be normalized to "coder" (no slash)
	ccSlash := NewCompilationContext()
	ccSlash.ShardID = "test-shard-123"
	ccSlash.ShardType = "/coder"
	factsSlash, err := s.buildContextFacts(ccSlash, nil, nil)
	require.NoError(t, err)
	found = false
	foundFact = ""
	for _, f := range factsSlash {
		str, ok := f.(string)
		if !ok {
			continue
		}
		if strings.Contains(str, "compile_shard") {
			found = true
			foundFact = str
			break
		}
	}
	require.True(t, found, "compile_shard fact should be present for slash-prefixed shard type, got facts: %v", factsSlash)
	assert.Contains(t, foundFact, `compile_shard("test-shard-123", "coder")`, "slash-prefixed ShardType should be normalized to \"coder\", got %q", foundFact)
	assert.NotContains(t, foundFact, `"/coder"`, "ShardType should not contain slash, got %q", foundFact)

	// Idempotent: plain "coder" should still emit "coder"
	ccPlain := NewCompilationContext()
	ccPlain.ShardID = "test-shard-123"
	ccPlain.ShardType = "coder"
	factsPlain, err := s.buildContextFacts(ccPlain, nil, nil)
	require.NoError(t, err)
	found = false
	foundFact = ""
	for _, f := range factsPlain {
		str, ok := f.(string)
		if !ok {
			continue
		}
		if strings.Contains(str, "compile_shard") {
			found = true
			foundFact = str
			break
		}
	}
	require.True(t, found, "compile_shard fact should be present for plain shard type, got facts: %v", factsPlain)
	assert.Contains(t, foundFact, `compile_shard("test-shard-123", "coder")`, "plain ShardType should emit \"coder\", got %q", foundFact)

	// Single slash "/" should emit no fact (after trimming slash becomes empty)
	ccSingleSlash := NewCompilationContext()
	ccSingleSlash.ShardID = "test-shard-123"
	ccSingleSlash.ShardType = "/"
	factsSingleSlash, err := s.buildContextFacts(ccSingleSlash, nil, nil)
	require.NoError(t, err)
	for _, f := range factsSingleSlash {
		if str, ok := f.(string); ok {
			assert.NotContains(t, str, "compile_shard", "should not emit compile_shard when ShardType is \"/\"")
		}
	}
}
