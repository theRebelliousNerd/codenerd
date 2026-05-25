package perception

import (
	"context"
	"testing"
)

func BenchmarkMatchVerbFromCorpus(b *testing.B) {
	// Setup a dummy corpus
	SetVerbCorpus([]VerbEntry{
		{Verb: "/explain", Category: "/query", ShardType: "general"},
		{Verb: "/test", Category: "/action", ShardType: "test"},
		{Verb: "/run", Category: "/action", ShardType: "run"},
	})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchVerbFromCorpus(ctx, "test input that might match something")
	}
}

func BenchmarkGetRegexCandidates(b *testing.B) {
	SetVerbCorpus([]VerbEntry{
		{Verb: "/explain", Category: "/query", ShardType: "general"},
		{Verb: "/test", Category: "/action", ShardType: "test"},
		{Verb: "/run", Category: "/action", ShardType: "run"},
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getRegexCandidates("test input", GetVerbCorpus())
	}
}
