package prompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildContextFacts_CompileShard(t *testing.T) {
	s := NewAtomSelector()

	findCompileShard := func(facts []any) (string, bool) {
		for _, f := range facts {
			str, ok := f.(string)
			if !ok {
				continue
			}
			if strings.Contains(str, "compile_shard") {
				return str, true
			}
		}
		return "", false
	}

	// Case: ShardType "/coder" with atom ShardTypes ["/coder"] should emit atom forms
	cc := NewCompilationContext()
	cc.ShardID = "test-shard-123"
	cc.ShardType = "/coder"
	atom := &PromptAtom{
		ID:         "atom-1",
		ShardTypes: []string{"/coder"},
	}
	facts, err := s.buildContextFacts(cc, []*PromptAtom{atom}, nil)
	require.NoError(t, err)

	foundFact, found := findCompileShard(facts)
	require.True(t, found, "compile_shard fact should be present when shard type is set, got facts: %v", facts)
	assert.Contains(t, foundFact, `"test-shard-123"`, "ShardID should be emitted as quoted string, got %q", foundFact)
	assert.Contains(t, foundFact, `, /coder)`, "ShardType should be emitted as atom /coder, got %q", foundFact)
	assert.Contains(t, foundFact, `compile_shard("test-shard-123", /coder)`, "compile_shard fact should have atom form, got %q", foundFact)
	assert.NotContains(t, foundFact, `"/coder"`, "ShardType should not be quoted string")
	assert.NotContains(t, foundFact, `"coder"`, "ShardType should not be old quoted form without slash")

	// atom_tag for both /shard and /shard_type with unquoted values
	hasShard := false
	hasShardType := false
	for _, f := range facts {
		str, ok := f.(string)
		if !ok {
			continue
		}
		if str == `atom_tag("atom-1", /shard, /coder)` {
			hasShard = true
		}
		if str == `atom_tag("atom-1", /shard_type, /coder)` {
			hasShardType = true
		}
	}
	assert.True(t, hasShard, "should emit atom_tag with /shard, /coder, got facts: %v", facts)
	assert.True(t, hasShardType, "should emit atom_tag with /shard_type, /coder, got facts: %v", facts)

	// Ensure atom_tag values are atoms not strings
	for _, f := range facts {
		str, ok := f.(string)
		if !ok {
			continue
		}
		if strings.Contains(str, `atom_tag("atom-1"`) && strings.Contains(str, "shard") {
			assert.NotContains(t, str, `"/coder"`, "atom_tag value should be atom not string, got %q", str)
			assert.NotContains(t, str, `"/shard"`, "dimension should be atom, got %q", str)
			assert.NotContains(t, str, `"/shard_type"`, "dimension should be atom, got %q", str)
		}
	}

	// Verify current_context fact still present
	hasCurrentContext := false
	for _, f := range facts {
		if str, ok := f.(string); ok && strings.Contains(str, "current_context") {
			hasCurrentContext = true
			break
		}
	}
	assert.True(t, hasCurrentContext, "current_context fact must still be present")

	// Case: atom with empty ShardTypes emits no shard atom_tag
	atomEmpty := &PromptAtom{
		ID:         "atom-empty",
		ShardTypes: []string{},
	}
	factsEmptyTags, err := s.buildContextFacts(cc, []*PromptAtom{atomEmpty}, nil)
	require.NoError(t, err)
	for _, f := range factsEmptyTags {
		str, ok := f.(string)
		if !ok {
			continue
		}
		if strings.Contains(str, "atom_tag") && strings.Contains(str, "atom-empty") && (strings.Contains(str, "/shard,") || strings.Contains(str, "/shard_type,")) {
			t.Fatalf("should not emit shard atom_tag for empty ShardTypes, got %q", str)
		}
	}

	// Case: Empty ShardType emits no compile_shard
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

	// Case: Whitespace-only shard type should emit no fact
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

	// Case: Slash-prefixed "/coder" should emit /coder atom
	ccSlash := NewCompilationContext()
	ccSlash.ShardID = "test-shard-123"
	ccSlash.ShardType = "/coder"
	factsSlash, err := s.buildContextFacts(ccSlash, nil, nil)
	require.NoError(t, err)
	foundFact, found = findCompileShard(factsSlash)
	require.True(t, found, "compile_shard fact should be present for slash-prefixed shard type, got facts: %v", factsSlash)
	assert.Contains(t, foundFact, `compile_shard("test-shard-123", /coder)`, "slash-prefixed ShardType should emit atom /coder, got %q", foundFact)
	assert.NotContains(t, foundFact, `"/coder"`, "ShardType should not be quoted")
	assert.NotContains(t, foundFact, `"coder"`, "should not be old quoted form")

	// Case: Plain "coder" without slash should still emit /coder atom
	ccPlain := NewCompilationContext()
	ccPlain.ShardID = "test-shard-123"
	ccPlain.ShardType = "coder"
	factsPlain, err := s.buildContextFacts(ccPlain, nil, nil)
	require.NoError(t, err)
	foundFact, found = findCompileShard(factsPlain)
	require.True(t, found, "compile_shard fact should be present for plain shard type, got facts: %v", factsPlain)
	assert.Contains(t, foundFact, `compile_shard("test-shard-123", /coder)`, "plain ShardType should emit atom /coder, got %q", foundFact)

	// Case: Single slash "/" should emit no fact (writeAtom returns false for invalid atom)
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

	// Case: atom with ShardTypes ["coder"] without slash should also emit /coder atom
	atomPlain := &PromptAtom{
		ID:         "atom-plain",
		ShardTypes: []string{"coder"},
	}
	factsPlainAtom, err := s.buildContextFacts(cc, []*PromptAtom{atomPlain}, nil)
	require.NoError(t, err)
	hasPlainShard := false
	hasPlainShardType := false
	for _, f := range factsPlainAtom {
		str, ok := f.(string)
		if !ok {
			continue
		}
		if str == `atom_tag("atom-plain", /shard, /coder)` {
			hasPlainShard = true
		}
		if str == `atom_tag("atom-plain", /shard_type, /coder)` {
			hasPlainShardType = true
		}
	}
	assert.True(t, hasPlainShard, "plain shard type should emit /coder atom for /shard, got %v", factsPlainAtom)
	assert.True(t, hasPlainShardType, "plain shard type should emit /coder atom for /shard_type, got %v", factsPlainAtom)
}
